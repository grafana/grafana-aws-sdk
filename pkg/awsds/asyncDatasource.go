package awsds

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"
	"github.com/grafana/sqlds/v3"
)

const fromAlertHeader = "FromAlert"
const fromExpressionHeader = "http_X-Grafana-From-Expr"

type dbConnection struct {
	db       AsyncDB
	settings backend.DataSourceInstanceSettings
}

type AsyncAWSDatasource struct {
	*sqlds.SQLDatasource

	dbConnections         sync.Map
	driver                AsyncDriver
	sqldsQueryDataHandler backend.QueryDataHandlerFunc
}

func (ds *AsyncAWSDatasource) getDBConnection(settings backend.DataSourceInstanceSettings, connectionArgs json.RawMessage) (dbConnection, bool) {
	key := sqlds.GetConnectionKey(settings, connectionArgs)
	conn, ok := ds.dbConnections.Load(key)
	if !ok {
		return dbConnection{}, false
	}
	return conn.(dbConnection), true
}

func (ds *AsyncAWSDatasource) storeDBConnection(dbConn dbConnection, connectionArgs json.RawMessage) {
	key := sqlds.GetConnectionKey(dbConn.settings, connectionArgs)
	ds.dbConnections.Store(key, dbConn)
}

func NewAsyncAWSDatasource(driver AsyncDriver) *AsyncAWSDatasource {
	sqlDs := sqlds.NewDatasource(driver)
	return &AsyncAWSDatasource{
		SQLDatasource:         sqlDs,
		driver:                driver,
		sqldsQueryDataHandler: sqlDs.QueryData,
	}
}

// isAsyncFlow checks the feature flag in query to see if it is async
func isAsyncFlow(query backend.DataQuery) bool {
	q, _ := GetQuery(query)
	return q.Meta.QueryFlow == "async"
}

func (ds *AsyncAWSDatasource) NewDatasource(ctx context.Context, settings backend.DataSourceInstanceSettings) (instancemgmt.Instance, error) {
	db, err := ds.driver.GetAsyncDB(settings, nil)
	if err != nil {
		return nil, err
	}
	ds.storeDBConnection(dbConnection{db, settings}, nil)

	// initialize the wrapped ds.SQLDatasource
	_, err = ds.SQLDatasource.NewDatasource(ctx, settings)
	return ds, err
}

func (ds *AsyncAWSDatasource) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	syncExectionEnabled := false
	for _, query := range req.Queries {
		if !isAsyncFlow(query) {
			syncExectionEnabled = true
			break
		}
	}

	_, isFromAlert := req.Headers[fromAlertHeader]
	_, isFromExpression := req.Headers[fromExpressionHeader]
	if syncExectionEnabled || isFromAlert || isFromExpression {
		return ds.sqldsQueryDataHandler.QueryData(ctx, req)
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
			frames, err = ds.handleAsyncQuery(ctx, query, *req.PluginContext.DataSourceInstanceSettings)
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

func (ds *AsyncAWSDatasource) CheckHealth(ctx context.Context, req *backend.CheckHealthRequest) (*backend.CheckHealthResult, error) {
	settings := *req.PluginContext.DataSourceInstanceSettings
	dbConn, ok := ds.getDBConnection(settings, nil)
	if !ok {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: "No database connection found for datasource uid: " + settings.UID,
		}, nil
	}
	err := dbConn.db.Ping(ctx)
	if err != nil {
		return &backend.CheckHealthResult{
			Status:  backend.HealthStatusError,
			Message: err.Error(),
		}, nil
	}
	return &backend.CheckHealthResult{
		Status:  backend.HealthStatusOk,
		Message: "Data source is working",
	}, nil
}

func (ds *AsyncAWSDatasource) getAsyncDBFromQuery(q *AsyncQuery, settings backend.DataSourceInstanceSettings) (AsyncDB, error) {
	if !ds.EnableMultipleConnections && len(q.ConnectionArgs) > 0 {
		return nil, sqlds.ErrorMissingMultipleConnectionsConfig
	}
	// The database connection may vary depending on query arguments
	// The raw arguments are used as key to store the db connection in memory so they can be reused
	if !ds.EnableMultipleConnections || len(q.ConnectionArgs) == 0 {
		dbConn, ok := ds.getDBConnection(settings, nil)
		if !ok {
			return nil, sqlds.ErrorMissingDBConnection
		}
		return dbConn.db, nil
	}

	if cachedConn, ok := ds.getDBConnection(settings, q.ConnectionArgs); ok {
		return cachedConn.db, nil
	}

	var err error
	db, err := ds.driver.GetAsyncDB(settings, q.ConnectionArgs)
	if err != nil {
		return nil, err
	}
	// Assign this connection in the cache
	dbConn := dbConnection{db, settings}
	ds.storeDBConnection(dbConn, q.ConnectionArgs)

	return dbConn.db, nil
}

type queryMeta struct {
	QueryID string `json:"queryID"`
	Status  string `json:"status"`
}

// handleQuery will call query, and attempt to reconnect if the query failed
func (ds *AsyncAWSDatasource) handleAsyncQuery(ctx context.Context, req backend.DataQuery, settings backend.DataSourceInstanceSettings) (data.Frames, error) {
	// Convert the backend.DataQuery into a Query object
	q, err := GetQuery(req)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}

	// Apply supported macros to the query
	q.RawSQL, err = sqlutil.Interpolate(&q.Query, ds.driver.Macros())
	if err != nil {
		return getErrorFrameFromQuery(q), fmt.Errorf("%s: %w", "Could not apply macros", err)
	}

	// Apply the default FillMode, overwritting it if the query specifies it
	driverSettings := ds.SQLDatasource.DriverSettings()
	fillMode := driverSettings.FillMode
	if q.FillMissing != nil {
		fillMode = q.FillMissing
	}

	asyncDB, err := ds.getAsyncDBFromQuery(q, settings)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}

	if q.QueryID == "" {
		queryID, err := startQuery(ctx, asyncDB, q)
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

	status, err := queryStatus(ctx, asyncDB, q)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}
	customMeta := queryMeta{QueryID: q.QueryID, Status: status.String()}
	if status != QueryFinished {
		return data.Frames{
			{Meta: &data.FrameMeta{
				ExecutedQueryString: q.RawSQL,
				Custom:              customMeta},
			},
		}, nil
	}

	db, err := ds.GetDBFromQuery(ctx, &q.Query, settings)
	if err != nil {
		return getErrorFrameFromQuery(q), err
	}
	res, err := queryAsync(ctx, db, ds.driver.Converters(), fillMode, q)
	if err == nil || errors.Is(err, sqlds.ErrorNoResults) {
		if len(res) == 0 {
			res = append(res, &data.Frame{})
		}
		res[0].Meta.Custom = customMeta
		return res, nil
	}

	return getErrorFrameFromQuery(q), err
}

func queryAsync(ctx context.Context, conn *sql.DB, converters []sqlutil.Converter, fillMode *data.FillMissing, q *AsyncQuery) (data.Frames, error) {
	return sqlds.QueryDB(ctx, conn, converters, fillMode, &q.Query, sql.NamedArg{Name: "queryID", Value: q.QueryID})
}
