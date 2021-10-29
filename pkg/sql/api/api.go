package api

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/sqlds/v2"
	"github.com/jpillora/backoff"
)

var (
	ExecuteError = errors.New("error executing query")
	StatusError  = errors.New("error getting query query status")
	StopError    = errors.New("error stopping query")
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
	Execute(aws.Context, *ExecuteQueryInput) (*ExecuteQueryOutput, error)
	Status(aws.Context, *ExecuteQueryOutput) (*ExecuteQueryStatus, error)
	Stop(*ExecuteQueryOutput) error
}

type Resources interface {
	Regions(aws.Context) ([]string, error)
	Databases(aws.Context, sqlds.Options) ([]string, error)
}

var (
	backoffMin = 200 * time.Millisecond
	backoffMax = 10 * time.Minute
)

// WaitOnQuery polls the datasource api until the query finishes, returning an error if it failed.
func WaitOnQuery(ctx context.Context, api SQL, output *ExecuteQueryOutput) error {
	backoffInstance := backoff.Backoff{
		Min:    backoffMin,
		Max:    backoffMax,
		Factor: 2,
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
