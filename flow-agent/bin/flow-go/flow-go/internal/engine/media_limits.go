package engine

import "context"

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
