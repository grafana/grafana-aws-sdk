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
		nilResponse bool
		shouldCache bool
	}{
		{
			"sync query should cache",
			map[string]interface{}{"foo": "bar"},
			false,
			true,
		},
		{
			"starting async query should cache",
			map[string]interface{}{"status": "started"},
			false,
			true,
		},
		{
			"submitted async query should not cache",
			map[string]interface{}{"status": "submitted"},
			false,
			false,
		},
		{
			"running async query should not cache",
			map[string]interface{}{"status": "running"},
			false,
			false,
		},
		{
			"done async query should cache",
			map[string]interface{}{"status": "done"},
			false,
			true,
		},
		{
			"should handle nil response",
			nil,
			true,
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
			if tc.nilResponse {
				fakeResponse = nil
			}
			res := ShouldCacheQuery(fakeResponse)
			assert.Equal(t, tc.shouldCache, res)
		})
	}
}
