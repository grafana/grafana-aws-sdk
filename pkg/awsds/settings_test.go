package awsds

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
)

// Test load settings from json
func TestLoadSettings(t *testing.T) {
	settings := &AWSDatasourceSettings{
		AuthType:      AuthTypeKeys,
		DefaultRegion: "aaaa",
	}

	bytes, _ := json.Marshal(settings)
	copy := &AWSDatasourceSettings{}
	config := backend.DataSourceInstanceSettings{
		DecryptedSecureJSONData: map[string]string{},
		JSONData:                bytes,
	}
	err := copy.Load(config)
	if err != nil {
		t.Fatalf("error reading config: %v", err)
	}

	assert.Empty(t, cmp.Diff(settings.AuthType, copy.AuthType))
	assert.Empty(t, cmp.Diff(settings.DefaultRegion, copy.DefaultRegion))
}

func TestLoadSettingsEndpoints(t *testing.T) {
	s := &AWSDatasourceSettings{}
	err := s.Load(backend.DataSourceInstanceSettings{
		JSONData: []byte(`{"endpoint":"https://athena.eu-west-2.amazonaws.com","stsEndpoint":"https://sts.eu-west-2.amazonaws.com"}`),
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://athena.eu-west-2.amazonaws.com", s.Endpoint)
	assert.Equal(t, "https://sts.eu-west-2.amazonaws.com", s.STSEndpoint)
}
