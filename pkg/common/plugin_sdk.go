package common

import (
	"context"
	"net"
	"net/http"
	"time"
)

// NewHTTPTransport returns a new HTTP Transport, based off the definition in
// the stdlib http.DefaultTransport. It's not a clone, because that would return
// any mutations of http.DefaultTransport from other code at the time of the call.
// Any plugin that needs a default http transport should use this function.
//
// Copied from grafana-plugin-sdk-go v0.278.0.
func NewHTTPTransport() http.RoundTripper {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
			return dialer.DialContext
		}(&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}),
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}
