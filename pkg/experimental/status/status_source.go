package status

// Source type defines the status source.
type Source string

const (
	// SourcePlugin status originates from plugin.
	SourcePlugin Source = "plugin"

	// SourceDownstream status originates from downstream service.
	SourceDownstream Source = "downstream"

	// DefaultSource is the default [Source] that should be used when it is not explicitly set.
	DefaultSource Source = SourcePlugin
)

func NewErrorWithSource(err error, source Source) ErrorWithSource {
	return ErrorWithSource{
		source: source,
		err:    err,
	}
}

// IsValid return true if es is [SourceDownstream] or [SourcePlugin].
func (s Source) IsValid() bool {
	return s == SourceDownstream || s == SourcePlugin
}

// String returns the string representation of s. If s is not valid, [DefaultSource] is returned.
func (s Source) String() string {
	if !s.IsValid() {
		return string(DefaultSource)
	}

	return string(s)
}

type ErrorWithSource struct {
	source Source
	err    error
}

// DownstreamError creates a new error with status [SourceDownstream].
func DownstreamError(err error) error {
	return NewErrorWithSource(err, SourceDownstream)
}

func (e ErrorWithSource) ErrorSource() Source {
	return e.source
}

// @deprecated Use [ErrorSource] instead.
func (e ErrorWithSource) Source() Source {
	return e.source
}

func (e ErrorWithSource) Error() string {
	return e.err.Error()
}

// Implements the interface used by [errors.Is].
func (e ErrorWithSource) Is(err error) bool {
	if errWithSource, ok := err.(ErrorWithSource); ok {
		return errWithSource.ErrorSource() == e.source
	}

	return false
}

func (e ErrorWithSource) Unwrap() error {
	return e.err
}
