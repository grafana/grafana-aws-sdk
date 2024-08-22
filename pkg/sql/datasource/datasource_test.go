package datasource

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	asyncDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver/async"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	sqlApi "github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	sqlDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v4"
)

type fakeLoader struct {
	driver sqlDriver.Driver
}

func (m fakeLoader) LoadSettings(_ context.Context) models.Settings {
	return &fakeSettings{}
}

func (m fakeLoader) LoadAPI(_ context.Context, _ *awsds.SessionCache, _ models.Settings) (sqlApi.AWSAPI, error) {
	return fakeAPI{}, nil
}

func (m fakeLoader) LoadDriver(_ context.Context, _ sqlApi.AWSAPI) (sqlDriver.Driver, error) {
	return m.driver, nil
}

func (m fakeLoader) LoadAsyncDriver(_ context.Context, _ sqlApi.AWSAPI) (asyncDriver.Driver, error) {
	return nil, nil
}
func newFakeLoader(db *sql.DB) Loader {
	return fakeLoader{driver: &fakeDriver{db: db}}

}

func TestNew(t *testing.T) {
	ds := New(newFakeLoader(nil))
	impl, ok := ds.(*awsClient)
	if !ok {
		t.Errorf("unexpected underlying type: %t", ds)
	}

	if impl.sessionCache == nil {
		t.Errorf("missing initialization")
	}
}

func TestInit(t *testing.T) {
	config := backend.DataSourceInstanceSettings{
		ID: 100,
	}
	ds := &awsClient{loader: newFakeLoader(nil)}
	ds.Init(config)
	if _, ok := ds.config.Load(config.ID); !ok {
		t.Errorf("missing config")
	}
}

type fakeDriver struct {
	db     *sql.DB
	closed bool
}

func (f *fakeDriver) Open(_ string) (driver.Conn, error) {
	return nil, nil
}

func (f *fakeDriver) Closed() bool {
	return f.closed
}

func (f *fakeDriver) OpenDB() (*sql.DB, error) {
	return f.db, nil
}

type fakeAPI struct {
	sqlApi.AWSAPI
}

func TestLoadAPI(t *testing.T) {
	api := &fakeAPI{}
	tests := []struct {
		description string
		id          int64
		args        sqlds.Options
		api         *fakeAPI
		res         *fakeAPI
	}{
		{
			description: "it should return a stored api without args",
			id:          1,
			args:        sqlds.Options{},
			api:         api,
			res:         api,
		},
		{
			description: "it should return a stored api with args",
			id:          1,
			args:        sqlds.Options{"foo": "bar"},
			api:         api,
			res:         api,
		},
		{
			description: "it should return an empty response",
			id:          1,
			args:        sqlds.Options{"foo": "bar"},
			api:         nil,
			res:         nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			ds := &awsClient{loader: newFakeLoader(nil)}
			key := connectionKey(tt.id, tt.args)
			if tt.api != nil {
				ds.api.Store(key, tt.api)
			}
			res, exists := ds.loadAPI(tt.id, tt.args)
			if res != tt.res && (res != nil || tt.res != nil) {
				t.Errorf("unexpected result %v", res)
			}
			if tt.res != nil && !exists {
				t.Errorf("should return true")
			}
		})
	}
}

type fakeSettings struct {
	settings backend.DataSourceInstanceSettings
	modifier sqlds.Options
}

func (f *fakeSettings) Load(c backend.DataSourceInstanceSettings) error {
	f.settings = c
	return nil
}

func (f *fakeSettings) Apply(args sqlds.Options) {
	f.modifier = args
}

func TestParseSettings(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &awsClient{loader: newFakeLoader(nil)}
	ds.config.Store(id, backend.DataSourceInstanceSettings{ID: id})

	settings := &fakeSettings{}
	err := ds.parseSettings(id, args, settings)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if settings.settings.ID != id {
		t.Errorf("failed to load config")
	}
	if settings.modifier["foo"] != "bar" {
		t.Errorf("failed to apply modifier")
	}
}

func TestCreateAPI(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &awsClient{loader: newFakeLoader(nil)}
	key := connectionKey(id, args)
	settings := &fakeSettings{}
	ctx := context.Background()

	api, err := ds.createAPI(ctx, id, args, settings)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if !cmp.Equal(api, fakeAPI{}) {
		t.Errorf("unexpected result api %v", cmp.Diff(api, fakeAPI{}))
	}
	cachedAPI, ok := ds.api.Load(key)
	if !ok || !cmp.Equal(cachedAPI, fakeAPI{}) {
		t.Errorf("unexpected cached api %v", cmp.Diff(cachedAPI, fakeAPI{}))
	}
}

func TestCreateDriver(t *testing.T) {
	ctx := context.Background()
	loader := newFakeLoader(nil)
	ds := &awsClient{loader: loader}
	api, err := ds.createAPI(ctx, 0, sqlds.Options{}, loader.LoadSettings(ctx))
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	dr, err := ds.createDriver(context.Background(), api)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if dr == nil {
		t.Errorf("unexpected result driver %v", dr)
	}
}

func TestCreateDB(t *testing.T) {
	db := &sql.DB{}
	dr := &fakeDriver{db: db}
	ds := &awsClient{loader: newFakeLoader(db)}

	res, err := ds.createDB(dr)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if res != db {
		t.Errorf("unexpected result db %v", res)
	}
}

func TestGetDB(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &awsClient{loader: newFakeLoader(&sql.DB{})}
	config := backend.DataSourceInstanceSettings{ID: id}
	ds.Init(config)

	res, err := ds.GetDB(context.Background(), config.ID, args)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if res == nil {
		t.Errorf("unexpected result db %v", res)
	}
}

func TestGetAPI(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &awsClient{loader: fakeLoader{}}
	config := backend.DataSourceInstanceSettings{ID: id}
	ds.Init(config)
	key := connectionKey(id, args)

	api, err := ds.GetAPI(context.Background(), id, args)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if !cmp.Equal(api, fakeAPI{}) {
		t.Errorf("unexpected result api %v", cmp.Diff(api, fakeAPI{}))
	}
	cachedAPI, ok := ds.api.Load(key)
	if !ok || !cmp.Equal(cachedAPI, fakeAPI{}) {
		t.Errorf("unexpected cached api %v", cmp.Diff(cachedAPI, fakeAPI{}))
	}
}
