package sigv4

import (
	"fmt"
	"net/http"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
)

// SigV4MiddlewareName the middleware name used by SigV4Middleware.
const SigV4MiddlewareName = "sigv4"

var newSigV4Func = New

// SigV4MiddlewareWithAuthSettings applies AWS Signature Version 4 request signing for the outgoing request.
// AuthSettings can be gotten from the datasource instance's context with awsds.ReadAuthSettingsFromContext
func SigV4MiddlewareWithAuthSettings(verboseLogging bool, authSettings awsds.AuthSettings) httpclient.Middleware {
	return httpclient.NamedMiddlewareFunc(SigV4MiddlewareName, func(opts httpclient.Options, next http.RoundTripper) http.RoundTripper {
		if opts.SigV4 == nil {
			return next
		}

		conf := &Config{
			Service:       opts.SigV4.Service,
			AccessKey:     opts.SigV4.AccessKey,
			SecretKey:     opts.SigV4.SecretKey,
			Region:        opts.SigV4.Region,
			AssumeRoleARN: opts.SigV4.AssumeRoleARN,
			AuthType:      opts.SigV4.AuthType,
			ExternalID:    opts.SigV4.ExternalID,
			Profile:       opts.SigV4.Profile,
		}

		rt, err := newSigV4Func(conf, authSettings, next, Opts{VerboseMode: verboseLogging})
		if err != nil {
			return invalidSigV4Config(err)
		}

		return rt
	})
}

// SigV4Middleware applies AWS Signature Version 4 request signing for the outgoing request.
// Deprecated: Use SigV4MiddlewareWithAuthSettings instead
func SigV4Middleware(verboseLogging bool) httpclient.Middleware {
	return SigV4MiddlewareWithAuthSettings(verboseLogging, *awsds.ReadAuthSettingsFromEnvironmentVariables())
}

func invalidSigV4Config(err error) http.RoundTripper {
	return httpclient.RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("invalid SigV4 configuration: %w", err)
	})
}
