package datasource

import (
	"context"
	"testing"
)

func TestGetDatasourceID(t *testing.T) {
	// It's not possible to test that GetDatasourceID returns an actual
	// ID because the ctx key is not exported. This just tests the fallback
	// path.
	if id := GetDatasourceID(context.Background()); id != 0 {
		t.Errorf("unexpected id: %d", id)
	}
}

func TestGetDatasourceLastUpdatedTime(t *testing.T) {
	// It's not possible to test that GetDatasourceLastUpdatedTime returns an actual
	// time because the ctx key is not exported. This just tests the fallback
	// path.
	if time := GetDatasourceLastUpdatedTime(context.Background()); time != "" {
		t.Errorf("unexpected time: %s", time)
	}
}
