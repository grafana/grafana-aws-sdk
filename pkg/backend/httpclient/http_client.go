package httpclient

import (
	"net/http"
)

// The RoundTripperFunc type is an adapter to allow the use of ordinary
// functions as RoundTrippers. If f is a function with the appropriate
// signature, RountTripperFunc(f) is a RoundTripper that calls f.
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

// Middleware is an interface representing the ability to create a middleware
// that implements the http.RoundTripper interface.
type Middleware interface {
	// CreateMiddleware creates a new middleware.
	CreateMiddleware(opts Options, next http.RoundTripper) http.RoundTripper
}

// The MiddlewareFunc type is an adapter to allow the use of ordinary
// functions as Middlewares. If f is a function with the appropriate
// signature, MiddlewareFunc(f) is a Middleware that calls f.
type MiddlewareFunc func(opts Options, next http.RoundTripper) http.RoundTripper

// CreateMiddleware implements the Middleware interface.
func (fn MiddlewareFunc) CreateMiddleware(opts Options, next http.RoundTripper) http.RoundTripper {
	return fn(opts, next)
}

// MiddlewareName is an interface representing the ability for a middleware to have a name.
type MiddlewareName interface {
	// MiddlewareName returns the middleware name.
	MiddlewareName() string
}

// ConfigureMiddlewareFunc function signature for configuring middleware chain.
type ConfigureMiddlewareFunc func(opts Options, existingMiddleware []Middleware) []Middleware

type namedMiddleware struct {
	Name       string
	Middleware Middleware
}

// NamedMiddlewareFunc type is an adapter to allow the use of ordinary
// functions as Middleware. If f is a function with the appropriate
// signature, NamedMiddlewareFunc(f) is a Middleware that calls f.
func NamedMiddlewareFunc(name string, fn MiddlewareFunc) Middleware {
	return &namedMiddleware{
		Name:       name,
		Middleware: fn,
	}
}

func (nm *namedMiddleware) CreateMiddleware(opts Options, next http.RoundTripper) http.RoundTripper {
	return nm.Middleware.CreateMiddleware(opts, next)
}

func (nm *namedMiddleware) MiddlewareName() string {
	return nm.Name
}
