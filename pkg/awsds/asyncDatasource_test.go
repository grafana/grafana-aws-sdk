package awsds

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/data/sqlutil"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/backend/instancemgmt"
	"github.com/grafana/sqlds/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func (d fakeDriver) Connect(context.Context, backend.DataSourceInstanceSettings, json.RawMessage) (*sql.DB, error) {
	return nil, fmt.Errorf("trying to use non-allowed auth method default")
}

func (d fakeDriver) Settings(context.Context, backend.DataSourceInstanceSettings) sqlds.DriverSettings {
	return sqlds.DriverSettings{}
}

func (d fakeDriver) Macros() sqlds.Macros            { return sqlds.Macros{} }
func (d fakeDriver) Converters() []sqlutil.Converter { return nil }

func TestNewDatasource_DefersAsyncDBOnGetAsyncDBError(t *testing.T) {
	d := &fakeDriver{openDBfn: func() (AsyncDB, error) {
		return nil, fmt.Errorf("trying to use non-allowed auth method default")
	}}
	ds := NewAsyncAWSDatasource(d)
	settings := backend.DataSourceInstanceSettings{UID: "uid-1", ID: 1}

	inst, err := ds.NewDatasource(context.Background(), settings)
	require.NoError(t, err)
	require.NotNil(t, inst)

	key := defaultKey("uid-1")
	conn, ok := ds.getDBConnection(key)
	require.True(t, ok, "settings stub must be stored")
	require.Nil(t, conn.db)
	require.Equal(t, "uid-1", conn.settings.UID)
}

func TestGetAsyncDBFromQuery_ConnectsWhenDeferred(t *testing.T) {
	live := &fakeAsyncDB{}
	calls := 0
	d := &fakeDriver{openDBfn: func() (AsyncDB, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("not ready")
		}
		return live, nil
	}}
	ds := NewAsyncAWSDatasource(d)
	settings := backend.DataSourceInstanceSettings{UID: "uid1"}
	_, err := ds.NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	db, err := ds.getAsyncDBFromQuery(context.Background(), &AsyncQuery{}, "uid1")
	require.NoError(t, err)
	require.Equal(t, live, db)
}

