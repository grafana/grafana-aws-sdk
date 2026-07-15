package awsds

// PreferGrafanaExternalID reports whether Grafana Assume Role should use the
// per-datasource external ID. Only an explicit usePerDatasourceExternalId=true
// enables that mode. When the bool is unset or false, use the stack external ID
// (legacy shared isolation), even if a dormant grafanaExternalId is stored.
func PreferGrafanaExternalID(usePerDatasourceExternalID *bool, _ string) bool {
	return usePerDatasourceExternalID != nil && *usePerDatasourceExternalID
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
