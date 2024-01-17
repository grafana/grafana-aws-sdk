package models

import (
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v4"
)

type Settings interface {
	Load(backend.DataSourceInstanceSettings) error
	Apply(args sqlds.Options)
}

type Loader func() Settings

const DefaultKey = "__default"
