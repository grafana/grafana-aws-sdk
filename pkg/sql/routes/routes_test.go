package routes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/sqlds/v4"
)

func TestWrite(t *testing.T) {
	rw := httptest.NewRecorder()
	msg := []byte("foo")
	Write(rw, msg)
	resp := rw.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(body, msg) {
		t.Errorf("unexpected result: %v", cmp.Diff(body, msg))
	}
}

func TestSendResources(t *testing.T) {
	rw := httptest.NewRecorder()
	SendResources(rw, []string{"foo"}, nil)
	resp := rw.Result()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	expected := []byte(`["foo"]`)
	if !cmp.Equal(body, expected) {
		t.Errorf("unexpected result: %v", cmp.Diff(body, expected))
	}
	if rw.Header().Get("Content-Type") != "application/json" {
		t.Errorf("unexpected Content-Type header: %s", rw.Header().Get("Content-Type"))
	}
}

type fakeDS struct {
	regions   []string
	databases map[string][]string
}

var ds = &fakeDS{
	regions: []string{"us-east1"},
	databases: map[string][]string{
		"us-east-1": {"db1"},
	},
}

func (f *fakeDS) Regions(context.Context) ([]string, error) {
	return f.regions, nil
}

func (f *fakeDS) Databases(_ context.Context, options sqlds.Options) ([]string, error) {
	dbs, ok := f.databases[options["region"]]
	if !ok {
		return nil, fmt.Errorf("error")
	}
	return dbs, nil
}

func (f *fakeDS) CancelQuery(context.Context, sqlds.Options, string) error {
	return nil
}
func TestDefaultRoutes(t *testing.T) {
	tests := []struct {
		description    string
		route          string
		reqBody        []byte
		expectedCode   int
		expectedResult string
	}{
		{
			description:    "return default regions",
			route:          "/regions",
			reqBody:        nil,
			expectedCode:   http.StatusOK,
			expectedResult: `["us-east1"]`,
		},
		{
			description:    "default databases",
			route:          "/databases",
			reqBody:        []byte(`{"region":"us-east-1"}`),
			expectedCode:   http.StatusOK,
			expectedResult: `["db1"]`,
		},
		{
			description:  "wrong region for databases",
			route:        "/databases",
			reqBody:      []byte(`{"region":"us-east-3"}`),
			expectedCode: http.StatusBadRequest,
		},
		{
			description:    "cancel query",
			route:          "/cancel",
			reqBody:        []byte(`{"queryId":"blah"}`),
			expectedCode:   http.StatusOK,
			expectedResult: `"Successfully canceled"`,
		},
		{
			description:  "no queryId for cancel",
			route:        "/cancel",
			reqBody:      []byte(`{"region":"us-east-1"}`),
			expectedCode: http.StatusBadRequest,
		},
	}
	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://example.com/foo", bytes.NewReader(tt.reqBody))
			rw := httptest.NewRecorder()
			rh := ResourceHandler{API: ds}
			routes := rh.DefaultRoutes()
			routes[tt.route](rw, req)

			resp := rw.Result()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}

			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expecting code %v got %v", tt.expectedCode, resp.StatusCode)
			}
			if resp.StatusCode == http.StatusOK && !cmp.Equal(string(body), tt.expectedResult) {
				t.Errorf("unexpected response: %v", cmp.Diff(string(body), tt.expectedResult))
			}
		})
	}
}
