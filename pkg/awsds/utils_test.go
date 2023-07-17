package awsds

import (
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
	"github.com/stretchr/testify/assert"
)

func TestShouldCacheQuery(t *testing.T) {

	testcases := []struct {
		name        string
		customMeta  map[string]interface{}
		shouldCache bool
	}{
		{
			"sync query should cache",
			map[string]interface{}{"foo": "bar"},
			true,
		},
		{
			"starting async query should cache",
			map[string]interface{}{"status": "started"},
			true,
		},
		{
			"submitted async query should not cache",
			map[string]interface{}{"status": "submitted"},
			false,
		},
		{
			"running async query should not cache",
			map[string]interface{}{"status": "running"},
			false,
		},
		{
			"done async query should cache",
			map[string]interface{}{"status": "done"},
			true,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			fakeResponse := &backend.QueryDataResponse{
				Responses: backend.Responses{
					"a": backend.DataResponse{
						Frames: data.Frames{
							&data.Frame{
								Meta: &data.FrameMeta{Custom: tc.customMeta},
							},
						},
					},
				},
			}
			res := ShouldCacheQuery(fakeResponse)
			assert.Equal(t, tc.shouldCache, res)
		})
	}
}
