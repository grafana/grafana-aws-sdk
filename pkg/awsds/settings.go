package awsds

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const defaultRegion = "default"

type AuthType int

const (
	AuthTypeDefault AuthType = iota
	AuthTypeSharedCreds
	AuthTypeKeys
)

func (at AuthType) String() string {
	switch at {
	case AuthTypeDefault:
		return "default"
	case AuthTypeSharedCreds:
		return "sharedCreds"
	case AuthTypeKeys:
		return "keys"
	default:
		panic(fmt.Sprintf("Unrecognized auth type %d", at))
	}
}

// DatasourceSettings holds basic connection info
type DatasourceSettings struct {
	Profile       string   `json:"profile"`
	Region        string   `json:"region"`
	DefaultRegion string   `json:"defaultRegion"` // NOT in cloudwatch?
	AuthType      AuthType `json:"authType"`
	AssumeRoleARN string   `json:"assumeRoleARN"`
	ExternalID    string   `json:"externalId"`

	// Loaded from DecryptedSecureJSONData (not the json object)
	AccessKey string `json:"-"`
	SecretKey string `json:"-"`
}

// LoadSettings will read and validate Settings from the DataSourceConfg
func LoadSettings(config backend.DataSourceInstanceSettings) (DatasourceSettings, error) {
	settings := DatasourceSettings{}

	if config.JSONData != nil && len(config.JSONData) > 1 {
		if err := json.Unmarshal(config.JSONData, &settings); err != nil {
			return settings, fmt.Errorf("could not unmarshal DatasourceSettings json: %w", err)
		}
	}

	if settings.Region == defaultRegion || settings.Region == "" {
		settings.Region = settings.DefaultRegion
	}

	if settings.Profile == "" {
		settings.Profile = config.Database // legacy support (only for cloudwatch?)
	}

	// at := authTypeDefault
	// switch atStr {
	// case "credentials":
	// 	at = authTypeSharedCreds
	// case "keys":
	// 	at = authTypeKeys
	// case "default":
	// 	at = authTypeDefault
	// case "arn":
	// 	at = authTypeDefault
	// 	plog.Warn("Authentication type \"arn\" is deprecated, falling back to default")
	// default:
	// 	plog.Warn("Unrecognized AWS authentication type", "type", atStr)
	// }

	settings.AccessKey = config.DecryptedSecureJSONData["accessKey"]
	settings.SecretKey = config.DecryptedSecureJSONData["secretKey"]

	return settings, nil
}
