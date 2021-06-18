package sigv4

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkSignRequest(b *testing.B) {
	authSettings = &awsds.AuthSettings{
		AllowedAuthProviders: []string{"credentials"},
	}

	m := middleware{
		config: &Config{
			AuthType:  "credentials",
			AccessKey: "auth-key",
			SecretKey: "secret-key",
		},
	}

	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = m.signRequest(req)
		assert.NoError(b, err)
	}
}

func TestNewSigner_InvalidConfig(t *testing.T) {
	authSettings = &awsds.AuthSettings{
		AllowedAuthProviders: []string{"default"},
	}

	testCases := []struct {
		name   string
		config *Config
	}{
		{
			"nil config",
			nil,
		},
		{
			"undeclared allowed auth provider",
			&Config{
				AuthType: "credentials",
			},
		},
		{
			"forbidden attempt to assume role",
			&Config{
				AuthType: "default",
				AssumeRoleARN: "my-arn",
			},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := newSigner((tc.config))
			assert.Error(t, err)
		})
	}
}

func TestReplaceBody(t *testing.T) {
	testPayload := `{ "id": "me" }`

	body := []byte(testPayload)

	req, err := http.NewRequest(http.MethodGet, "", bytes.NewReader(body))
	require.NoError(t, err)

	payload, err := replaceBody(req)
	assert.NoError(t, err)
	assert.Equal(t, testPayload, string(payload))
}

func TestReplaceBody_NilBody(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	body, err := replaceBody(req)

	assert.NoError(t, err)
	assert.Empty(t, body)
}

func TestStripHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	dummyHeaderValue := []string{"-"}

	// Set the header map directly to avoid canonicalization
	for k := range permittedHeaders {
		req.Header[k] = dummyHeaderValue
	}

	// Add another header that should be stripped
	req.Header.Set("Another-Header", "test")

	stripHeaders(req)

	for k := range permittedHeaders {
		v, ok := req.Header[k]
		assert.True(t, ok)
		assert.Equal(t, dummyHeaderValue, v)
	}
	assert.Empty(t, req.Header.Get("Another-Header"))
}
