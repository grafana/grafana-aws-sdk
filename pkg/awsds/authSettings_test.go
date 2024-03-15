package awsds

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
	"github.com/stretchr/testify/require"
)

func TestReadAuthSettingsFromContext(t *testing.T) {
	tcs := []struct {
		name                string
		cfg                 *backend.GrafanaCfg
		expectedSettings    *AuthSettings
		expectedHasSettings bool
	}{
		{
			name:                "nil config",
			cfg:                 nil,
			expectedSettings:    defaultAuthSettings(),
			expectedHasSettings: false,
		},
		{
			name:                "empty config",
			cfg:                 &backend.GrafanaCfg{},
			expectedSettings:    defaultAuthSettings(),
			expectedHasSettings: false,
		},
		{
			name:                "nil config map",
			cfg:                 backend.NewGrafanaCfg(nil),
			expectedSettings:    defaultAuthSettings(),
			expectedHasSettings: false,
		},
		{
			name:                "empty config map",
			cfg:                 backend.NewGrafanaCfg(make(map[string]string)),
			expectedSettings:    defaultAuthSettings(),
			expectedHasSettings: false,
		},
		{
			name: "aws settings in config",
			cfg: backend.NewGrafanaCfg(map[string]string{
				AllowedAuthProvidersEnvVarKeyName:   "foo , bar,baz",
				AssumeRoleEnabledEnvVarKeyName:      "false",
				GrafanaAssumeRoleExternalIdKeyName:  "mock_id",
				ListMetricsPageLimitKeyName:         "50",
				proxy.PluginSecureSocksProxyEnabled: "true",
			}),
			expectedSettings: &AuthSettings{
				AllowedAuthProviders:      []string{"foo", "bar", "baz"},
				AssumeRoleEnabled:         false,
				ExternalID:                "mock_id",
				ListMetricsPageLimit:      50,
				SessionDuration:           &stscreds.DefaultDuration,
				SecureSocksDSProxyEnabled: true,
			},
			expectedHasSettings: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := backend.WithGrafanaConfig(context.Background(), tc.cfg)
			settings, hasSettings := ReadAuthSettingsFromContext(ctx)

			require.Equal(t, tc.expectedHasSettings, hasSettings)
			require.Equal(t, tc.expectedSettings, settings)
		})
	}
}

func TestReadAuthSettings(t *testing.T) {
	originalExternalId := os.Getenv(GrafanaAssumeRoleExternalIdKeyName)
	os.Setenv(GrafanaAssumeRoleExternalIdKeyName, "env_id")
	defer func() {
		os.Setenv(GrafanaAssumeRoleExternalIdKeyName, originalExternalId)
	}()

	ctxDuration := 10 * time.Minute
	envDuration := 20 * time.Minute
	expectedSessionContextSettings := &AuthSettings{
		AllowedAuthProviders:      []string{"foo", "bar", "baz"},
		AssumeRoleEnabled:         false,
		SessionDuration:           &ctxDuration,
		ExternalID:                "mock_id",
		ListMetricsPageLimit:      50,
		SecureSocksDSProxyEnabled: true,
	}

	expectedSessionEnvSettings := &AuthSettings{
		AllowedAuthProviders:      []string{"default", "keys", "credentials"},
		AssumeRoleEnabled:         true,
		SessionDuration:           &envDuration,
		ExternalID:                "env_id",
		ListMetricsPageLimit:      30,
		SecureSocksDSProxyEnabled: false,
	}

	require.NoError(t, os.Setenv(ListMetricsPageLimitKeyName, "30"))
	require.NoError(t, os.Setenv(SessionDurationEnvVarKeyName, "20m"))
	require.NoError(t, os.Setenv(proxy.PluginSecureSocksProxyEnabled, "false"))
	defer unsetEnvironmentVariables()

	tcs := []struct {
		name             string
		cfg              *backend.GrafanaCfg
		expectedSettings *AuthSettings
	}{
		{
			name:             "read from env if config is nil",
			cfg:              nil,
			expectedSettings: expectedSessionEnvSettings,
		},
		{
			name:             "read from env if config is empty",
			cfg:              &backend.GrafanaCfg{},
			expectedSettings: expectedSessionEnvSettings,
		},
		{
			name:             "read from env if config map is nil",
			cfg:              backend.NewGrafanaCfg(nil),
			expectedSettings: expectedSessionEnvSettings,
		},
		{
			name:             "read from env if config map is empty",
			cfg:              backend.NewGrafanaCfg(make(map[string]string)),
			expectedSettings: expectedSessionEnvSettings,
		},
		{
			name: "read from context",
			cfg: backend.NewGrafanaCfg(map[string]string{
				AllowedAuthProvidersEnvVarKeyName:   "foo , bar,baz",
				AssumeRoleEnabledEnvVarKeyName:      "false",
				SessionDurationEnvVarKeyName:        "10m",
				GrafanaAssumeRoleExternalIdKeyName:  "mock_id",
				ListMetricsPageLimitKeyName:         "50",
				proxy.PluginSecureSocksProxyEnabled: "true",
			}),
			expectedSettings: expectedSessionContextSettings,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := backend.WithGrafanaConfig(context.Background(), tc.cfg)
			settings := ReadAuthSettings(ctx)

			require.Equal(t, tc.expectedSettings, settings)
		})
	}
}

func TestReadSigV4Settings(t *testing.T) {
	tcs := []struct {
		name             string
		cfg              *backend.GrafanaCfg
		expectedSettings *SigV4Settings
	}{
		{
			name:             "empty config map",
			cfg:              backend.NewGrafanaCfg(make(map[string]string)),
			expectedSettings: &SigV4Settings{},
		},
		{
			name: "aws settings in config",
			cfg: backend.NewGrafanaCfg(map[string]string{
				SigV4AuthEnabledEnvVarKeyName:    "true",
				SigV4VerboseLoggingEnvVarKeyName: "true",
			}),
			expectedSettings: &SigV4Settings{
				Enabled:        true,
				VerboseLogging: true,
			},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := backend.WithGrafanaConfig(context.Background(), tc.cfg)
			settings := ReadSigV4Settings(ctx)

			require.Equal(t, tc.expectedSettings, settings)
		})
	}
}

func unsetEnvironmentVariables() {
	os.Unsetenv(AllowedAuthProvidersEnvVarKeyName)
	os.Unsetenv(AssumeRoleEnabledEnvVarKeyName)
	os.Unsetenv(SessionDurationEnvVarKeyName)
	os.Unsetenv(ListMetricsPageLimitKeyName)
}
