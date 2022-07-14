package datasource

import (
	"database/sql"
	"fmt"
	"sync"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/driver"
	asyncDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver/async"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v2"
)

// AWSDatasource stores a cache of several instances.
// Each Map will depend on the datasource ID (and connection options):
// - config: Base configuration. It will be used as base to populate datasource settings.
//           It does not depend on connection options (only one per datasource)
// - api: API instace with the common methods to contact the data source API.
// - driver: Abstraction on top the upstream sql.Driver. Defined to handle multiple connections.
// - db: Upstream database (sql.DB).
// - sessionCache: AWS cache. This is not a Map since it does not depend on the datasource.
type AWSDatasource struct {
	sessionCache *awsds.SessionCache
	config       sync.Map
	api          sync.Map
	driver       sync.Map
}

func New() *AWSDatasource {
	ds := &AWSDatasource{sessionCache: awsds.NewSessionCache()}
	return ds
}

func (d *AWSDatasource) storeConfig(config backend.DataSourceInstanceSettings) {
	d.config.Store(config.ID, config)
}

func (s *AWSDatasource) createDB(id int64, args sqlds.Options, settings models.Settings, dr driver.Driver) (*sql.DB, error) {
	db, err := dr.OpenDB()
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to connect to database. Is the hostname and port correct?", err)
	}

	return db, nil
}

func (s *AWSDatasource) createAsyncDB(dr asyncDriver.AsyncDriver) (sqlds.AsyncDB, error) {
	db, err := dr.GetAsyncDB()
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to connect to database. Is the hostname and port correct?", err)
	}

	return db, nil
}

func (d *AWSDatasource) storeAPI(id int64, args sqlds.Options, dsAPI api.AWSAPI) {
	key := connectionKey(id, args)
	d.api.Store(key, dsAPI)
}

func (d *AWSDatasource) loadAPI(id int64, args sqlds.Options) (api.AWSAPI, bool) {
	key := connectionKey(id, args)
	dsAPI, exists := d.api.Load(key)
	if exists {
		return dsAPI.(api.AWSAPI), true
	}
	return nil, false
}

func (s *AWSDatasource) createAPI(id int64, args sqlds.Options, settings models.Settings, loader api.Loader) (api.AWSAPI, error) {
	api, err := loader(s.sessionCache, settings)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to create client", err)
	}
	s.storeAPI(id, args, api)
	return api, err
}

func (d *AWSDatasource) storeDriver(id int64, args sqlds.Options, dr interface{}) {
	key := connectionKey(id, args)
	d.driver.Store(key, dr)
}

func (s *AWSDatasource) createDriver(id int64, args sqlds.Options, settings models.Settings, dsAPI api.AWSAPI, loader driver.Loader) (driver.Driver, error) {
	dr, err := loader(dsAPI)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to create client", err)
	}
	s.storeDriver(id, args, dr)

	return dr, nil
}

func (s *AWSDatasource) createAsyncDriver(id int64, args sqlds.Options, settings models.Settings, dsAPI api.AWSAPI, loader asyncDriver.Loader) (asyncDriver.AsyncDriver, error) {
	dr, err := loader(dsAPI)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to create client", err)
	}
	s.storeDriver(id, args, dr)

	return dr, nil
}

func (d *AWSDatasource) parseSettings(id int64, args sqlds.Options, settings models.Settings) error {
	config, ok := d.config.Load(id)
	if !ok {
		return fmt.Errorf("unable to find stored configuration for datasource %d. Initialize it first", id)
	}
	err := settings.Load(config.(backend.DataSourceInstanceSettings))
	if err != nil {
		return fmt.Errorf("error reading settings: %s", err.Error())
	}
	settings.Apply(args)
	return nil
}

// Init stores the data source configuration. It's needed for the GetDB and GetAPI functions
func (s *AWSDatasource) Init(config backend.DataSourceInstanceSettings) {
	s.storeConfig(config)
}

// GetDB returns a *sql.DB. It will use the loader functions to initialize the required
// settings, API and driver and finally create a DB.
func (s *AWSDatasource) GetDB(
	id int64,
	options sqlds.Options,
	settingsLoader models.Loader,
	apiLoader api.Loader,
	driverLoader driver.Loader,
) (*sql.DB, error) {
	settings := settingsLoader()
	err := s.parseSettings(id, options, settings)
	if err != nil {
		return nil, err
	}

	dsAPI, err := s.createAPI(id, options, settings, apiLoader)
	if err != nil {
		return nil, err
	}

	dr, err := s.createDriver(id, options, settings, dsAPI, driverLoader)
	if err != nil {
		return nil, err
	}

	return s.createDB(id, options, settings, dr)
}

func (s *AWSDatasource) GetAsyncDB(id int64,
	options sqlds.Options,
	settingsLoader models.Loader,
	apiLoader api.Loader,
	driverLoader asyncDriver.Loader) (sqlds.AsyncDB, error) {
	settings := settingsLoader()
	err := s.parseSettings(id, options, settings)
	if err != nil {
		return nil, err
	}

	dsAPI, err := s.createAPI(id, options, settings, apiLoader)
	if err != nil {
		return nil, err
	}

	dr, err := s.createAsyncDriver(id, options, settings, dsAPI, driverLoader)
	if err != nil {
		return nil, err
	}

	return s.createAsyncDB(dr)
}

// GetAPI returns an API interface. When called multiple times with the same id and options, it
// will return a cached version of the API. The first time, it will use the loader
// functions to initialize the required settings and API.
func (s *AWSDatasource) GetAPI(
	id int64,
	options sqlds.Options,
	settingsLoader models.Loader,
	apiLoader api.Loader,
) (api.AWSAPI, error) {
	cachedAPI, exists := s.loadAPI(id, options)
	if exists {
		return cachedAPI, nil
	}

	// create new api
	settings := settingsLoader()
	err := s.parseSettings(id, options, settings)
	if err != nil {
		return nil, err
	}
	return s.createAPI(id, options, settings, apiLoader)
}