func TestGetAsyncDBFromQuery_ConcurrentDeferredConnectOpensOnce(t *testing.T) {
	const workers = 16

	live := &fakeAsyncDB{}
	gate := make(chan struct{})
	var calls atomic.Int32
	d := &fakeDriver{openDBfn: func() (AsyncDB, error) {
		if calls.Add(1) == 1 {
			return nil, fmt.Errorf("not ready")
		}
		<-gate
		return live, nil
	}}
	ds := NewAsyncAWSDatasource(d)
	settings := backend.DataSourceInstanceSettings{UID: "uid1"}
	_, err := ds.NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			db, err := ds.getAsyncDBFromQuery(context.Background(), &AsyncQuery{}, "uid1")
			if err == nil && db != live {
				err = fmt.Errorf("expected cached live db")
			}
			errs <- err
		}()
	}

	close(start)
	deadline := time.Now().Add(time.Second)
	for calls.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(25 * time.Millisecond)
	close(gate)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, int32(2), calls.Load(), "expected bootstrap plus one on-demand GetAsyncDB call")
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
			ds.storeDBConnection(context.Background(), key, dbConnection{db: db, settings: settings})
			if tt.args != "" {
				key = keyWithConnectionArgs(tt.dsUID, []byte(tt.args))
			}
			if tt.existingDB != nil {
				ds.storeDBConnection(context.Background(), key, dbConnection{db: tt.existingDB, settings: settings})
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

func Test_Async_QueryData_error_source(t *testing.T) {
	awsErr := errors.New("operation error Athena: GetQueryResults, https response error StatusCode: 400")
	tests := []struct {
		desc     string
		err      error
		expected backend.ErrorSource
	}{
		{
			desc:     "downstream error wrapped as a plugin error by sqlds",
			err:      backend.NewErrorWithSource(fmt.Errorf("%w: %w", sqlds.ErrorQuery, backend.DownstreamError(awsErr)), backend.ErrorSourcePlugin),
			expected: backend.ErrorSourceDownstream,
		},
		{
			desc:     "downstream error",
			err:      backend.DownstreamError(awsErr),
			expected: backend.ErrorSourceDownstream,
		},
		{
			desc:     "plugin error",
			err:      backend.PluginError(awsErr),
			expected: backend.ErrorSourcePlugin,
		},
		{
			desc:     "error without a source",
			err:      awsErr,
			expected: backend.ErrorSourcePlugin,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			ds := NewAsyncAWSDatasource(&fakeDriver{openDBfn: func() (AsyncDB, error) { return nil, tt.err }})
			settings := backend.DataSourceInstanceSettings{UID: "uid1"}
			ds.storeDBConnection(context.Background(), defaultKey(settings.UID), dbConnection{settings: settings})

			resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
				PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
				Queries:       []backend.DataQuery{{RefID: "A", JSON: []byte(`{"meta": {"queryFlow": "async"}}`)}},
			})
			require.NoError(t, err)
			require.ErrorIs(t, resp.Responses["A"].Error, awsErr)
			assert.Equal(t, tt.expected, resp.Responses["A"].ErrorSource)
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
				db:       db,
				settings: backend.DataSourceInstanceSettings{UID: "uid1"},
			}
			ds := &AsyncAWSDatasource{}
			ds.storeDBConnection(context.Background(), defaultKey("uid1"), dbC)

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

func TestCheckHealth_ConnectsWhenDeferred(t *testing.T) {
	live := &fakeAsyncDB{}
	calls := 0
	d := &fakeDriver{openDBfn: func() (AsyncDB, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("not ready")
		}
		return live, nil
	}}
	ds := NewAsyncAWSDatasource(d)
	settings := backend.DataSourceInstanceSettings{UID: "uid1"}
	_, err := ds.NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	result, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
	})
	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusOk, result.Status)
	require.Equal(t, "Data source is working", result.Message)
}

func TestCheckHealth_ReturnsDeferredConnectError(t *testing.T) {
	d := &fakeDriver{openDBfn: func() (AsyncDB, error) {
		return nil, fmt.Errorf("trying to use non-allowed auth method default")
	}}
	ds := NewAsyncAWSDatasource(d)
	settings := backend.DataSourceInstanceSettings{UID: "uid1"}
	_, err := ds.NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	result, err := ds.CheckHealth(context.Background(), &backend.CheckHealthRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
	})
	require.NoError(t, err)
	require.Equal(t, backend.HealthStatusError, result.Status)
	require.Equal(t, "trying to use non-allowed auth method default", result.Message)
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

type panickingStatusDB struct{ fakeAsyncDB }

func (panickingStatusDB) QueryStatus(context.Context, string) (QueryStatus, error) {
	var reason *string
	return QueryUnknown, errors.New(*reason)
}

func Test_QueryData_AsyncQueryPanicReturnsPluginError(t *testing.T) {
	ds := NewAsyncAWSDatasource(&fakeDriver{openDBfn: func() (AsyncDB, error) { return panickingStatusDB{}, nil }})
	settings := backend.DataSourceInstanceSettings{UID: "uid1"}
	_, err := ds.NewDatasource(context.Background(), settings)
	require.NoError(t, err)

	resp, err := ds.QueryData(context.Background(), &backend.QueryDataRequest{
		PluginContext: backend.PluginContext{DataSourceInstanceSettings: &settings},
		Queries: []backend.DataQuery{
			{RefID: "A", JSON: []byte(`{"meta": {"queryFlow": "async"}, "queryID": "q1", "rawSql": "SELECT 1"}`)},
			{RefID: "B", JSON: []byte(`{"meta": {"queryFlow": "async"}, "rawSql": "SELECT 1"}`)},
		},
	})
	require.NoError(t, err)
	require.Error(t, resp.Responses["A"].Error)
	assert.Contains(t, resp.Responses["A"].Error.Error(), "async query execution panic")
	assert.Equal(t, backend.ErrorSourcePlugin, resp.Responses["A"].ErrorSource)
	require.NoError(t, resp.Responses["B"].Error)
	require.Len(t, resp.Responses["B"].Frames, 1)
	assert.Equal(t, queryMeta{Status: "started"}, resp.Responses["B"].Frames[0].Meta.Custom)
}

