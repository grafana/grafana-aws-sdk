package datasource

import (
	"context"
	"fmt"

	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
)

func startQuery(ctx context.Context, db api.AsyncDB, query *api.AsyncQuery) (string, error) {
	if db == nil {
		return "", fmt.Errorf("async handler not defined")
	}

	found, queryID, err := db.GetQueryID(ctx, query.RawSQL)
	if found || err != nil {
		return queryID, err
	}

	return db.StartQuery(ctx, query.RawSQL)
}

func queryStatus(ctx context.Context, db api.AsyncDB, query *api.AsyncQuery) (api.QueryStatus, error) {
	if db == nil {
		return api.QueryUnknown, fmt.Errorf("async handler not defined")
	}
	return db.QueryStatus(ctx, query.QueryID)
}
