package backend

import (
	"github.com/grafana/grafana-aws-sdk-for-backport/pkg/backend/log"
)

// Logger is the default logger instance. This can be used directly to log from
// your plugin to grafana-server with calls like Logger.Debug(...).
var Logger = log.DefaultLogger
