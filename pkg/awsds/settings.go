package awsds

import (
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-aws-sdk/pkg/auth"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const defaultRegion = "default"

// DatasourceSettings holds basic connection info
type AWSDatasourceSettings struct {
	Profile       string        `json:"profile"`
	Region        string        `json:"region"`
	AuthType      auth.AuthType `json:"authType"`
	AssumeRoleARN string        `json:"assumeRoleARN"`
	ExternalID    string        `json:"externalId"`

	// Override the client endpoint
	Endpoint string `json:"endpoint"`

	//go:deprecated Use Region instead
	DefaultRegion string `json:"defaultRegion"`

	// Loaded from DecryptedSecureJSONData (not the json object)
	AccessKey    string `json:"-"`
	SecretKey    string `json:"-"`
	SessionToken string `json:"-"`
}

// LoadSettings will read and validate Settings from the DataSourceConfg
func (s *AWSDatasourceSettings) Load(config backend.DataSourceInstanceSettings) error {
	if config.JSONData != nil && len(config.JSONData) > 1 {
		if err := json.Unmarshal(config.JSONData, s); err != nil {
			return fmt.Errorf("could not unmarshal DatasourceSettings json: %w", err)
		}
	}

	if s.Region == defaultRegion || s.Region == "" {
		s.Region = s.DefaultRegion
	}

	if s.Profile == "" {
		s.Profile = config.Database // legacy support (only for cloudwatch?)
	}

	s.AccessKey = config.DecryptedSecureJSONData["accessKey"]
	s.SecretKey = config.DecryptedSecureJSONData["secretKey"]
	s.SessionToken = config.DecryptedSecureJSONData["sessionToken"]

	return nil
}
