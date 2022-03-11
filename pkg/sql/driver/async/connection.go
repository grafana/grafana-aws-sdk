package async

import (
	"context"
	"database/sql/driver"
	"fmt"

	sqlAPI "github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	"github.com/grafana/sqlds/v2"
)

// Implements "*sql.DB"
type Conn struct {
	db sqlds.AsyncDB
}

func NewConnection(db sqlds.AsyncDB) *Conn {
	return &Conn{db: db}
}

func (c *Conn) CheckNamedValue(v *driver.NamedValue) error {
	if v.Name != "queryID" {
		return fmt.Errorf("only queryID parameters are supported")
	}
	return nil
}

func (c *Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	// Asynchronous flow
	queryID := ""
	for _, arg := range args {
		if arg.Name == "queryID" {
			queryID = arg.Value.(string)
		}
	}
	if queryID != "" {
		return c.db.GetRows(ctx, queryID)
	}
	// Synchronous flow
	queryID, err := c.db.StartQuery(ctx, query, args)
	if err != nil {
		return nil, err
	}

	if err := sqlAPI.WaitOnQueryID(ctx, queryID, c.db); err != nil {
		return nil, err
	}

	return c.db.GetRows(ctx, queryID)
}

func (c *Conn) Ping(ctx context.Context) error {
	return c.db.Ping(ctx)
}

func (c *Conn) Begin() (driver.Tx, error) {
	return c.db.Begin()
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	return c.db.Prepare(query)
}

func (c *Conn) Close() error {
	return c.db.Close()
}
