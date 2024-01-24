package datasource

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/grafana/sqlds/v3"
)

func connectionKey(id int64, args sqlds.Options) string {
	return fmt.Sprintf("%d-%v", id, args)
}

func GetDatasourceID(ctx context.Context) int64 {
	plugin := httpadapter.PluginConfigFromContext(ctx)
	if plugin.DataSourceInstanceSettings != nil {
		return plugin.DataSourceInstanceSettings.ID
	}
	return 0
}

func GetDatasourceLastUpdatedTime(ctx context.Context) string {
	plugin := httpadapter.PluginConfigFromContext(ctx)
	if plugin.DataSourceInstanceSettings != nil {
		return plugin.DataSourceInstanceSettings.Updated.String()
	}
	return ""
}
