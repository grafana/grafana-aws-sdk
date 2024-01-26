package awsds

import (
	"context"
	"os"
	"strconv"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/gtime"
	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
)

// ReadAuthSettings gets the Grafana auth settings from the context if its available, the environment variables if not
func ReadAuthSettings(ctx context.Context) *AuthSettings {
	settings, exists := ReadAuthSettingsFromContext(ctx)
	if !exists {
		settings = ReadAuthSettingsFromEnvironmentVariables()
	}
	return settings
}

// ReadAuthSettingsFromContext tries to get the auth settings from the GrafanaConfig in ctx, and returns true if it finds a config
func ReadAuthSettingsFromContext(ctx context.Context) (*AuthSettings, bool) {
	cfg := backend.GrafanaConfigFromContext(ctx)
	// initialize settings with the default values set
	settings := &AuthSettings{}
	if cfg == nil {
		return settings, false
	}
	hasSettings := false

	allowedAuthProviders := []string{}
	if providers := cfg.Get(AllowedAuthProvidersEnvVarKeyName); providers != "" {
		for _, authProvider := range strings.Split(providers, ",") {
			authProvider = strings.TrimSpace(authProvider)
			if authProvider != "" {
				allowedAuthProviders = append(allowedAuthProviders, authProvider)
			}
		}
		if len(allowedAuthProviders) != 0 {
			settings.AllowedAuthProviders = allowedAuthProviders
		}
		hasSettings = true
	}

	var err error
	if v := cfg.Get(AssumeRoleEnabledEnvVarKeyName); v != "" {
		settings.AssumeRoleEnabled, err = strconv.ParseBool(v)
		if err != nil {
			backend.Logger.Error("could not parse context variable", "var", AssumeRoleEnabledEnvVarKeyName)
			// assume role enabled defaults to on
			settings.AssumeRoleEnabled = true
		}
		hasSettings = true
	}

	if v := cfg.Get(GrafanaAssumeRoleExternalIdKeyName); v != "" {
		settings.ExternalID = v
		hasSettings = true
	}

	if v := cfg.Get(GrafanaListMetricsPageLimit); v != "" {
		settings.ListMetricsPageLimit, err = strconv.Atoi(v)
		if err != nil {
			backend.Logger.Error("could not parse context variable", "var", GrafanaListMetricsPageLimit)
			settings.ListMetricsPageLimit = defaultListMetricsPageLimit
		}
		hasSettings = true
	}

	if v := cfg.Get(proxy.PluginSecureSocksProxyEnabled); v != "" {
		settings.SecureSocksDSProxyEnabled, err = strconv.ParseBool(v)
		if err != nil {
			backend.Logger.Error("could not parse context variable", "var", proxy.PluginSecureSocksProxyEnabled)
			settings.SecureSocksDSProxyEnabled = false
		}
		hasSettings = true
	}

	// Users set session duration directly as an environment variable
	sessionDurationString := os.Getenv(SessionDurationEnvVarKeyName)
	if sessionDurationString != "" {
		sessionDuration, err := gtime.ParseDuration(sessionDurationString)
		if err != nil {
			backend.Logger.Error("could not parse env variable", "var", SessionDurationEnvVarKeyName)
		} else {
			settings.SessionDuration = &sessionDuration
		}
	}

	return settings, hasSettings
}
