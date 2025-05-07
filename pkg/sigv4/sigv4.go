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
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/private/protocol/rest"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
)

var (
	signerCache sync.Map
)

type middleware struct {
	signer *v4.Signer
	config *Config
	next   http.RoundTripper

	verboseMode bool
}

type Config struct {
	AuthType string

	Profile string

	Service string

	AccessKey    string
	SecretKey    string
	SessionToken string

	AssumeRoleARN string
	ExternalID    string
	Region        string
}

type Opts struct {
	VerboseMode bool
}

func (c Config) asSha256() (string, error) {
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

// New instantiates a new signing middleware with an optional succeeding
// middleware. The http.DefaultTransport will be used if nil
// AuthSettings can be gotten from the datasource instance's context with awsds.ReadAuthSettingsFromContext
func New(cfg *Config, authSettings awsds.AuthSettings, next http.RoundTripper, opts ...Opts) (http.RoundTripper, error) {
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
			signer, err = createSigner(cfg, authSettings, sigv4Opts.VerboseMode)
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

func cachedSigner(cfg *Config) (*v4.Signer, bool) {
	sha, err := cfg.asSha256()
	if err != nil {
		return nil, false
	}

	if cached, exists := signerCache.Load(sha); exists {
		return cached.(*v4.Signer), true
	}
	return nil, false
}

func createSigner(cfg *Config, authSettings awsds.AuthSettings, verboseMode bool) (*v4.Signer, error) {
	authType, err := awsds.ToAuthType(cfg.AuthType)
	if err != nil {
		return nil, err
	}

	authTypeAllowed := false
	for _, provider := range authSettings.AllowedAuthProviders {
		if provider == authType.String() {
			authTypeAllowed = true
			break
		}
	}

	if !authTypeAllowed {
		return nil, fmt.Errorf("attempting to use an auth type for SigV4 that is not allowed: %q", authType.String())
	}

	if cfg.AssumeRoleARN != "" && !authSettings.AssumeRoleEnabled {
		return nil, fmt.Errorf("attempting to use assume role (ARN) for SigV4 which is not enabled")
	}

	var signerOpts = func(s *v4.Signer) {
		if verboseMode {
			s.Logger = awsLoggerAdapter{logger: backend.Logger}
			s.Debug = aws.LogDebugWithSigning
		}
	}

	var c *credentials.Credentials
	switch authType {
	case awsds.AuthTypeKeys:
		c = credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken)
	case awsds.AuthTypeSharedCreds:
		c = credentials.NewSharedCredentials("", cfg.Profile)
	case awsds.AuthTypeEC2IAMRole:
		s, err := session.NewSession(&aws.Config{
			Region: aws.String(cfg.Region),
		})
		if err != nil {
			return nil, err
		}
		c = credentials.NewCredentials(defaults.RemoteCredProvider(*s.Config, s.Handlers))

		if cfg.AssumeRoleARN != "" {
			s, err = session.NewSession(&aws.Config{
				CredentialsChainVerboseErrors: aws.Bool(true),
				Region:                        aws.String(cfg.Region),
				Credentials:                   c,
			})
			if err != nil {
				return nil, err
			}
			c = stscreds.NewCredentials(s, cfg.AssumeRoleARN)
		}

		return v4.NewSigner(c, signerOpts), nil
	case awsds.AuthTypeDefault:
		s, err := session.NewSession(&aws.Config{
			Region: aws.String(cfg.Region),
		})
		if err != nil {
			return nil, err
		}

		if cfg.AssumeRoleARN != "" {
			return getAssumeRoleSigner(s, cfg, signerOpts)
		}

		return v4.NewSigner(s.Config.Credentials, signerOpts), nil
	default:
		if cfg.AssumeRoleARN != "" {
			s, err := session.NewSession(&aws.Config{
				Region: aws.String(cfg.Region),
			})
			if err != nil {
				return nil, err
			}
			return getAssumeRoleSigner(s, cfg, signerOpts)
		}
		return nil, fmt.Errorf("invalid SigV4 auth type %q", authType)
	}

	if cfg.AssumeRoleARN != "" {
		s, err := session.NewSession(&aws.Config{
			Region:      aws.String(cfg.Region),
			Credentials: c},
		)
		if err != nil {
			return nil, err
		}
		return getAssumeRoleSigner(s, cfg, signerOpts)
	}

	return v4.NewSigner(c, signerOpts), nil
}

func getAssumeRoleSigner(s *session.Session  , cfg *Config, signerOpts func(s *v4.Signer)) (*v4.Signer, error) {
	if cfg.ExternalID != "" {
		return v4.NewSigner(stscreds.NewCredentials(s, cfg.AssumeRoleARN, func(p *stscreds.AssumeRoleProvider) {
			p.ExternalID = aws.String(cfg.ExternalID)
		}), signerOpts), nil
	} 
	return v4.NewSigner(stscreds.NewCredentials(s, cfg.AssumeRoleARN), signerOpts), nil
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

func validateConfig(cfg *Config) error {
	_, err := awsds.ToAuthType(cfg.AuthType)
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
