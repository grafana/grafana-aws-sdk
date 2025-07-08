package awsds

import (
	"time"
)

// AuthSettings stores the AWS settings from Grafana
type AuthSettings struct {
	AllowedAuthProviders       []string
	AssumeRoleEnabled          bool
	SessionDuration            *time.Duration
	ExternalID                 string
	ListMetricsPageLimit       int
	MultiTenantTempCredentials bool

	// necessary for a work around until https://github.com/grafana/grafana/issues/39089 is implemented
	SecureSocksDSProxyEnabled bool
}

// SigV4Settings stores the settings for SigV4 authentication
type SigV4Settings struct {
	Enabled        bool
	VerboseLogging bool
}
