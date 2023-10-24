package sigv4

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/aws/aws-sdk-go/aws"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/private/protocol/rest"

	"github.com/grafana/grafana-aws-sdk/pkg/auth"
)

var (
	signerCache sync.Map
)

type SigV4Config struct {
	AuthType string

	Profile string

	AccessKey    string
	SecretKey    string
	SessionToken string

	AssumeRoleARN string
	ExternalID    string
	Region        string

	Service string
}

func (c SigV4Config) asSha256() (string, error) {
	h := sha256.New()
	_, err := h.Write([]byte(fmt.Sprintf("%v", c)))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// The RoundTripperFunc type is an adapter to allow the use of ordinary
// functions as RoundTrippers. If f is a function with the appropriate
// signature, RoundTripperFunc(f) is a RoundTripper that calls f.
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

type Opts struct {
	VerboseMode bool
}

// New instantiates a new signing middleware with an optional succeeding
// middleware. The http.DefaultTransport will be used if nil
func New(cfg *SigV4Config, next http.RoundTripper, opts ...Opts) (http.RoundTripper, error) {
	var sigv4Opts Opts
	switch len(opts) {
	case 0:
		sigv4Opts = Opts{
			VerboseMode: false,
		}
	case 1:
		sigv4Opts = opts[0]
	default:
		return nil, fmt.Errorf("only empty or one Opts is valid as an argument")
	}

	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if next == nil {
			next = http.DefaultTransport
		}

		var signer *v4.Signer
		cached, cacheHit := cachedSigner(cfg)
		if cacheHit {
			signer = cached
		} else {
			var err error
			signer, err = createSigner(cfg, sigv4Opts.VerboseMode)
			if err != nil {
				return nil, err
			}

			sha, err := cfg.asSha256()
			if err != nil {
				return nil, err
			}
			signerCache.Store(sha, signer)
		}

		m := &middleware{
			config:      cfg,
			next:        next,
			signer:      signer,
			verboseMode: sigv4Opts.VerboseMode,
		}

		return m.exec(r)
	}), nil
}

type middleware struct {
	signer *v4.Signer
	config *SigV4Config
	next   http.RoundTripper

	verboseMode bool
}

func (m *middleware) exec(origReq *http.Request) (*http.Response, error) {
	req, err := m.createSignedRequest(origReq)
	if err != nil {
		return nil, err
	}

	return m.next.RoundTrip(req)
}

func (m *middleware) createSignedRequest(origReq *http.Request) (*http.Request, error) {
	m.logRequest(origReq, "stage", "pre-signature")

	req, err := http.NewRequest(origReq.Method, origReq.URL.String(), origReq.Body)
	if err != nil {
		return nil, err
	}

	body := bytes.NewReader([]byte{})
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}

	if strings.Contains(req.URL.RawPath, "%2C") {
		req.URL.RawPath = rest.EscapePath(req.URL.RawPath, false)
	}

	_, err = m.signer.Sign(req, body, m.config.Service, m.config.Region, time.Now().UTC())

	copyHeaderWithoutOverwrite(req.Header, origReq.Header)

	m.logRequest(req, "stage", "post-signature")

	return req, err
}

func cachedSigner(cfg *SigV4Config) (*v4.Signer, bool) {
	sha, err := cfg.asSha256()
	if err != nil {
		return nil, false
	}

	if cached, exists := signerCache.Load(sha); exists {
		return cached.(*v4.Signer), true
	}
	return nil, false
}

func createSigner(cfg *SigV4Config, verboseMode bool) (*v4.Signer, error) {
	c, err := auth.GetCredsFromConfig(auth.CredentialsConfig{
		AuthType:      cfg.AuthType,
		Profile:       cfg.Profile,
		AccessKey:     cfg.AccessKey,
		SecretKey:     cfg.SecretKey,
		SessionToken:  cfg.SessionToken,
		AssumeRoleARN: cfg.AssumeRoleARN,
		ExternalID:    cfg.ExternalID,
		Region:        cfg.Region,
	})
	if err != nil {
		return nil, err
	}

	var signerOpts = func(s *v4.Signer) {
		if verboseMode {
			s.Logger = awsLoggerAdapter{logger: backend.Logger}
			s.Debug = aws.LogDebugWithSigning
		}
	}
	return v4.NewSigner(c, signerOpts), nil
}

func copyHeaderWithoutOverwrite(dst, src http.Header) {
	for k, vv := range src {
		if _, ok := dst[k]; !ok {
			for _, v := range vv {
				dst.Add(k, v)
			}
		}
	}
}

func validateConfig(cfg *SigV4Config) error {
	_, err := auth.ToAuthType(cfg.AuthType)
	if err != nil {
		return err
	}

	return nil
}

func (m *middleware) logRequest(req *http.Request, args ...interface{}) {
	if !m.verboseMode {
		return
	}
	dump, err := httputil.DumpRequest(req, true)
	if err != nil {
		backend.Logger.Error("Unable to dump request", "err", err)
	}
	backend.Logger.Debug("Request dump", append([]interface{}{"dump", string(dump)}, args...)...)
}

type awsLoggerAdapter struct {
	logger log.Logger
}

func (a awsLoggerAdapter) Log(args ...interface{}) {
	a.logger.Debug("[AWS SigV4 log]", "args", args)
}
