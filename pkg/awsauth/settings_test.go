package awsauth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetProxyUrl(t *testing.T) {
	tests := []struct {
		name     string
		settings Settings
		want     string
		wantErr  error
	}{
		{
			name:     "should add username and password to the proxy url",
			settings: Settings{PerDatasourceProxySettings: &PerDatasourceProxySettings{ProxyType: ProxyTypeUrl, ProxyUrl: "https://foo.com:3001/proxy", ProxyUsername: "usr", ProxyPassword: "pass"}},
			want:     "https://usr:pass@foo.com:3001/proxy",
		},
		{
			name:     "should override username and password to the proxy url",
			settings: Settings{PerDatasourceProxySettings: &PerDatasourceProxySettings{ProxyType: ProxyTypeUrl, ProxyUrl: "https://usr:pass@foo.com:3001/proxy", ProxyUsername: "new_usr", ProxyPassword: "new_pass"}},
			want:     "https://new_usr:new_pass@foo.com:3001/proxy",
		},
		{
			name:     "shouldn't set username and password if not present but present in url",
			settings: Settings{PerDatasourceProxySettings: &PerDatasourceProxySettings{ProxyType: ProxyTypeUrl, ProxyUrl: "https://usr:pass@foo.com:3001/proxy", ProxyUsername: "", ProxyPassword: ""}},
			want:     "https://usr:pass@foo.com:3001/proxy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProxyUrl(*tt.settings.PerDatasourceProxySettings)
			if tt.wantErr != nil {
				require.NotNil(t, err)
				assert.EqualError(t, err, tt.wantErr.Error())
				return
			}
			require.Nil(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestSettingsHashIncludesEndpoints(t *testing.T) {
	base := Settings{AuthType: AuthTypeKeys, Region: "us-west-2", AssumeRoleARN: "arn:aws:iam::1234567890:role/r"}
	withSTS := base
	withSTS.STSEndpoint = "https://sts.us-west-2.amazonaws.com"
	withService := base
	withService.Endpoint = "https://sts.us-west-2.amazonaws.com"

	assert.NotEqual(t, base.Hash(), withSTS.Hash())
	assert.NotEqual(t, withSTS.Hash(), withService.Hash())
}
