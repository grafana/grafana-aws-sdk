package awsauth

import (
	"context"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

const (
	FlagMultiTenantTempCredentials = "multiTenantTempCredentials"
	FlagCloudwatchRemoteDatasource = "cloudwatchRemoteDatasource"
)

func IsEnabled(ctx context.Context, feature string) bool {
	return backend.GrafanaConfigFromContext(ctx).FeatureToggles().IsEnabled(feature)
}
