package engine

import (
	"context"

	"github.com/kodelyx/flow-go/flow-go/internal/limit"
)

func acquireMediaLimit(ctx context.Context, gate chan struct{}) (func(), error) {
	if gate == nil {
		return func() {}, nil
	}
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func acquireConfiguredMediaLimit(ctx context.Context, adjustable *limit.Gate, legacy chan struct{}) (func(), error) {
	if adjustable != nil {
		return adjustable.Acquire(ctx)
	}
	return acquireMediaLimit(ctx, legacy)
}
