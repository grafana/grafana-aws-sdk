package awsds

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/backend"
)

const defaultRegion = "default"

type AuthType int

const (
	AuthTypeDefault AuthType = iota
	AuthTypeSharedCreds
	AuthTypeKeys
	AuthTypeEC2IAMRole
	AuthTypeGrafanaAssumeRole //cloud only
)

func (at *AuthType) String() string {
	switch *at {
	case AuthTypeDefault:
		return "default"
	case AuthTypeSharedCreds:
		return "credentials"
	case AuthTypeKeys:
		return "keys"
	case AuthTypeEC2IAMRole:
		return "ec2_iam_role"
	case AuthTypeGrafanaAssumeRole:
		return "grafana_assume_role"
	default:
		panic(fmt.Sprintf("Unrecognized auth type %d", at))
	}
}

func ToAuthType(authType string) (AuthType, error) {
	switch authType {
	case "credentials", "sharedCreds":
		return AuthTypeSharedCreds, nil
	case "keys":
		return AuthTypeKeys, nil
	case "default":
		return AuthTypeDefault, nil
	case "ec2_iam_role":
		return AuthTypeEC2IAMRole, nil
	case "arn":
		return AuthTypeDefault, nil
	case "grafana_assume_role":
		return AuthTypeGrafanaAssumeRole, nil
	default:
		return AuthTypeDefault, fmt.Errorf("invalid auth type: %s", authType)
	}
}

// MarshalJSON marshals the enum as a quoted json string
func (at *AuthType) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(at.String())
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

// UnmarshalJSON unmashals a quoted json string to the enum value
func (at *AuthType) UnmarshalJSON(b []byte) error {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return err
	}
	switch j {
	case "credentials": // Old name
		fallthrough
	case "sharedCreds":
		*at = AuthTypeSharedCreds
	case "keys":
		*at = AuthTypeKeys
	case "ec2_iam_role":
		*at = AuthTypeEC2IAMRole
	case "grafana_assume_role":
		*at = AuthTypeGrafanaAssumeRole
	case "default":
		fallthrough
	default:
		*at = AuthTypeDefault // For old 'arn' option
	}
	return nil
}

// DatasourceSettings holds basic connection info
type AWSDatasourceSettings struct {
	Profile       string   `json:"profile"`
	Region        string   `json:"region"`
	AuthType      AuthType `json:"authType"`
	AssumeRoleARN string   `json:"assumeRoleARN"`
	ExternalID    string   `json:"externalId"`

	// Override the client endpoint
	Endpoint string `json:"endpoint"`

	//go:deprecated Use Region instead
	DefaultRegion string `json:"defaultRegion"`

	// Loaded from DecryptedSecureJSONData (not the json object)
	AccessKey    string `json:"-"`
	SecretKey    string `json:"-"`
	SessionToken string `json:"-"`
}

// DataSourceInstanceSettings represents settings for a data source instance.
//
// In Grafana a data source instance is a data source plugin of certain
// type that have been configured and created in a Grafana organization.
//
// Copied from grafana-plugin-sdk-go
type DataSourceInstanceSettings struct {
	// Deprecated ID is the Grafana assigned numeric identifier of the the data source instance.
	ID int64

	// UID is the Grafana assigned string identifier of the the data source instance.
	UID string

	// Type is the unique identifier of the plugin that the request is for.
	// This should be the same value as PluginContext.PluginId.
	Type string

	// Name is the configured name of the data source instance.
	Name string

	// URL is the configured URL of a data source instance (e.g. the URL of an API endpoint).
	URL string

	// User is a configured user for a data source instance. This is not a Grafana user, rather an arbitrary string.
	User string

	// Database is the configured database for a data source instance.
	// Only used by Elasticsearch and Influxdb.
	// Please use JSONData to store information related to database.
	Database string

	// BasicAuthEnabled indicates if this data source instance should use basic authentication.
	BasicAuthEnabled bool

	// BasicAuthUser is the configured user for basic authentication. (e.g. when a data source uses basic
	// authentication to connect to whatever API it fetches data from).
	BasicAuthUser string

	// JSONData contains the raw DataSourceConfig as JSON as stored by Grafana server. It repeats the properties in
	// this object and includes custom properties.
	JSONData json.RawMessage

	// DecryptedSecureJSONData contains key,value pairs where the encrypted configuration in Grafana server have been
	// decrypted before passing them to the plugin.
	DecryptedSecureJSONData map[string]string

	// Updated is the last time the configuration for the data source instance was updated.
	Updated time.Time

	// The API Version when settings were saved
	// NOTE: this may be older than the current version
	APIVersion string
}

// LoadSettings will read and validate Settings from the DataSourceConfg
func (s *AWSDatasourceSettings) Load(config backend.DataSourceInstanceSettings) error {
	if len(config.JSONData) > 1 {
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
