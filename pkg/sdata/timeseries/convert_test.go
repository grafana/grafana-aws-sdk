package timeseries_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/sdata/timeseries"
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/data"
	"github.com/stretchr/testify/require"
)

func TestLongToMulti(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		ls := timeseries.LongFrame{
			data.NewFrame("",
				data.NewField("time", nil, []time.Time{time.UnixMilli(1), time.UnixMilli(1)}),
				data.NewField("host", nil, []string{"a", "b"}),
				data.NewField("iface", nil, []string{"eth0", "eth0"}),
				data.NewField("in_bytes", nil, []float64{1, 2}),
				data.NewField("out_bytes", nil, []int64{3, 4}),
			).SetMeta(&data.FrameMeta{Type: data.FrameTypeTimeSeriesLong, TypeVersion: data.FrameTypeVersion{0, 1}}),
		}

		multiFrame, err := timeseries.LongToMulti(&ls)
		require.NoError(t, err)

		expectedFrames := timeseries.MultiFrame{
			addFields(emptyFrameWithTypeMD(data.FrameTypeTimeSeriesMulti, timeseries.MultiFrameVersionLatest),
				data.NewField("time", nil, []time.Time{time.UnixMilli(1)}),
				data.NewField("in_bytes", data.Labels{"host": "a", "iface": "eth0"}, []float64{1})),
			addFields(emptyFrameWithTypeMD(data.FrameTypeTimeSeriesMulti, timeseries.MultiFrameVersionLatest),
				data.NewField("time", nil, []time.Time{time.UnixMilli(1)}),
				data.NewField("in_bytes", data.Labels{"host": "b", "iface": "eth0"}, []float64{2})),
			addFields(emptyFrameWithTypeMD(data.FrameTypeTimeSeriesMulti, timeseries.MultiFrameVersionLatest),
				data.NewField("time", nil, []time.Time{time.UnixMilli(1)}),
				data.NewField("out_bytes", data.Labels{"host": "a", "iface": "eth0"}, []int64{3})),
			addFields(emptyFrameWithTypeMD(data.FrameTypeTimeSeriesMulti, timeseries.MultiFrameVersionLatest),
				data.NewField("time", nil, []time.Time{time.UnixMilli(1)}),
				data.NewField("out_bytes", data.Labels{"host": "b", "iface": "eth0"}, []int64{4})),
		}

		require.Equal(t, len(*multiFrame), len(expectedFrames))
		for i := range *multiFrame {
			if diff := cmp.Diff((expectedFrames)[i], (*multiFrame)[i], data.FrameTestCompareOptions()...); diff != "" {
				require.FailNow(t, "mismatch (-want +got):\n", diff)
			}
		}
	})
}
