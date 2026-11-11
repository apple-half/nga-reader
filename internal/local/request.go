package local

import (
	"context"
	"sync"
)

type RequestGate struct {
	mu         sync.Mutex
	generation uint64
	cancel     context.CancelFunc
	closed     bool
}

func (g *RequestGate) Begin(parent context.Context) (context.Context, uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.cancel != nil {
		g.cancel()
	}
	g.generation++
	ctx, cancel := context.WithCancel(parent)
	g.cancel = cancel
	if g.closed {
		cancel()
	}
	return ctx, g.generation
}
func (g *RequestGate) Accept(generation uint64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return !g.closed && generation == g.generation
}
func (g *RequestGate) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
	g.generation++
	if g.cancel != nil {
		g.cancel()
	}
}
