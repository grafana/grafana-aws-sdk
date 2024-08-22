package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/grafana/grafana-aws-sdk/pkg/awsds"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/api"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/driver"
	asyncDriver "github.com/grafana/grafana-aws-sdk/pkg/sql/driver/async"
	"github.com/grafana/grafana-aws-sdk/pkg/sql/models"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/sqlds/v4"
)

// AWSClient provides creation and caching of sessions, database connections, and API clients
type AWSClient interface {
	Init(config backend.DataSourceInstanceSettings)
	GetDB(ctx context.Context, id int64, options sqlds.Options) (*sql.DB, error)
	GetAsyncDB(ctx context.Context, id int64, options sqlds.Options) (awsds.AsyncDB, error)
	GetAPI(ctx context.Context, id int64, options sqlds.Options) (api.AWSAPI, error)
}

type Loader interface {
	LoadSettings(context.Context) models.Settings
	LoadAPI(context.Context, *awsds.SessionCache, models.Settings) (api.AWSAPI, error)
	LoadDriver(context.Context, api.AWSAPI) (driver.Driver, error)
	LoadAsyncDriver(context.Context, api.AWSAPI) (asyncDriver.Driver, error)
}

// awsClient provides creation and caching of several types of instances.
// Each Map will depend on the datasource ID (and connection options):
//   - sessionCache: AWS cache. This is not a Map since it does not depend on the datasource.
//   - config: Base configuration. It will be used as base to populate datasource settings.
//     It does not depend on connection options (only one per datasource)
//   - api: API instance with the common methods to contact the data source API.
type awsClient struct {
	sessionCache *awsds.SessionCache
	config       sync.Map
	api          sync.Map

	loader Loader
}

func New(loader Loader) AWSClient {
	ds := &awsClient{sessionCache: awsds.NewSessionCache(), loader: loader}
	return ds
}

func (ds *awsClient) storeConfig(config backend.DataSourceInstanceSettings) {
	ds.config.Store(config.ID, config)
}

func (ds *awsClient) createDB(dr driver.Driver) (*sql.DB, error) {
	db, err := dr.OpenDB()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to connect to database (check hostname and port?)", err)
	}

	return db, nil
}

func (ds *awsClient) createAsyncDB(dr asyncDriver.Driver) (awsds.AsyncDB, error) {
	db, err := dr.GetAsyncDB()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to connect to database (check hostname and port)", err)
	}

	return db, nil
}

func (ds *awsClient) storeAPI(id int64, args sqlds.Options, dsAPI api.AWSAPI) {
	key := connectionKey(id, args)
	ds.api.Store(key, dsAPI)
}

func (ds *awsClient) loadAPI(id int64, args sqlds.Options) (api.AWSAPI, bool) {
	key := connectionKey(id, args)
	dsAPI, exists := ds.api.Load(key)
	if exists {
		return dsAPI.(api.AWSAPI), true
	}
	return nil, false
}

func (ds *awsClient) createAPI(ctx context.Context, id int64, args sqlds.Options, settings models.Settings) (api.AWSAPI, error) {
	dsAPI, err := ds.loader.LoadAPI(ctx, ds.sessionCache, settings)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to create client", err)
	}
	ds.storeAPI(id, args, dsAPI)
	return dsAPI, err
}

func (ds *awsClient) createDriver(ctx context.Context, dsAPI api.AWSAPI) (driver.Driver, error) {
	dr, err := ds.loader.LoadDriver(ctx, dsAPI)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to create client", err)
	}

	return dr, nil
}

func (ds *awsClient) createAsyncDriver(ctx context.Context, dsAPI api.AWSAPI) (asyncDriver.Driver, error) {
	dr, err := ds.loader.LoadAsyncDriver(ctx, dsAPI)
	if err != nil {
		return nil, fmt.Errorf("%w: Failed to create client", err)
	}

	return dr, nil
}

func (ds *awsClient) parseSettings(id int64, args sqlds.Options, settings models.Settings) error {
	config, ok := ds.config.Load(id)
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
func (ds *awsClient) Init(config backend.DataSourceInstanceSettings) {
	ds.storeConfig(config)
}

// GetDB returns a *sql.DB. It will use the loader functions to initialize the required
// settings, API and driver and finally create a DB.
func (ds *awsClient) GetDB(
	ctx context.Context,
	id int64,
	options sqlds.Options,
) (*sql.DB, error) {
	settings := ds.loader.LoadSettings(ctx)
	err := ds.parseSettings(id, options, settings)
	if err != nil {
		return nil, err
	}

	dsAPI, err := ds.createAPI(ctx, id, options, settings)
	if err != nil {
		return nil, err
	}

	dr, err := ds.createDriver(ctx, dsAPI)
	if err != nil {
		return nil, err
	}

	return ds.createDB(dr)
}

// GetAsyncDB returns a sqlds.AsyncDB. It will use the loader functions to initialize the required
// settings, API and driver and finally create a DB.
func (ds *awsClient) GetAsyncDB(
	ctx context.Context,
	id int64,
	options sqlds.Options,
) (awsds.AsyncDB, error) {
	settings := ds.loader.LoadSettings(ctx)
	err := ds.parseSettings(id, options, settings)
	if err != nil {
		return nil, err
	}

	dsAPI, err := ds.createAPI(ctx, id, options, settings)
	if err != nil {
		return nil, err
	}

	dr, err := ds.createAsyncDriver(ctx, dsAPI)
	if err != nil {
		return nil, err
	}

	return ds.createAsyncDB(dr)
}

// GetAPI returns an API interface. When called multiple times with the same id and options, it
// will return a cached version of the API. The first time, it will use the loader
// functions to initialize the required settings and API.
func (ds *awsClient) GetAPI(
	ctx context.Context,
	id int64,
	options sqlds.Options,
) (api.AWSAPI, error) {
	cachedAPI, exists := ds.loadAPI(id, options)
	if exists {
		return cachedAPI, nil
	}

	// create new api
	settings := ds.loader.LoadSettings(ctx)
	err := ds.parseSettings(id, options, settings)
	if err != nil {
		return nil, err
	}
	return ds.createAPI(ctx, id, options, settings)
}
