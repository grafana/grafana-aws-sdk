package awsds

// ResolveGrafanaAssumeRoleExternalID returns the external ID for Grafana
// Assume Role. When usePerDatasourceExternalID is explicitly true and
// grafanaExternalID is set, that value is used; otherwise the stack-level ID
// is returned (legacy shared isolation), even if a dormant grafanaExternalID
// is stored.
func ResolveGrafanaAssumeRoleExternalID(usePerDatasourceExternalID *bool, grafanaExternalID, stackExternalID string) string {
	if usePerDatasourceExternalID != nil && *usePerDatasourceExternalID && grafanaExternalID != "" {
		return grafanaExternalID
	}
	return stackExternalID
}
