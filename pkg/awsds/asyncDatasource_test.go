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
	"github.com/grafana/sqlds/v4"
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

func (d fakeDriver) GetAsyncDB(context.Context, backend.DataSourceInstanceSettings, json.RawMessage) (db AsyncDB, err error) {
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
			key := defaultKey(tt.dsUID)
			// Add the mandatory default db
			ds.storeDBConnection(key, dbConnection{db, settings})
			if tt.args != "" {
				key = keyWithConnectionArgs(tt.dsUID, []byte(tt.args))
			}
			if tt.existingDB != nil {
				ds.storeDBConnection(key, dbConnection{tt.existingDB, settings})
			}

			dbConn, err := ds.getAsyncDBFromQuery(context.Background(), &AsyncQuery{Query: sqlutil.Query{ConnectionArgs: json.RawMessage(tt.args)}}, tt.dsUID)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if key != tt.expectedKey {
				t.Fatalf("unexpected cache key %s", key)
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
			db := new(MockDB)
			db.On("Ping", context.Background()).Return(tt.mockPingResponse)
			dbC := dbConnection{
				db,
				backend.DataSourceInstanceSettings{UID: "uid1"},
			}
			ds := &AsyncAWSDatasource{dbConnections: sync.Map{}}
			ds.dbConnections.Store(defaultKey("uid1"), dbC)

			result, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{
				PluginContext: backend.PluginContext{
					DataSourceInstanceSettings: &backend.DataSourceInstanceSettings{UID: "uid1"},
				},
			})
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_isAsyncFlow(t *testing.T) {
	tests := []struct {
		name     string
		json     []byte
		expected bool
	}{
		{
			name:     "async flow enabled",
			json:     []byte(`{"meta": {"queryFlow": "async"}}`),
			expected: true,
		},
		{
			name:     "async flow disabled",
			json:     []byte(`{"meta": {"queryFlow": "sync"}}`),
			expected: false,
		},
		{
			name:     "no meta field",
			json:     []byte(`{"rawSql": "SELECT 1"}`),
			expected: false,
		},
		{
			name:     "empty meta",
			json:     []byte(`{"meta": {}}`),
			expected: false,
		},
		{
			name:     "malformed JSON - incomplete object",
			json:     []byte(`{malformed json`),
			expected: false,
		},
		{
			name:     "malformed JSON - invalid syntax",
			json:     []byte(`{"meta": {"queryFlow": "async"`),
			expected: false,
		},
		{
			name:     "malformed JSON - not JSON",
			json:     []byte(`not json at all`),
			expected: false,
		},
		{
			name:     "empty JSON",
			json:     []byte(``),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := backend.DataQuery{
				RefID: "A",
				JSON:  tt.json,
			}

			result := isAsyncFlow(query)
			assert.Equal(t, tt.expected, result, "isAsyncFlow result should match expected for: %s", tt.name)
		})
	}
}

func Test_QueryData_MalformedJSON_FallsBackToSync(t *testing.T) {
	req := &backend.QueryDataRequest{
		Queries: []backend.DataQuery{
			{
				RefID: "A",
				JSON:  []byte(`{malformed json`), // Invalid JSON that will cause GetQuery to fail
			},
		},
	}

	syncCalled := false
	mockSyncHandler := func(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
		syncCalled = true
		return &backend.QueryDataResponse{}, nil
	}

	ds := &AsyncAWSDatasource{
		sqldsQueryDataHandler: mockSyncHandler,
	}

	_, err := ds.QueryData(context.Background(), req)
	assert.NoError(t, err, "QueryData should not error when handling malformed JSON")
	assert.True(t, syncCalled, "QueryData should fall back to sync flow when isAsyncFlow returns false due to malformed JSON")
}