type trackedAsyncDB struct {
	fakeAsyncDB
	closed atomic.Bool
}

func (db *trackedAsyncDB) Close() error {
	db.closed.Store(true)
	return nil
}

type closableConnector struct{ closed atomic.Bool }

func (c *closableConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("not implemented")
}
func (c *closableConnector) Driver() driver.Driver { return nil }
func (c *closableConnector) Close() error {
	c.closed.Store(true)
	return nil
}

type recordingDriver struct {
	fakeDriver
	sqlConnector closableConnector
	argsGate     chan struct{}

	mu           sync.Mutex
	openedByArgs map[string][]*trackedAsyncDB
}

func (d *recordingDriver) GetAsyncDB(_ context.Context, _ backend.DataSourceInstanceSettings, args json.RawMessage) (AsyncDB, error) {
	db := &trackedAsyncDB{}
	d.mu.Lock()
	d.openedByArgs[string(args)] = append(d.openedByArgs[string(args)], db)
	d.mu.Unlock()
	if len(args) > 0 && d.argsGate != nil {
		<-d.argsGate
	}
	return db, nil
}

func (d *recordingDriver) Connect(context.Context, backend.DataSourceInstanceSettings, json.RawMessage) (*sql.DB, error) {
	return sql.OpenDB(&d.sqlConnector), nil
}

func (d *recordingDriver) openedFor(args string) []*trackedAsyncDB {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.openedByArgs[args])
}

const (
	argsA = `{"region":"a"}`
	argsB = `{"region":"b"}`
)

func newRecordingDatasource(t *testing.T) (*AsyncAWSDatasource, *recordingDriver) {
	t.Helper()
	drv := &recordingDriver{openedByArgs: map[string][]*trackedAsyncDB{}}
	ds := NewAsyncAWSDatasource(drv)
	ds.EnableMultipleConnections = true
	_, err := ds.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{UID: "uid1"})
	require.NoError(t, err)
	return ds, drv
}

func queryWithArgs(connArgs string) *AsyncQuery {
	return &AsyncQuery{Query: sqlutil.Query{ConnectionArgs: json.RawMessage(connArgs)}}
}

func lookupAsyncDB(t *testing.T, ds *AsyncAWSDatasource, connArgs string) AsyncDB {
	t.Helper()
	db, err := ds.getAsyncDBFromQuery(context.Background(), queryWithArgs(connArgs), "uid1")
	require.NoError(t, err)
	return db
}

func TestDispose_ClosesEveryAsyncDBAndTheSQLDatasource(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		drv := &recordingDriver{openedByArgs: map[string][]*trackedAsyncDB{}}
		ds := NewAsyncAWSDatasource(drv)
		ds.EnableMultipleConnections = true
		inst, err := ds.NewDatasource(context.Background(), backend.DataSourceInstanceSettings{UID: "uid1"})
		require.NoError(t, err)
		lookupAsyncDB(t, ds, argsA)
		lookupAsyncDB(t, ds, argsB)

		disposer, ok := inst.(instancemgmt.InstanceDisposer)
		require.True(t, ok)
		disposer.Dispose()

		for _, args := range []string{"", argsA, argsB} {
			opened := drv.openedFor(args)
			require.Len(t, opened, 1, args)
			assert.True(t, opened[0].closed.Load(), "AsyncDB for %q must be closed", args)
		}
		assert.True(t, drv.sqlConnector.closed.Load(), "the sqlds connection must be closed")
	})
}

