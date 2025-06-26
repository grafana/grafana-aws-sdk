package api

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeDS struct {
	statusCounter int
	status        []*ExecuteQueryStatus
	statusErr     error
	stopped       bool
}

func (s *fakeDS) Execute(context.Context, *ExecuteQueryInput) (*ExecuteQueryOutput, error) {
	return &ExecuteQueryOutput{}, nil
}

func (s *fakeDS) Stop(*ExecuteQueryOutput) error {
	s.stopped = true
	return nil
}

func (s *fakeDS) Status(context.Context, *ExecuteQueryOutput) (*ExecuteQueryStatus, error) {
	i := s.statusCounter
	s.statusCounter++
	return s.status[i], s.statusErr
}

func TestWaitOnQuery(t *testing.T) {
	// for tests we override backoff instance to always take 1 millisecond so the tests run quickly
	backoffMin = 1 * time.Millisecond
	backoffMax = 1 * time.Millisecond

	tests := []struct {
		description string
		ds          *fakeDS
		expectedErr error
	}{
		{
			"returns with no error",
			&fakeDS{
				statusCounter: 0,
				status:        []*ExecuteQueryStatus{{Finished: true}},
				statusErr:     nil,
			},
			nil,
		},
		{
			"returns with no error after several calls",
			&fakeDS{
				statusCounter: 0,
				status:        []*ExecuteQueryStatus{{Finished: false}, {Finished: true}},
				statusErr:     nil,
			},
			nil,
		},
		{
			"returns an error",
			&fakeDS{
				statusCounter: 0,
				status:        []*ExecuteQueryStatus{{}},
				statusErr:     ErrorStatus,
			},
			nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			err := WaitOnQuery(context.Background(), tc.ds, &ExecuteQueryOutput{})
			if tc.ds.statusCounter != len(tc.ds.status) {
				t.Errorf("status not called the right amount of times. Want %d got %d", len(tc.ds.status), tc.ds.statusCounter)
			}
			if (err != nil || tc.ds.statusErr != nil) && !errors.Is(err, tc.ds.statusErr) {
				t.Errorf("unexpected error %v", err)
			}
		})
	}
}

func TestConnection_waitOnQueryCancelled(t *testing.T) {
	// add a big timeout to have time to cancel
	backoffMin = 10000 * time.Millisecond
	backoffMax = 10000 * time.Millisecond

	ds := &fakeDS{
		statusCounter: 0,
		status:        []*ExecuteQueryStatus{{Finished: false}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan bool)

	// start the execution in parallel
	go func() {
		err := WaitOnQuery(ctx, ds, &ExecuteQueryOutput{})
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Errorf("unexpected error %v", err)
		}
		done <- true
	}()
	cancel()
	<-done

	if !ds.stopped {
		t.Errorf("failed to cancel the request")
	}
}
