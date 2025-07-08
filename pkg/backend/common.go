package backend

import (
	"encoding/json"
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/backend/useragent"
	"time"
)

// User represents a Grafana user.
type User struct {
	Login string
	Name  string
	Email string
	Role  string
}

// DataSourceInstanceSettings represents settings for an app instance.
//
// In Grafana an app instance is an app plugin of certain
// type that have been configured and enabled in a Grafana organization.
// DataSourceInstanceSettings represents settings for a data source instance.
//
// In Grafana a data source instance is a data source plugin of certain
// type that have been configured and created in a Grafana organization.
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

// PluginContext holds contextual information about a plugin request, such as
// Grafana organization, user and plugin instance settings.
type PluginContext struct {
	// OrgID is The Grafana organization identifier the request originating from.
	OrgID int64

	// PluginID is the unique identifier of the plugin that the request is for.
	PluginID string

	// PluginVersion is the version of the plugin that the request is for.
	PluginVersion string

	// User is the Grafana user making the request.
	//
	// Will not be provided if Grafana backend initiated the request,
	// for example when request is coming from Grafana Alerting.
	User *User

	// AppInstanceSettings is the configured app instance settings.
	//
	// In Grafana an app instance is an app plugin of certain
	// type that have been configured and enabled in a Grafana organization.
	//
	// Will only be set if request targeting an app instance.
	AppInstanceSettings *AppInstanceSettings

	// DataSourceConfig is the configured data source instance
	// settings.
	//
	// In Grafana a data source instance is a data source plugin of certain
	// type that have been configured and created in a Grafana organization.
	//
	// Will only be set if request targeting a data source instance.
	DataSourceInstanceSettings *DataSourceInstanceSettings

	// GrafanaConfig is the configuration settings provided by Grafana.
	GrafanaConfig *GrafanaCfg

	// UserAgent is the user agent of the Grafana server that initiated the gRPC request.
	// Will only be set if request is made from Grafana v10.2.0 or later.
	UserAgent *useragent.UserAgent

	// The requested API version
	APIVersion string
}
