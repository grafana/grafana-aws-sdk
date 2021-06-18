package sigv4

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/aws/aws-sdk-go/private/protocol/rest"
	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
)

// Host header is likely not necessary here
// (see https://github.com/golang/go/blob/cad6d1fef5147d31e94ee83934c8609d3ad150b7/src/net/http/request.go#L92)
// but adding for completeness
var permittedHeaders = map[string]struct{}{
	"Host":            {},
	"Uber-Trace-Id":   {},
	"User-Agent":      {},
	"Accept":          {},
	"Accept-Encoding": {},
	"Content-Type":    {},
	"Content-Length":  {},
	"securitytenant":  {},
	"sgtenant":        {},
	"kbn-xsrf":        {},
}

var (
	authSettings *awsds.AuthSettings = nil
)

type middleware struct {
	config *Config
	next   http.RoundTripper

	signer *v4.Signer
}

type Config struct {
	AuthType string

	Profile string

	Service string

	AccessKey string
	SecretKey string

	AssumeRoleARN string
	ExternalID    string
	Region        string
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
func New(config *Config, next http.RoundTripper) http.RoundTripper {
	// Need to delay fetching auth settings until env vars have had a chance to propagate
	if authSettings == nil {
		authSettings = awsds.ReadAuthSettingsFromEnvironmentVariables()
	}

	return RoundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if next == nil {
			next = http.DefaultTransport
		}
		return (&middleware{
			config: config,
			next:   next,
		}).exec(r)
	})
}

func (m *middleware) exec(req *http.Request) (*http.Response, error) {
	_, err := m.signRequest(req)
	if err != nil {
		return nil, err
	}

	return m.next.RoundTrip(req)
}

func (m *middleware) signRequest(req *http.Request) (http.Header, error) {
	if m.signer == nil {
		signer, err := newSigner(m.config)
		if err != nil {
			return nil, err
		}
		m.signer = signer
	}

	body, err := replaceBody(req)
	if err != nil {
		return nil, err
	}

	if strings.Contains(req.URL.RawPath, "%2C") {
		req.URL.RawPath = rest.EscapePath(req.URL.RawPath, false)
	}

	stripHeaders(req)

	return m.signer.Sign(req, bytes.NewReader(body), m.config.Service, m.config.Region, time.Now().UTC())
}

func newSigner(config *Config) (*v4.Signer, error) {
	if config == nil {
		return nil, errors.New("configuration must be provided")
	}

	authType := awsds.ToAuthType(config.AuthType)

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

	if config.AssumeRoleARN != "" && !authSettings.AssumeRoleEnabled {
		return nil, fmt.Errorf("attempting to use assume role (ARN) for SigV4 which is not enabled")
	}

	var c *credentials.Credentials
	switch authType {
	case awsds.AuthTypeKeys:
		c = credentials.NewStaticCredentials(config.AccessKey, config.SecretKey, "")
	case awsds.AuthTypeSharedCreds:
		c = credentials.NewSharedCredentials("", config.Profile)
	case awsds.AuthTypeEC2IAMRole:
		s, err := session.NewSession(&aws.Config{
			Region: aws.String(config.Region),
		})
		if err != nil {
			return nil, err
		}
		c = credentials.NewCredentials(
			&ec2rolecreds.EC2RoleProvider{
				Client:       ec2metadata.New(s),
				ExpiryWindow: stscreds.DefaultDuration,
			},
		)

		return v4.NewSigner(c), nil
	case awsds.AuthTypeDefault:
		s, err := session.NewSession(&aws.Config{
			Region: aws.String(config.Region),
		})
		if err != nil {
			return nil, err
		}

		if config.AssumeRoleARN != "" {
			return v4.NewSigner(stscreds.NewCredentials(s, config.AssumeRoleARN)), nil
		}

		return v4.NewSigner(s.Config.Credentials), nil
	default:
		return nil, fmt.Errorf("invalid SigV4 auth type")
	}

	if config.AssumeRoleARN != "" {
		s, err := session.NewSession(
			&aws.Config{
				Region:      aws.String(config.Region),
				Credentials: c,
			},
		)
		if err != nil {
			return nil, err
		}

		return v4.NewSigner(stscreds.NewCredentials(s, config.AssumeRoleARN)), nil
	}

	return v4.NewSigner(c), nil
}

func replaceBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return []byte{}, nil
	}

	payload, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	req.Body = ioutil.NopCloser(bytes.NewReader(payload))
	return payload, nil
}

func stripHeaders(req *http.Request) {
	for h := range req.Header {
		if _, exists := permittedHeaders[h]; !exists {
			req.Header.Del(h)
		}
	}
}
