package awsds

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v2"
)

type AsyncAWSDatasource struct {
	*sqlds.SQLDatasource
	asyncDB              AsyncDB
	connSettings         backend.DataSourceInstanceSettings
	driver               AsyncDriver
	sqldsQueryDataHander backend.QueryDataHandlerFunc
}

func NewAsyncAWSDatasource(driver AsyncDriver) *AsyncAWSDatasource {
	sqlDs := sqlds.NewDatasource(driver)
	return &AsyncAWSDatasource{
		SQLDatasource:        sqlDs,
		driver:               driver,
		sqldsQueryDataHander: sqlDs.QueryData,
	}
}

func getQueryFlow(query backend.DataQuery) string {
	q, _ := GetQuery(query)
	if q.Meta.QueryFlow == "async" {
		return "async"
	}
	return "sync"
}

func (ds *AsyncAWSDatasource) NewDatasource(settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	var err error
	ds.connSettings = settings
	ds.asyncDB, err = ds.driver.GetAsyncDB(settings, nil)
	if err != nil {
		return nil, err
	}
	_, err = ds.SQLDatasource.NewDatasource(settings)
	return ds, err
}

func (ds *AsyncAWSDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	syncExectionEnabled := false
	for _, query := range req.Queries {
		if getQueryFlow(query) == "sync" {
			syncExectionEnabled = true
			break
		}
	}

	_, isFromAlert := req.Headers["FromAlert"]
	if syncExectionEnabled || isFromAlert {
		return ds.sqldsQueryDataHander.QueryData(ctx, req)
	}

	// async flow
	var (
		response = sqlds.NewResponse(backend.NewQueryDataResponse())
		wg       = sync.WaitGroup{}
	)

	// Execute each query and store the results by query RefID
	for _, q := range req.Queries {
		wg.Add(1)
		go func(query backend.DataQuery) {
			var frames data.Frames
			var err error
			frames, err = ds.handleAsyncQuery(ctx, query, getDatasourceUID(*req.PluginContext.DataSourceInstanceSettings))
			response.Set(query.RefID, backend.DataResponse{
				Frames: frames,
				Error:  err,
			})

			wg.Done()
		}(q)
	}

	wg.Wait()
	return response.Response(), nil
}

type queryMeta struct {
	QueryID string `json:"queryID"`
	Status  string `json:"status"`
}

// handleQuery will call query, and attempt to reconnect if the query failed
func (ds *AsyncAWSDatasource) handleAsyncQuery(ctx context.Context, req backend.DataQuery, datasourceUID string) (data.Frames, error) {
	// Convert the backend.DataQuery into a Query object
	q, err := GetQuery(req)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}

	// Apply supported macros to the query
	q.RawSQL, err = sqlds.Interpolate(ds.driver, &q.Query)
	if err != nil {
		return getErrorFrameFromQuery(q), fmt.Errorf("%s: %w", "Could not apply macros", err)
	}

	// Apply the default FillMode, overwritting it if the query specifies it
	driverSettings := ds.SQLDatasource.DriverSettings()
	fillMode := driverSettings.FillMode
	if q.FillMissing != nil {
		fillMode = q.FillMissing
	}

	if driverSettings.Timeout != 0 {
		tctx, cancel := context.WithTimeout(ctx, driverSettings.Timeout)
		defer cancel()
		ctx = tctx
	}

	if q.QueryID == "" {
		queryID, err := startQuery(ctx, ds.asyncDB, q)
		if err != nil {
			return getErrorFrameFromQuery(q), err
		}
		return data.Frames{
			{Meta: &data.FrameMeta{
				ExecutedQueryString: q.RawSQL,
				Custom:              queryMeta{QueryID: queryID, Status: "started"}},
			},
		}, nil
	}

	status, err := queryStatus(ctx, ds.asyncDB, q)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}
	customMeta := queryMeta{QueryID: q.QueryID, Status: status.String()}
	if !status.Finished() {
		return data.Frames{
			{Meta: &data.FrameMeta{
				ExecutedQueryString: q.RawSQL,
				Custom:              customMeta},
			},
		}, nil
	}

	conn, err := ds.driver.Connect(ds.connSettings, q.ConnectionArgs)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}
	res, err := queryAsync(ctx, conn, ds.driver.Converters(), fillMode, q)
	if err == nil || errors.Is(err, sqlds.ErrorNoResults) {
		if len(res) == 0 {
			res = append(res, &data.Frame{})
		}
		res[0].Meta.Custom = customMeta
		return res, nil
	}

	if !errors.Is(err, sqlds.ErrorQuery) {
		return nil, err
	}

	conn, err = ds.driver.Connect(ds.connSettings, q.ConnectionArgs)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}
	res, err = queryAsync(ctx, conn, ds.driver.Converters(), fillMode, q)
	if err == nil || errors.Is(err, sqlds.ErrorNoResults) {
		if len(res) == 0 {
			res = append(res, &data.Frame{})
		}
		res[0].Meta.Custom = customMeta
	}
	return res, err
}

func queryAsync(ctx context.Context, conn *sql.DB, converters []sqlutil.Converter, fillMode *data.FillMissing, q *AsyncQuery) (data.Frames, error) {
	return sqlds.QueryDB(ctx, conn, converters, fillMode, &q.Query, sql.NamedArg{Name: "queryID", Value: q.QueryID})
}