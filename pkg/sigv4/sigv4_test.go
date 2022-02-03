package sigv4

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("Can't create new middleware without valid auth type", func(t *testing.T) {
		rt, err := New(&Config{}, nil)
		require.Error(t, err)
		require.Nil(t, rt)

	})
	t.Run("Can create new middleware with any valid auth type", func(t *testing.T) {
		for _, authType := range []string{"credentials", "sharedCreds", "keys", "default", "ec2_iam_role", "arn"} {
			rt, err := New(&Config{AuthType: authType}, nil)

			require.NoError(t, err)
			require.NotNil(t, rt)
		}
	})

	t.Run("Can sign a request", func(t *testing.T) {
		cfg := &Config{AuthType: "default"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		signerCache.Store(cfg.asSha256(), v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

		res, err := rt.RoundTrip(r)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, r.Host, res.Request.Host)
		require.Equal(t, r.URL, res.Request.URL)
		require.Equal(t, r.RequestURI, res.Request.RequestURI)
		require.Equal(t, r.Method, res.Request.Method)
		require.NotNil(t, res.Request.Body)
		require.Equal(t, r.ContentLength, res.Request.ContentLength)

		authHeader := res.Request.Header.Get("Authorization")
		require.NotEmpty(t, authHeader)
		require.True(t, strings.Contains(authHeader, "SignedHeaders=host;x-amz-date,"))
		require.NotEmpty(t, res.Request.Header.Get("X-Amz-Date"))
	})

	t.Run("Can sign a request with extra headers which are not signed", func(t *testing.T) {
		cfg := &Config{AuthType: "default"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		r.Header.Add("Foo", "Bar")

		// mock signer
		signerCache.Store(cfg.asSha256(), v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

		res, err := rt.RoundTrip(r)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, r.Host, res.Request.Host)
		require.Equal(t, r.URL, res.Request.URL)
		require.Equal(t, r.RequestURI, res.Request.RequestURI)
		require.Equal(t, r.Method, res.Request.Method)
		require.NotNil(t, res.Request.Body)
		require.Equal(t, r.ContentLength, res.Request.ContentLength)

		authHeader := res.Request.Header.Get("Authorization")
		require.NotEmpty(t, authHeader)
		require.True(t, strings.Contains(authHeader, "SignedHeaders=host;x-amz-date,"))
		require.NotEmpty(t, res.Request.Header.Get("X-Amz-Date"))
		require.Equal(t, "Bar", res.Request.Header.Get("Foo"))
	})

	t.Run("Signed request overwrites existing Authorization header", func(t *testing.T) {
		cfg := &Config{AuthType: "default"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		r.Header.Add("Authorization", "test")

		// mock signer
		signerCache.Store(cfg.asSha256(), v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

		res, err := rt.RoundTrip(r)
		require.NoError(t, err)
		require.NotNil(t, res)

		authHeader := res.Request.Header.Get("Authorization")
		require.NotEqual(t, "test", authHeader)
		require.True(t, strings.Contains(authHeader, "AWS4-HMAC-SHA256"))
		require.True(t, strings.Contains(authHeader, "SignedHeaders="))
		require.True(t, strings.Contains(authHeader, "Signature="))
	})

	t.Run("Can't sign a request without valid credentials", func(t *testing.T) {
		cfg := &Config{AuthType: "ec2_iam_role"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		signerCache.Store(cfg.asSha256(), v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{noCredentials: true})))

		res, err := rt.RoundTrip(r)
		require.Error(t, err)
		require.Nil(t, res)
	})

	t.Run("Will log requests during signing if configured", func(t *testing.T) {
		cfg := &Config{AuthType: "ec2_iam_role"}
		logger := &fakeLogger{}
		rt, err := NewWithLogger(cfg, &fakeTransport{}, logger)
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		signerCache.Store(cfg.asSha256(), v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

		res, err := rt.RoundTrip(r)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, 2, len(logger.requestsLogged))
		require.Equal(t, r, logger.requestsLogged[0])
		require.Equal(t, res.Request, logger.requestsLogged[1])
	})
}

func TestConfig(t *testing.T) {
	t.Run("SHA generation is consistent", func(t *testing.T) {
		cfg1 := &Config{
			AuthType:      "A",
			Profile:       "B",
			Service:       "C",
			AccessKey:     "D",
			SecretKey:     "E",
			SessionToken:  "F",
			AssumeRoleARN: "G",
			ExternalID:    "H",
			Region:        "I",
		}

		sha1, sha2 := cfg1.asSha256(), cfg1.asSha256()
		require.Equal(t, sha1, sha2)
	})

	t.Run("Config field order does not affect SHA", func(t *testing.T) {
		cfg1 := &Config{
			AuthType:      "A",
			Profile:       "B",
			Service:       "C",
			AccessKey:     "D",
			SecretKey:     "E",
			SessionToken:  "F",
			AssumeRoleARN: "G",
			ExternalID:    "H",
			Region:        "I",
		}

		cfg2 := &Config{
			Region:        "I",
			ExternalID:    "H",
			AssumeRoleARN: "G",
			SessionToken:  "F",
			SecretKey:     "E",
			AccessKey:     "D",
			Service:       "C",
			Profile:       "B",
			AuthType:      "A",
		}

		sha1, sha2 := cfg1.asSha256(), cfg2.asSha256()
		require.Equal(t, sha1, sha2)
	})

	t.Run("Config SHA changes depending on contents", func(t *testing.T) {
		cfg1 := &Config{
			AuthType:      "A",
			Profile:       "B",
			Service:       "C",
			AccessKey:     "D",
			SecretKey:     "E",
			SessionToken:  "F",
			AssumeRoleARN: "G",
			ExternalID:    "H",
			Region:        "I",
		}

		cfg2 := &Config{
			AuthType:      "AB",
			Profile:       "B",
			Service:       "C",
			AccessKey:     "D",
			SecretKey:     "E",
			SessionToken:  "F",
			AssumeRoleARN: "G",
			ExternalID:    "H",
			Region:        "I",
		}

		sha1, sha2 := cfg1.asSha256(), cfg2.asSha256()
		require.NotEqual(t, sha1, sha2)

		cfg2.AuthType = "A"

		sha2 = cfg2.asSha256()
		require.Equal(t, sha1, sha2)
	})
}

type mockCredentialsProvider struct {
	credentials.Provider
	noCredentials bool
}

func (m *mockCredentialsProvider) Retrieve() (credentials.Value, error) {
	if m.noCredentials {
		return credentials.Value{}, fmt.Errorf("no valid credentials")
	}
	return credentials.Value{}, nil
}

type fakeTransport struct{}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Header:     make(http.Header),
		Request:    req,
		StatusCode: http.StatusOK,
	}, nil
}

type fakeLogger struct {
	Logger

	requestsLogged []*http.Request
}

func (l *fakeLogger) Log(_ ...interface{}) {

}
func (l *fakeLogger) LogRequest(req *http.Request, _ ...interface{}) {
	l.requestsLogged = append(l.requestsLogged, req)
}

func (l *fakeLogger) VerboseMode() bool {
	return false
}
