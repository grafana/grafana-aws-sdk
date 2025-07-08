package backend

import (
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/experimental/status"
)

// ErrorSource type defines the source of the error
type ErrorSource = status.Source
type ErrorWithSource = status.ErrorWithSource

const (
	// ErrorSourcePlugin error originates from plugin.
	ErrorSourcePlugin = status.SourcePlugin

	// ErrorSourceDownstream error originates from downstream service.
	ErrorSourceDownstream = status.SourceDownstream
	// DefaultErrorSource is the default [ErrorSource] that should be used when it is not explicitly set.
)

func NewErrorWithSource(err error, source ErrorSource) ErrorWithSource {
	return status.NewErrorWithSource(err, source)
}

// ErrorSourceFromHTTPStatus returns an [ErrorSource] based on provided HTTP status code.
func ErrorSourceFromHTTPStatus(statusCode int) ErrorSource {
	return status.SourceFromHTTPStatus(statusCode)
}

// IsDownstreamError return true if provided error is an error with downstream source or
// a timeout error or a cancelled error.
func IsDownstreamError(err error) bool {
	return status.IsDownstreamError(err)
}

// IsDownstreamError return true if provided error is an error with downstream source or
// a HTTP timeout error or a cancelled error or a connection reset/refused error or dns not found error.
func IsDownstreamHTTPError(err error) bool {
	return status.IsDownstreamHTTPError(err)
}

// DownstreamError creates a new error with status [ErrorSourceDownstream].
func DownstreamError(err error) error {
	return status.DownstreamError(err)
}

// PluginError creates a new error with status [ErrorSourcePlugin].
func PluginError(err error) error {
	return status.PluginError(err)
}
