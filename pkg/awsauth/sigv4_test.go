package awsauth

import (
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

var OnceUponATime = time.Unix(1234567890, 0) // 2009-02-13 UTC
var AtALaterTime = time.Unix(1234567891, 0)  // 2009-02-13 UTC

func TestSignerRoundTripper_SignHTTP(t *testing.T) {
	tests := []struct {
		name           string
		sigV4Config    *httpclient.SigV4Config
		requestBody    string
		customHeaders  http.Header
		differentTimes bool
	}{
		{
			name: "basic success",
			sigV4Config: &httpclient.SigV4Config{
				AuthType:  "keys",
				AccessKey: "good",
				SecretKey: "excellent",
				Region:    "us-east-1",
			},
		},
		{
			name: "with custom headers",
			sigV4Config: &httpclient.SigV4Config{
				AuthType:  "keys",
				AccessKey: "good",
				SecretKey: "excellent",
				Region:    "us-east-1",
			},
			customHeaders: http.Header{"X-Testing-Stuff": []string{"is good"}},
		},
		{
			name: "signature changes with different time",
			sigV4Config: &httpclient.SigV4Config{
				AuthType:  "keys",
				AccessKey: "good",
				SecretKey: "excellent",
				Region:    "us-east-1",
			},
			differentTimes: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := &testRoundTripper{}
			s := NewSignerRoundTripper(httpclient.Options{SigV4: tt.sigV4Config}, next, v4.NewSigner())
			s.awsConfigProvider = NewFakeConfigProvider(false)
			s.clock = staticClock{OnceUponATime}

			req, _ := http.NewRequest("GET", "https://service.aws.amazon.notreally", strings.NewReader(tt.requestBody))
			_, err := s.RoundTrip(req)
			require.NoError(t, err)
			require.NotEmpty(t, req.Header["Authorization"])

			if tt.customHeaders != nil {
				reqWithHeaders, _ := http.NewRequest("GET", "https://service.aws.amazon.notreally", strings.NewReader(tt.requestBody))
				reqWithHeaders.Header = tt.customHeaders
				_, err = s.RoundTrip(reqWithHeaders)
				require.NoError(t, err)

				// custom headers should not affect the signature
				require.Equal(t, req.Header["Authorization"], reqWithHeaders.Header["Authorization"])
				// ... but should be retained
				for k, v := range tt.customHeaders {
					require.Equal(t, v, reqWithHeaders.Header[k])
				}
			}
			if tt.differentTimes {
				s.clock = staticClock{AtALaterTime}
				reqLater, _ := http.NewRequest("GET", "https://service.aws.amazon.notreally", strings.NewReader(tt.requestBody))
				_, err = s.RoundTrip(reqLater)
				require.NoError(t, err)
				require.NotEqual(t, req.Header["Authorization"], reqLater.Header["Authorization"])

			}
		})
	}
}
func Test_getRequestBodyHash(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		expected string
	}{
		{
			name:     "empty body is empty hash",
			body:     "",
			expected: EmptySha256Hash,
		},
		{
			name:     "hello world",
			body:     "hello world",
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("get", "https://whatever.wherever:999", strings.NewReader(tt.body))
			got, _ := getRequestBodyHash(req)
			assert.Equalf(t, tt.expected, got, "getRequestBodyHash(%v)", req)
		})
	}
}

func Test_getRequestBodyHash_serverStyleRequest(t *testing.T) {
	// Requests forwarded by a reverse proxy (e.g. Grafana's datasource proxy)
	// carry a Body but never GetBody, which net/http only sets on client
	// requests. The body must still be hashed, and must remain readable for
	// the downstream round trip.
	tests := []struct {
		name     string
		body     io.ReadCloser
		expected string
	}{
		{
			name:     "nil body is empty hash",
			body:     nil,
			expected: EmptySha256Hash,
		},
		{
			name:     "http.NoBody is empty hash",
			body:     http.NoBody,
			expected: EmptySha256Hash,
		},
		{
			name:     "non-empty body is hashed",
			body:     io.NopCloser(strings.NewReader("hello world")),
			expected: "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("POST", "https://whatever.wherever:999", nil)
			req.Body = tt.body

			got, err := getRequestBodyHash(req)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)

			if tt.body != nil && tt.body != http.NoBody {
				require.NotNil(t, req.Body, "body must remain readable after hashing")
				remaining, err := io.ReadAll(req.Body)
				require.NoError(t, err)
				assert.Equal(t, "hello world", string(remaining), "body must be intact after hashing")
			}
		})
	}
}

func TestSignerRoundTripper_SignHTTP_proxiedRequestMatchesClientRequest(t *testing.T) {
	sigV4Config := &httpclient.SigV4Config{
		AuthType:  "keys",
		AccessKey: "good",
		SecretKey: "excellent",
		Region:    "us-east-1",
	}
	body := "{\"index\":\"metrics-*\"}\n{\"query\":{\"match_all\":{}}}\n"

	next := &testRoundTripper{}
	s := NewSignerRoundTripper(httpclient.Options{SigV4: sigV4Config}, next, v4.NewSigner())
	s.awsConfigProvider = NewFakeConfigProvider(false)
	s.clock = staticClock{OnceUponATime}

	clientReq, _ := http.NewRequest("POST", "https://service.aws.amazon.notreally/_msearch", strings.NewReader(body))
	_, err := s.RoundTrip(clientReq)
	require.NoError(t, err)

	// A proxied request has Body set but GetBody nil, as produced by
	// httputil.ReverseProxy forwarding an inbound server request.
	proxiedReq, _ := http.NewRequest("POST", "https://service.aws.amazon.notreally/_msearch", nil)
	proxiedReq.Body = io.NopCloser(strings.NewReader(body))
	proxiedReq.ContentLength = int64(len(body))
	_, err = s.RoundTrip(proxiedReq)
	require.NoError(t, err)

	require.Equal(t, clientReq.Header["Authorization"], proxiedReq.Header["Authorization"],
		"identical payloads must produce identical signatures regardless of GetBody being set")

	sentBody, err := io.ReadAll(next.seen.Body)
	require.NoError(t, err)
	assert.Equal(t, body, string(sentBody), "proxied request body must reach the next round tripper intact")
}

type staticClock struct {
	when time.Time
}

func (s staticClock) Now() time.Time { return s.when }

type testRoundTripper struct {
	seen *http.Request
}

func (t *testRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	t.seen = request
	return &http.Response{Status: "everything is awesome", StatusCode: 200}, nil
}
