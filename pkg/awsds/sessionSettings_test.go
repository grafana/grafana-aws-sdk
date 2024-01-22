package awsds

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/proxy"
	"github.com/stretchr/testify/require"
)

func TestReadSettingsFromContext(t *testing.T) {
	defaultAuthSettings := &AuthSettings{
		AllowedAuthProviders: []string{"default", "keys", "credentials"},
		AssumeRoleEnabled:    true,
		ListMetricsPageLimit: defaultListMetricsPageLimit,
	}
	tcs := []struct {
		name                string
		cfg                 *backend.GrafanaCfg
		expectedSettings    *AuthSettings
		expectedHasSettings bool
	}{
		{
			name:                "nil config",
			cfg:                 nil,
			expectedSettings:    defaultAuthSettings,
			expectedHasSettings: false,
		},
		{
			name:                "empty config",
			cfg:                 &backend.GrafanaCfg{},
			expectedSettings:    defaultAuthSettings,
			expectedHasSettings: false,
		},
		{
			name:                "nil config map",
			cfg:                 backend.NewGrafanaCfg(nil),
			expectedSettings:    defaultAuthSettings,
			expectedHasSettings: false,
		},
		{
			name:                "empty config map",
			cfg:                 backend.NewGrafanaCfg(make(map[string]string)),
			expectedSettings:    defaultAuthSettings,
			expectedHasSettings: false,
		},
		{
			name: "aws settings in config",
			cfg: backend.NewGrafanaCfg(map[string]string{
				AllowedAuthProvidersEnvVarKeyName:   "foo , bar,baz",
				AssumeRoleEnabledEnvVarKeyName:      "false",
				GrafanaAssumeRoleExternalIdKeyName:  "mock_id",
				GrafanaListMetricsPageLimit:         "50",
				proxy.PluginSecureSocksProxyEnabled: "true",
			}),
			expectedSettings: &AuthSettings{
				AllowedAuthProviders:      []string{"foo", "bar", "baz"},
				AssumeRoleEnabled:         false,
				ExternalID:                "mock_id",
				ListMetricsPageLimit:      50,
				SecureSocksDSProxyEnabled: true,
			},
			expectedHasSettings: true,
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := backend.WithGrafanaConfig(context.Background(), tc.cfg)
			settings, hasSettings := ReadSettingsFromContext(ctx)

			require.Equal(t, tc.expectedHasSettings, hasSettings)
			require.Equal(t, tc.expectedSettings, settings)
		})
	}
}

func TestReadSettings(t *testing.T) {
	originalExternalId := os.Getenv(GrafanaAssumeRoleExternalIdKeyName)
	os.Setenv(GrafanaAssumeRoleExternalIdKeyName, "env_id")
	defer func() {
		os.Setenv(GrafanaAssumeRoleExternalIdKeyName, originalExternalId)
	}()

	expectedDuration, err := time.ParseDuration("20m")
	require.NoError(t, err)
	expectedSessionContextSettings := &AuthSettings{
		AllowedAuthProviders:      []string{"foo", "bar", "baz"},
		AssumeRoleEnabled:         false,
		SessionDuration:           &expectedDuration, //20 minutes in nanoseconds count,
		ExternalID:                "mock_id",
		ListMetricsPageLimit:      50,
		SecureSocksDSProxyEnabled: true,
	}

	expectedSessionEnvSettings := &AuthSettings{
		AllowedAuthProviders:      []string{"env1", "env2"},
		AssumeRoleEnabled:         true,
		SessionDuration:           &expectedDuration,
		ExternalID:                "env_id",
		ListMetricsPageLimit:      30,
		SecureSocksDSProxyEnabled: false,
	}

	require.NoError(t, os.Setenv(AllowedAuthProvidersEnvVarKeyName, "env1,env2"))
	require.NoError(t, os.Setenv(AssumeRoleEnabledEnvVarKeyName, "true"))
	require.NoError(t, os.Setenv(GrafanaListMetricsPageLimit, "30"))
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
			name: "reaf from context",
			cfg: backend.NewGrafanaCfg(map[string]string{
				AllowedAuthProvidersEnvVarKeyName:   "foo , bar,baz",
				AssumeRoleEnabledEnvVarKeyName:      "false",
				GrafanaAssumeRoleExternalIdKeyName:  "mock_id",
				GrafanaListMetricsPageLimit:         "50",
				proxy.PluginSecureSocksProxyEnabled: "true",
			}),
			expectedSettings: expectedSessionContextSettings,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := backend.WithGrafanaConfig(context.Background(), tc.cfg)
			settings := ReadSettings(ctx)

			require.Equal(t, tc.expectedSettings, settings)
		})
	}
}
