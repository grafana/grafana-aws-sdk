package reader_test

import (
	"fmt"
	"testing"

	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/sdata/numeric"
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/sdata/reader"
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/sdata/timeseries"
	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/data"
	"github.com/stretchr/testify/require"
)

func TestCanReadBasedOnMeta(t *testing.T) {
	t.Run("basic test", func(t *testing.T) {
		tsWideNoData, err := timeseries.NewWideFrame("A", timeseries.WideFrameVersionLatest)
		require.NoError(t, err)

		dt, err := reader.CanReadBasedOnMeta(tsWideNoData.Frames())
		require.NoError(t, err)
		require.Equal(t, data.KindTimeSeries, dt.Kind())

		nopeFrames := data.Frames{data.NewFrame("")}
		_, err = reader.CanReadBasedOnMeta(nopeFrames)
		require.Error(t, err)
	})

	t.Run("read something", func(t *testing.T) {
		n, err := numeric.NewWideFrame("A", numeric.WideFrameVersionLatest)
		require.NoError(t, err)

		err = n.AddMetric("cpu", data.Labels{"host": "king_sloth"}, 5.3)
		require.NoError(t, err)
		err = n.AddMetric("cpu", data.Labels{"host": "queen_sloth"}, 5.2)
		require.NoError(t, err)

		frames := n.Frames()

		dt, err := reader.CanReadBasedOnMeta(frames)
		require.NoError(t, err)

		if dt.Kind() == data.KindNumeric {
			nr, err := numeric.CollectionReaderFromFrames(frames)
			require.NoError(t, err)

			c, err := nr.GetCollection(false)
			require.NoError(t, err)
			require.NoError(t, c.Warning)

			require.Len(t, c.Refs, 2)
			for _, ref := range c.Refs {
				_ = ref
				val, empty, err := ref.NullableFloat64Value()
				require.NoError(t, err)
				require.Equal(t, false, empty)
				fmt.Printf("%v %v %v\n", ref.GetMetricName(), ref.GetLabels().String(), *val)
			}
		}
	})
}
