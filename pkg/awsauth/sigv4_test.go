package awsauth

import (
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/grafana/grafana-plugin-sdk-go/backend/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
