package datasource

import (
	"database/sql"
	"database/sql/driver"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	sqlDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v2"
)

func TestNew(t *testing.T) {
	ds := New()
	if ds.sessionCache == nil {
		t.Errorf("missing initialization")
	}
}

func TestInit(t *testing.T) {
	config := backend.DataSourceInstanceSettings{
		ID: 100,
	}
	ds := &AWSDatasource{}
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
	api.AWSAPI
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
			ds := &AWSDatasource{}
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
	ds := &AWSDatasource{}
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

func fakeAPILoader(cache *awsds.SessionCache, settings models.Settings) (api.AWSAPI, error) {
	return fakeAPI{}, nil
}

func TestCreateAPI(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &AWSDatasource{}
	key := connectionKey(id, args)
	settings := &fakeSettings{}

	api, err := ds.createAPI(id, args, settings, fakeAPILoader)
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

func fakeDriverLoader(api.AWSAPI) (sqlDriver.Driver, error) {
	return &fakeDriver{db: &sql.DB{}}, nil
}

func TestCreateDriver(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &AWSDatasource{}
	key := connectionKey(id, args)
	api := fakeAPI{}
	settings := &fakeSettings{}

	dr, err := ds.createDriver(id, args, settings, api, fakeDriverLoader)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if dr == nil {
		t.Errorf("unexpected result driver %v", dr)
	}
	cachedDriver, ok := ds.driver.Load(key)
	if !ok || cachedDriver == nil {
		t.Errorf("unexpected cached driver %v", cachedDriver)
	}
}

func TestCreateDB(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &AWSDatasource{}
	db := &sql.DB{}
	dr := &fakeDriver{db: db}
	settings := &fakeSettings{}

	res, err := ds.createDB(id, args, settings, dr)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if res != db {
		t.Errorf("unexpected result db %v", res)
	}
}

func fakeSettingsLoader() models.Settings {
	return &fakeSettings{}
}

func TestGetDB(t *testing.T) {
	id := int64(1)
	args := sqlds.Options{"foo": "bar"}
	ds := &AWSDatasource{}
	config := backend.DataSourceInstanceSettings{ID: id}
	ds.Init(config)

	res, err := ds.GetDB(config.ID, args, fakeSettingsLoader, fakeAPILoader, fakeDriverLoader)
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
	ds := &AWSDatasource{}
	config := backend.DataSourceInstanceSettings{ID: id}
	ds.Init(config)
	key := connectionKey(id, args)

	api, err := ds.GetAPI(id, args, fakeSettingsLoader, fakeAPILoader)
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