func TestGetAsyncDBFromQuery_ClosesAsyncDBIdlePastTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ds, drv := newRecordingDatasource(t)
		lookupAsyncDB(t, ds, argsA)

		time.Sleep(5*time.Minute + time.Second)
		lookupAsyncDB(t, ds, argsB)

		assert.True(t, drv.openedFor(argsA)[0].closed.Load(), "idle AsyncDB must be closed")
		_, ok := ds.getDBConnection(keyWithConnectionArgs("uid1", json.RawMessage(argsA)))
		assert.False(t, ok, "idle AsyncDB must be removed")
		assert.False(t, drv.openedFor("")[0].closed.Load(), "the default AsyncDB is never evicted")

		lookupAsyncDB(t, ds, argsA)
		assert.Len(t, drv.openedFor(argsA), 2, "a lookup after eviction opens a new AsyncDB")
		ds.Dispose()
	})
}

func TestGetAsyncDBFromQuery_KeepsAsyncDBUsedWithinIdleTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ds, drv := newRecordingDatasource(t)
		first := lookupAsyncDB(t, ds, argsA)

		time.Sleep(4 * time.Minute)
		lookupAsyncDB(t, ds, argsA)
		time.Sleep(4 * time.Minute)
		lookupAsyncDB(t, ds, argsB)

		assert.Same(t, first, lookupAsyncDB(t, ds, argsA))
		assert.False(t, drv.openedFor(argsA)[0].closed.Load())
		assert.Len(t, drv.openedFor(argsA), 1)
		ds.Dispose()
	})
}

func TestGetAsyncDBFromQuery_ConcurrentFirstLookupsOpenOneAsyncDB(t *testing.T) {
	const workers = 16
	ds, drv := newRecordingDatasource(t)
	t.Cleanup(ds.Dispose)
	drv.argsGate = make(chan struct{})

	dbs := make([]AsyncDB, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			dbs[i], errs[i] = ds.getAsyncDBFromQuery(context.Background(), queryWithArgs(argsA), "uid1")
		})
	}
	// Workers waiting on a sync.Mutex are not durably blocked, so synctest cannot tell when all have arrived.
	deadline := time.Now().Add(100 * time.Millisecond)
	for len(drv.openedFor(argsA)) < workers && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(drv.argsGate)
	wg.Wait()

	assert.Len(t, drv.openedFor(argsA), 1)
	for i := range workers {
		require.NoError(t, errs[i])
		assert.Same(t, dbs[0], dbs[i])
	}
}

func TestGetAsyncDBFromQuery_ClosesAsyncDBOpenedDuringDispose(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ds, drv := newRecordingDatasource(t)
		drv.argsGate = make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan AsyncDB)
		go func() {
			db, _ := ds.getAsyncDBFromQuery(ctx, queryWithArgs(argsA), "uid1")
			result <- db
		}()

		synctest.Wait()
		ds.Dispose()
		close(drv.argsGate)
		db := <-result

		require.NotNil(t, db)
		assert.False(t, drv.openedFor(argsA)[0].closed.Load(), "the request that opened it can still use it")
		cancel()
		synctest.Wait()
		assert.True(t, drv.openedFor(argsA)[0].closed.Load(), "AsyncDB must be closed when its request ends")
		_, ok := ds.getDBConnection(keyWithConnectionArgs("uid1", json.RawMessage(argsA)))
		assert.False(t, ok, "nothing is cached after Dispose")
	})
}

func TestDispose_LeavesNoGoroutineRunning(t *testing.T) {
	// synctest.Test fails if a goroutine started in the bubble is still blocked when it returns.
	synctest.Test(t, func(t *testing.T) {
		ds, _ := newRecordingDatasource(t)
		lookupAsyncDB(t, ds, argsA)
		time.Sleep(6 * time.Minute)
		lookupAsyncDB(t, ds, argsB)
		ds.Dispose()
	})
}
