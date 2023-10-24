package awsds

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/grafana-aws-sdk/pkg/auth"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/stretchr/testify/assert"
)

// Test load settings from json
func TestLoadSettings(t *testing.T) {
	settings := &AWSDatasourceSettings{
		AuthType:      auth.AuthTypeKeys,
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
