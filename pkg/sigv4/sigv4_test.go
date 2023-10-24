package sigv4

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"

	"github.com/aws/aws-sdk-go/aws/credentials"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("Can't create new middleware without valid auth type", func(t *testing.T) {
		rt, err := New(&SigV4Config{}, nil)
		require.Error(t, err)
		require.Nil(t, rt)
	})
	t.Run("Can create new middleware with any valid auth type", func(t *testing.T) {
		for _, authType := range []string{"credentials", "sharedCreds", "keys", "default", "ec2_iam_role", "arn", "grafana_assume_role"} {
			rt, err := New(&SigV4Config{AuthType: authType}, nil)

			require.NoError(t, err)
			require.NotNil(t, rt)
		}
	})

	t.Run("Can sign a request", func(t *testing.T) {
		cfg := &SigV4Config{AuthType: "default"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		sha, err := cfg.asSha256()
		require.NoError(t, err)
		signerCache.Store(sha, v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

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
		cfg := &SigV4Config{AuthType: "default"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		r.Header.Add("Foo", "Bar")

		// mock signer
		sha, err := cfg.asSha256()
		require.NoError(t, err)
		signerCache.Store(sha, v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

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
		cfg := &SigV4Config{AuthType: "default"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		r.Header.Add("Authorization", "test")

		// mock signer
		sha, err := cfg.asSha256()
		require.NoError(t, err)
		signerCache.Store(sha, v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

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
		cfg := &SigV4Config{AuthType: "ec2_iam_role"}
		rt, err := New(cfg, &fakeTransport{})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		sha, err := cfg.asSha256()
		require.NoError(t, err)
		signerCache.Store(sha, v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{noCredentials: true})))

		res, err := rt.RoundTrip(r)
		require.Error(t, err)
		require.Nil(t, res)
	})

	t.Run("Will log requests during signing if verboseMode is true", func(t *testing.T) {
		cfg := &SigV4Config{AuthType: "ec2_iam_role"}

		// Mock logger
		origLogger := backend.Logger
		t.Cleanup(func() {
			backend.Logger = origLogger
		})

		fakeLogger := &fakeLogger{}
		backend.Logger = fakeLogger

		rt, err := New(cfg, &fakeTransport{}, Opts{VerboseMode: true})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		sha, err := cfg.asSha256()
		require.NoError(t, err)
		signerCache.Store(sha, v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

		res, err := rt.RoundTrip(r)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Equal(t, 2, len(fakeLogger.logs))
		require.Equal(t, "Request dump", fakeLogger.logs[0])
		require.Equal(t, "Request dump", fakeLogger.logs[1])
	})

	t.Run("Will not log requests during signing if verboseMode is false", func(t *testing.T) {
		cfg := &SigV4Config{AuthType: "ec2_iam_role"}

		// Mock logger
		origLogger := backend.Logger
		t.Cleanup(func() {
			backend.Logger = origLogger
		})

		fakeLogger := &fakeLogger{}
		backend.Logger = fakeLogger

		rt, err := New(cfg, &fakeTransport{}, Opts{VerboseMode: false})
		require.NoError(t, err)
		require.NotNil(t, rt)
		r, err := http.NewRequest("GET", "http://grafana.sigv4.test", nil)
		require.NoError(t, err)

		// mock signer
		sha, err := cfg.asSha256()
		require.NoError(t, err)
		signerCache.Store(sha, v4.NewSigner(credentials.NewCredentials(&mockCredentialsProvider{})))

		res, err := rt.RoundTrip(r)
		require.NoError(t, err)
		require.NotNil(t, res)

		require.Empty(t, fakeLogger.logs)
	})
}

func TestConfig(t *testing.T) {
	t.Run("SHA generation is consistent", func(t *testing.T) {
		cfg := &SigV4Config{
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

		sha1, err := cfg.asSha256()
		require.NoError(t, err)

		sha2, err := cfg.asSha256()
		require.NoError(t, err)

		require.Equal(t, sha1, sha2)
	})

	t.Run("Config field order does not affect SHA", func(t *testing.T) {
		cfg1 := &SigV4Config{
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

		cfg2 := &SigV4Config{
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

		sha1, err := cfg1.asSha256()
		require.NoError(t, err)

		sha2, err := cfg2.asSha256()
		require.NoError(t, err)

		require.Equal(t, sha1, sha2)
	})

	t.Run("Config SHA changes depending on contents", func(t *testing.T) {
		cfg1 := &SigV4Config{
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

		cfg2 := &SigV4Config{
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

		sha1, err := cfg1.asSha256()
		require.NoError(t, err)

		sha2, err := cfg2.asSha256()
		require.NoError(t, err)

		require.NotEqual(t, sha1, sha2)

		cfg2.AuthType = "A"

		sha2, err = cfg2.asSha256()
		require.NoError(t, err)

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
	log.Logger

	logs []string
}

func (l *fakeLogger) Debug(msg string, _ ...interface{}) {
	l.logs = append(l.logs, msg)
}
