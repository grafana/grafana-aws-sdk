package backend

import (
	"github.com/grafana/grafana-aws-sdk-for-backport/pkg/experimental/status"
)

// DownstreamError creates a new error with status [ErrorSourceDownstream].
func DownstreamError(err error) error {
	return status.DownstreamError(err)
}
