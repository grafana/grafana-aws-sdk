package jsoniter_test

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/grafana/grafana-aws-sdk-frankenstein/pkg/backend"
	sdkjsoniter "github.com/grafana/grafana-aws-sdk-frankenstein/pkg/data/utils/jsoniter"
	j "github.com/json-iterator/go"
	"github.com/stretchr/testify/require"
)

func TestNewIterator(t *testing.T) {
	iter := j.NewIterator(j.ConfigDefault)
	jiter := sdkjsoniter.NewIterator(iter)
	require.NotNil(t, jiter)
}

func TestRead(t *testing.T) {
	t.Run("should be able read the error", func(t *testing.T) {
		jiter := sdkjsoniter.NewIterator(j.NewIterator(j.ConfigDefault))
		read, err := jiter.Read()
		require.Error(t, err)
		require.Nil(t, read)
	})

	t.Run("should be able read the json data", func(t *testing.T) {
		iter := j.Parse(sdkjsoniter.ConfigDefault, io.NopCloser(strings.NewReader(`{"test":123}`)), 128)
		jiter := sdkjsoniter.NewIterator(iter)
		read, err := jiter.Read()
		require.NoError(t, err)
		require.NotNil(t, read)
		r := read.(map[string]interface{})
		require.Equal(t, r["test"], float64(123))
	})
}

func TestParse(t *testing.T) {
	t.Run("should create a new iterator without any error", func(t *testing.T) {
		iter, err := sdkjsoniter.Parse(sdkjsoniter.ConfigDefault, io.NopCloser(strings.NewReader(`{"test":123}`)), 128)
		require.NoError(t, err)
		require.NotNil(t, iter)
	})
}

func TestMarshalUnmarshal(t *testing.T) {
	qdr := &backend.QueryDataResponse{
		Responses: backend.Responses{
			"A": {
				Status: 400,
				Error:  errors.New("test"),
			},
		},
	}
	resp, err := json.Marshal(qdr)
	require.NoError(t, err)
	qdr2 := &backend.QueryDataResponse{}
	require.NoError(t, json.Unmarshal(resp, qdr2))
	require.Equal(t, qdr, qdr2)
}
