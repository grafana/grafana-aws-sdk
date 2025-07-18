package api

import (
	"context"
	"errors"
	"time"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/sqlds/v4"
	"github.com/jpillora/backoff"
)

var (
	ErrorExecute = errors.New("error executing query")
	ErrorStatus  = errors.New("error getting query query status")
	ErrorStop    = errors.New("error stopping query")

	backoffMin = 200 * time.Millisecond
	backoffMax = 10 * time.Minute
)

type ExecuteQueryInput struct {
	ID    string
	Query string
}

type ExecuteQueryOutput struct {
	ID string
}

type ExecuteQueryStatus struct {
	ID       string
	Finished bool
	State    string
}

type SQL interface {
	Execute(context.Context, *ExecuteQueryInput) (*ExecuteQueryOutput, error)
	Status(context.Context, *ExecuteQueryOutput) (*ExecuteQueryStatus, error)
	Stop(*ExecuteQueryOutput) error
}

type Resources interface {
	Regions(context.Context) ([]string, error)
	Databases(context.Context, sqlds.Options) ([]string, error)
	CancelQuery(context.Context, sqlds.Options, string) error
}

type AWSAPI interface {
	SQL
	Resources
}

// WaitOnQuery polls the datasource api until the query finishes, returning an error if it failed.
func WaitOnQuery(ctx context.Context, api SQL, output *ExecuteQueryOutput) error {
	backoffInstance := backoff.Backoff{
		Min:    backoffMin,
		Max:    backoffMax,
		Factor: 1.1,
	}
	for {
		status, err := api.Status(ctx, output)
		if err != nil {
			return err
		}
		if status.Finished {
			return nil
		}
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.Canceled) {
				err := api.Stop(output)
				if err != nil {
					return err
				}
			}
			log.DefaultLogger.Debug("request failed", "query ID", output.ID, "error", err)
			return err
		case <-time.After(backoffInstance.Duration()):
			continue
		}
	}
}

func WaitOnQueryID(ctx context.Context, queryID string, db awsds.AsyncDB) error {
	backoffInstance := backoff.Backoff{
		Min:    backoffMin,
		Max:    backoffMax,
		Factor: 2,
	}
	for {
		status, err := db.QueryStatus(ctx, queryID)
		if err != nil {
			return err
		}
		if status.Finished() {
			return nil
		}
		select {
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.Canceled) {
				err := db.CancelQuery(context.Background(), queryID)
				if err != nil {
					return err
				}
			}
			log.DefaultLogger.Debug("request failed", "query ID", queryID, "error", err)
			return err
		case <-time.After(backoffInstance.Duration()):
			continue
		}
	}
}
