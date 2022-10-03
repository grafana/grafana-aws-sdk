package async

import (
	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	sqlDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver"
)

type Driver interface {
	sqlDriver.Driver
	GetAsyncDB() (awsds.AsyncDB, error)
}

type Loader func(api.AWSAPI) (Driver, error)
