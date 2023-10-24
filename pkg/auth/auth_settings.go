package auth

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"
)

// AuthSettings defines whether certain auth types and provider can be used or not
type AuthSettings struct {
	AllowedAuthProviders []string
	AssumeRoleEnabled    bool
	SessionDuration      *time.Duration
}

func ReadAuthSettingsFromEnvironmentVariables() *AuthSettings {
	authSettings := &AuthSettings{}
	allowedAuthProviders := []string{}
	providers := os.Getenv(AllowedAuthProvidersEnvVarKeyName)
	for _, authProvider := range strings.Split(providers, ",") {
		authProvider = strings.TrimSpace(authProvider)
		if authProvider != "" {
			allowedAuthProviders = append(allowedAuthProviders, authProvider)
		}
	}

	if len(allowedAuthProviders) == 0 {
		allowedAuthProviders = []string{"default", "keys", "credentials"}
		backend.Logger.Warn("could not find allowed auth providers. falling back to 'default, keys, credentials'")
	}
	authSettings.AllowedAuthProviders = allowedAuthProviders

	assumeRoleEnabledString := os.Getenv(AssumeRoleEnabledEnvVarKeyName)
	if len(assumeRoleEnabledString) == 0 {
		backend.Logger.Warn("environment variable missing. falling back to enable assume role", "var", AssumeRoleEnabledEnvVarKeyName)
		assumeRoleEnabledString = "true"
	}

	var err error
	authSettings.AssumeRoleEnabled, err = strconv.ParseBool(assumeRoleEnabledString)
	if err != nil {
		backend.Logger.Error("could not parse env variable", "var", AssumeRoleEnabledEnvVarKeyName)
		authSettings.AssumeRoleEnabled = true
	}

	sessionDurationString := os.Getenv(SessionDurationEnvVarKeyName)
	if sessionDurationString != "" {
		sessionDuration, err := gtime.ParseDuration(sessionDurationString)
		if err != nil {
			backend.Logger.Error("could not parse env variable", "var", SessionDurationEnvVarKeyName)
		} else {
			authSettings.SessionDuration = &sessionDuration
		}
	}

	return authSettings
}
