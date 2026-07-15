package awsds

// PreferGrafanaExternalID reports whether Grafana Assume Role should use the
// per-datasource external ID. An explicit usePerDatasourceExternalId value
// wins; when unset, a stored grafanaExternalId means per-DS mode (legacy).
func PreferGrafanaExternalID(usePerDatasourceExternalID *bool, grafanaExternalID string) bool {
	if usePerDatasourceExternalID != nil {
		return *usePerDatasourceExternalID
	}
	return grafanaExternalID != ""
}

// ResolveGrafanaAssumeRoleExternalID returns the external ID for Grafana
// Assume Role: the per-datasource ID when preferred and set, otherwise the
// stack-level ID.
func ResolveGrafanaAssumeRoleExternalID(usePerDatasourceExternalID *bool, grafanaExternalID, stackExternalID string) string {
	if PreferGrafanaExternalID(usePerDatasourceExternalID, grafanaExternalID) && grafanaExternalID != "" {
		return grafanaExternalID
	}
	return stackExternalID
}
