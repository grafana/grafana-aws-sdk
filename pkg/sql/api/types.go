package api

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v2"
)

// QueryStatus represents the status of an async query
type QueryStatus uint32

const (
	QueryUnknown QueryStatus = iota
	QuerySubmitted
	QueryRunning
	QueryFinished
	QueryCanceled
	QueryFailed
)

func (qs QueryStatus) Finished() bool {
	return qs == QueryCanceled || qs == QueryFailed || qs == QueryFinished
}

func (qs QueryStatus) String() string {
	switch qs {
	case QuerySubmitted:
		return "submitted"
	case QueryRunning:
		return "running"
	case QueryFinished:
		return "finished"
	case QueryCanceled:
		return "canceled"
	case QueryFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type QueryMeta struct {
	QueryFlow string `json:"queryFlow,omitempty"`
}

type AsyncQuery struct {
	sqlds.Query
	QueryID string    `json:"queryID,omitempty"`
	Meta    QueryMeta `json:"meta,omitempty"`
}

// GetQuery returns a Query object given a backend.DataQuery using json.Unmarshal
func GetQuery(query backend.DataQuery) (*AsyncQuery, error) {
	model := &AsyncQuery{}

	if err := json.Unmarshal(query.JSON, &model); err != nil {
		return nil, fmt.Errorf("%w: %v", sqlds.ErrorJSON, err)
	}

	// Copy directly from the well typed query
	return &AsyncQuery{
		Query:   model.Query,
		QueryID: model.QueryID,
		Meta:    model.Meta,
	}, nil
}

// AsyncDB represents an async SQL connection
type AsyncDB interface {
	// DB generic methods
	driver.Conn
	Ping(ctx context.Context) error

	// Async flow
	StartQuery(ctx context.Context, query string, args ...interface{}) (string, error)
	GetQueryID(ctx context.Context, query string, args ...interface{}) (bool, string, error)
	QueryStatus(ctx context.Context, queryID string) (QueryStatus, error)
	CancelQuery(ctx context.Context, queryID string) error
	GetRows(ctx context.Context, queryID string) (driver.Rows, error)
}

// AsyncDriver extends the driver interface to also connect to async SQL datasources
type AsyncDriver interface {
	sqlds.Driver
	GetAsyncDB(backend.DataSourceInstanceSettings) (AsyncDB, error)
}
