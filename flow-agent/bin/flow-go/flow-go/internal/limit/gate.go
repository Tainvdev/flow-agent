// Package limit provides adjustable admission gates for live service limits.
package limit

import (
	"context"
	"fmt"
	"sync"
)

// Gate limits concurrent operations. Changing the limit never revokes permits
// already held; lowering it takes effect as those permits are released.
type Gate struct {
	mu      sync.Mutex
	limit   int
	active  int
	changed chan struct{}
}

func New(limit int) (*Gate, error) {
	if limit < 1 {
		return nil, fmt.Errorf("limit: value must be positive")
	}
	return &Gate{limit: limit, changed: make(chan struct{})}, nil
}

func (g *Gate) signalLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}

func (g *Gate) SetLimit(limit int) error {
	if limit < 1 {
		return fmt.Errorf("limit: value must be positive")
	}
	g.mu.Lock()
	g.limit = limit
	g.signalLocked()
	g.mu.Unlock()
	return nil
}

func (g *Gate) Limit() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.limit
}

func (g *Gate) Active() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.active
}

func (g *Gate) TryAcquire() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.active >= g.limit {
		return false
	}
	g.active++
	return true
}

func (g *Gate) Acquire(ctx context.Context) (func(), error) {
	for {
		g.mu.Lock()
		if g.active < g.limit {
			g.active++
			g.mu.Unlock()
			var once sync.Once
			return func() { once.Do(g.Release) }, nil
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (g *Gate) Release() {
	g.mu.Lock()
	if g.active > 0 {
		g.active--
	}
	g.signalLocked()
	g.mu.Unlock()
}
