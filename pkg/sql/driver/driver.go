package driver

import (
	"database/sql"
	"database/sql/driver"

	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
)

type Driver interface {
	Open(_ string) (driver.Conn, error)
	Closed() bool
	OpenDB() (*sql.DB, error)
}

type Loader func(api.AWSAPI) (Driver, error)
