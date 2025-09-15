package backend

import (
	"context"
	"github.com/grafana/grafana-aws-sdk-for-backport/pkg/backend/proxy"
)

const (
	AppURL                           = "GF_APP_URL"
	ConcurrentQueryCount             = "GF_CONCURRENT_QUERY_COUNT"
	UserFacingDefaultError           = "GF_USER_FACING_DEFAULT_ERROR"
	SQLRowLimit                      = "GF_SQL_ROW_LIMIT"
	SQLMaxOpenConnsDefault           = "GF_SQL_MAX_OPEN_CONNS_DEFAULT"
	SQLMaxIdleConnsDefault           = "GF_SQL_MAX_IDLE_CONNS_DEFAULT"
	SQLMaxConnLifetimeSecondsDefault = "GF_SQL_MAX_CONN_LIFETIME_SECONDS_DEFAULT"
	ResponseLimit                    = "GF_RESPONSE_LIMIT"
	AppClientSecret                  = "GF_PLUGIN_APP_CLIENT_SECRET" // nolint:gosec
)

type configKey struct{}

// GrafanaConfigFromContext returns Grafana config from context.
func GrafanaConfigFromContext(ctx context.Context) *GrafanaCfg {
	v := ctx.Value(configKey{})
	if v == nil {
		return NewGrafanaCfg(nil)
	}

	cfg := v.(*GrafanaCfg)
	if cfg == nil {
		return NewGrafanaCfg(nil)
	}

	return cfg
}

// WithGrafanaConfig injects supplied Grafana config into context.
func WithGrafanaConfig(ctx context.Context, cfg *GrafanaCfg) context.Context {
	ctx = context.WithValue(ctx, configKey{}, cfg)
	return ctx
}

type GrafanaCfg struct {
	config map[string]string
}

func NewGrafanaCfg(cfg map[string]string) *GrafanaCfg {
	return &GrafanaCfg{config: cfg}
}

func (c *GrafanaCfg) Get(key string) string {
	return c.config[key]
}

type Proxy struct {
	clientCfg *proxy.ClientCfg
}
