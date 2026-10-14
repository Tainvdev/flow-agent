package engine

import (
	"context"
	"testing"
	"time"
)

func TestMediaLimitBoundsConcurrentWorkAndHonorsCancellation(t *testing.T) {
	gate := make(chan struct{}, 1)
	releaseFirst, err := acquireMediaLimit(context.Background(), gate)
	if err != nil {
		t.Fatal(err)
	}

	secondAcquired := make(chan struct{})
	go func() {
		release, acquireErr := acquireMediaLimit(context.Background(), gate)
		if acquireErr == nil {
			close(secondAcquired)
			release()
		}
	}()
	select {
	case <-secondAcquired:
		t.Fatal("second media operation exceeded the configured limit")
	case <-time.After(30 * time.Millisecond):
	}
	releaseFirst()
	select {
	case <-secondAcquired:
	case <-time.After(time.Second):
		t.Fatal("second media operation did not acquire the released slot")
	}

	gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := acquireMediaLimit(ctx, gate); err == nil {
		t.Fatal("cancelled media operation acquired a full gate")
	}
	<-gate
}
