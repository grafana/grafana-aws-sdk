package datasource

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/grafana/grafana-aws-sdk/pkg/sql/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend/resource/httpadapter"
	"github.com/grafana/sqlds/v2"
)

func connectionKey(id int64, args sqlds.Options, settings models.Settings) string {
	settingsStr := fmt.Sprintf("%v", settings)
	hashedSettings := sha256.Sum256([]byte(settingsStr))
	return fmt.Sprintf("%d-%v-%x", id, args, hashedSettings)
}

func GetDatasourceID(ctx context.Context) int64 {
	plugin := httpadapter.PluginConfigFromContext(ctx)
	if plugin.DataSourceInstanceSettings != nil {
		return plugin.DataSourceInstanceSettings.ID
	}
	return 0
}
