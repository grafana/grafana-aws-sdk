package awsds

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type fakeAsyncDB struct{}

func (fakeAsyncDB) Begin() (driver.Tx, error)                                        { return nil, nil }
func (fakeAsyncDB) Prepare(query string) (driver.Stmt, error)                        { return nil, nil }
func (fakeAsyncDB) Close() error                                                     { return nil }
func (fakeAsyncDB) Ping(ctx context.Context) error                                   { return nil }
func (fakeAsyncDB) CancelQuery(ctx context.Context, queryID string) error            { return nil }
func (fakeAsyncDB) GetRows(ctx context.Context, queryID string) (driver.Rows, error) { return nil, nil }

func (fakeAsyncDB) GetQueryID(ctx context.Context, query string, args ...interface{}) (bool, string, error) {
	return false, "", nil
}

func (fakeAsyncDB) QueryStatus(ctx context.Context, queryID string) (QueryStatus, error) {
	return QueryUnknown, nil
}

func (fakeAsyncDB) StartQuery(ctx context.Context, query string, args ...interface{}) (string, error) {
	return "", nil
}

type fakeDriver struct {
	openDBfn func() (AsyncDB, error)
	AsyncDriver
}

func (d fakeDriver) GetAsyncDB(backend.DataSourceInstanceSettings, json.RawMessage) (db AsyncDB, err error) {
	return d.openDBfn()
}

func Test_getDBConnectionFromQuery(t *testing.T) {
	db := &fakeAsyncDB{}
	db2 := &fakeAsyncDB{}
	db3 := &fakeAsyncDB{}
	d := &fakeDriver{openDBfn: func() (AsyncDB, error) { return db3, nil }}
	tests := []struct {
		desc        string
		dsUID       string
		args        string
		existingDB  AsyncDB
		expectedKey string
		expectedDB  AsyncDB
	}{
		{
			desc:        "it should return the default db with no args",
			dsUID:       "uid1",
			args:        "",
			expectedKey: "uid1-default",
			expectedDB:  db,
		},
		{
			desc:        "it should return the cached connection for the given args",
			dsUID:       "uid1",
			args:        "foo",
			expectedKey: "uid1-foo",
			existingDB:  db2,
			expectedDB:  db2,
		},
		{
			desc:        "it should create a new connection with the given args",
			dsUID:       "uid1",
			args:        "foo",
			expectedKey: "uid1-foo",
			expectedDB:  db3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ds := &AsyncAWSDatasource{driver: d, SQLDatasource: &sqlds.SQLDatasource{EnableMultipleConnections: true}}
			settings := backend.DataSourceInstanceSettings{UID: tt.dsUID}
			// Add the mandatory default db
			ds.storeDBConnection(dbConnection{db, settings}, nil)
			args := []byte(tt.args)
			if len(args) > 0 {
				ds.storeDBConnection(dbConnection{db, settings}, args)
			}
			if tt.existingDB != nil {
				ds.storeDBConnection(dbConnection{tt.existingDB, settings}, args)
			}

			dbConn, err := ds.getAsyncDBFromQuery(&AsyncQuery{Query: sqlutil.Query{ConnectionArgs: json.RawMessage(tt.args)}}, settings)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if dbConn != tt.expectedDB {
				t.Fatalf("unexpected result %v", dbConn)
			}
		})
	}
}

func Test_Async_QueryData_uses_synchronous_flow_when_header_has_alert_and_expression(t *testing.T) {
	tests := []struct {
		desc    string
		headers map[string]string
	}{
		{
			"alert header",
			map[string]string{fromAlertHeader: "some value"},
		},
		{
			"expression Header",
			map[string]string{fromExpressionHeader: "some value"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			syncCalled := false
			mockQueryData := func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
				syncCalled = true
				return nil, nil
			}
			ds := &AsyncAWSDatasource{sqldsQueryDataHandler: mockQueryData}

			_, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{Headers: tt.headers})
			assert.NoError(t, err)
			assert.True(t, syncCalled)
		})
	}
}

type MockDB struct {
	mock.Mock
}

func (m *MockDB) Ping(context context.Context) error {
	args := m.Called(context)
	return args.Error(0)
}

func (m *MockDB) Begin() (driver.Tx, error) {
	args := m.Called()
	return args.Get(0).(driver.Tx), args.Error(1)
}

func (m *MockDB) CancelQuery(ctx context.Context, queryID string) error {
	args := m.Called(ctx, queryID)
	return args.Error(0)
}
func (m *MockDB) Close() error {
	args := m.Called()
	return args.Error(0)
}
func (m *MockDB) GetQueryID(ctx context.Context, query string, args ...interface{}) (bool, string, error) {
	arg := m.Called(ctx, query, args)
	return arg.Bool(0), arg.String(1), arg.Error(2)
}
func (m *MockDB) GetRows(ctx context.Context, queryID string) (driver.Rows, error) {
	args := m.Called(ctx, queryID)
	return args.Get(0).(driver.Rows), args.Error(1)
}
func (m *MockDB) Prepare(query string) (driver.Stmt, error) {
	args := m.Called(query)
	return args.Get(0).(driver.Stmt), args.Error(1)
}
func (m *MockDB) QueryStatus(ctx context.Context, queryID string) (QueryStatus, error) {
	args := m.Called(ctx, queryID)
	return args.Get(0).(QueryStatus), args.Error(1)
}
func (m *MockDB) StartQuery(ctx context.Context, query string, args ...interface{}) (string, error) {
	arg := m.Called(ctx, query, args)
	return arg.String(0), arg.Error(1)
}

func Test_AsyncDatasource_CheckHealth(t *testing.T) {
	tests := []struct {
		desc             string
		mockPingResponse error
		expected         *backend.CheckHealthResult
	}{
		{
			desc:             "it returns an error when ping fails",
			mockPingResponse: fmt.Errorf("your auth wasn't right"),
			expected: &backend.CheckHealthResult{
				Status:  backend.HealthStatusError,
				Message: "your auth wasn't right",
			},
		},
		{
			desc:             "it returns an ok when the query succeeds",
			mockPingResponse: nil,
			expected: &backend.CheckHealthResult{
				Status:  backend.HealthStatusOk,
				Message: "Data source is working",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			settings := backend.DataSourceInstanceSettings{UID: "uid1"}
			db := new(MockDB)
			db.On("Ping", context.Background()).Return(tt.mockPingResponse)
			dbC := dbConnection{db, settings}
			ds := &AsyncAWSDatasource{dbConnections: sync.Map{}}
			ds.storeDBConnection(dbC, nil)

			result, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
