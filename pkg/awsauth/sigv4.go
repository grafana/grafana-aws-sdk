package awsauth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/signer/v4"

	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

func NewSigV4Middleware(signerOpts ...func(signer *v4.SignerOptions)) httpclient.Middleware {
	return SignerMiddleware{signerOpts}
}

type SignerMiddleware struct {
	signerOpts []func(*v4.SignerOptions)
}

func (s SignerMiddleware) CreateMiddleware(opts httpclient.Options, next http.RoundTripper) http.RoundTripper {
	if opts.SigV4 == nil {
		return next
	}
	return NewSignerRoundTripper(opts, next, v4.NewSigner(s.signerOpts...))
}

func (s SignerMiddleware) MiddlewareName() string {
	return "sigv4"
}

func NewSignerRoundTripper(opts httpclient.Options, next http.RoundTripper, signer v4.HTTPSigner) SignerRoundTripper {
	return SignerRoundTripper{
		sigV4Config:       opts.SigV4,
		customHeaders:     opts.Header,
		next:              next,
		awsConfigProvider: NewConfigProvider(),
		signer:            signer,
		clock:             systemClock{},
	}
}

type SignerRoundTripper struct {
	sigV4Config       *httpclient.SigV4Config
	customHeaders     http.Header
	next              http.RoundTripper
	awsConfigProvider ConfigProvider
	signer            v4.HTTPSigner
	clock             Clock
}

func (s SignerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	awsAuthSettings := Settings{
		AuthType:           AuthType(s.sigV4Config.AuthType),
		AccessKey:          s.sigV4Config.AccessKey,
		SecretKey:          s.sigV4Config.SecretKey,
		Region:             s.sigV4Config.Region,
		CredentialsProfile: s.sigV4Config.Profile,
		AssumeRoleARN:      s.sigV4Config.AssumeRoleARN,
		ExternalID:         s.sigV4Config.ExternalID,
		// TODO: support PDC:
		//ProxyOptions:       nil,
	}
	ctx := req.Context()
	cfg, err := s.awsConfigProvider.GetConfig(ctx, awsAuthSettings)
	if err != nil {
		return nil, err
	}
	credentials, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return nil, err
	}
	err = s.SignHTTP(ctx, req, credentials)
	if err != nil {
		return nil, err
	}
	return s.next.RoundTrip(req)
}

func (s SignerRoundTripper) SignHTTP(ctx context.Context, req *http.Request, credentials aws.Credentials) error {
	// remove any custom headers from req, so they don't get included in the signature
	for k := range s.customHeaders {
		req.Header.Del(k)
	}
	defer func() {
		// replace the custom headers before returning
		for k, v := range s.customHeaders {
			req.Header[k] = v
		}
	}()
	payloadHash, err := getRequestBodyHash(req)
	if err != nil {
		return err
	}
	return s.signer.SignHTTP(ctx, credentials, req, payloadHash, s.sigV4Config.Service, s.sigV4Config.Region, s.clock.Now().UTC())
}

func getRequestBodyHash(req *http.Request) (string, error) {
	body, err := req.GetBody()
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	_, err = io.Copy(hash, body)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil

}

type Clock interface {
	Now() time.Time
}
type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }
