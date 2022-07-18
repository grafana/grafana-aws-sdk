package async

import (
	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	sqlDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver"
	"github.com/grafana/sqlds/v2"
)

type Driver interface {
	sqlDriver.Driver
	GetAsyncDB() (sqlds.AsyncDB, error)
}

type Loader func(api.AWSAPI) (Driver, error)
