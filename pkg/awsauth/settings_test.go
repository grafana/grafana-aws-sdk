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
			settings: Settings{ProxyType: ProxyTypeUrl, ProxyUrl: "https://foo.com:3001/proxy", ProxyUsername: "usr", ProxyPassword: "pass"},
			want:     "https://usr:pass@foo.com:3001/proxy",
		},
		{
			name:     "should override username and password to the proxy url",
			settings: Settings{ProxyType: ProxyTypeUrl, ProxyUrl: "https://usr:pass@foo.com:3001/proxy", ProxyUsername: "new_usr", ProxyPassword: "new_pass"},
			want:     "https://new_usr:new_pass@foo.com:3001/proxy",
		},
		{
			name:     "shouldn't set username and password if not present but present in url",
			settings: Settings{ProxyType: ProxyTypeUrl, ProxyUrl: "https://usr:pass@foo.com:3001/proxy", ProxyUsername: "", ProxyPassword: ""},
			want:     "https://usr:pass@foo.com:3001/proxy",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetProxyUrl(tt.settings)
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
