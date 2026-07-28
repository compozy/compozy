package extensionpkg

import (
	"context"
	"sync"
)

const maxConcurrentBridgeControls = 4

var (
	bridgeControlGlobalSlots   = make(chan struct{}, maxConcurrentBridgeControls)
	bridgeControlInstanceGates bridgeControlGateRegistry
)

func acquireBridgeControlGlobal(ctx context.Context) (func(), error) {
	select {
	case bridgeControlGlobalSlots <- struct{}{}:
		return func() { <-bridgeControlGlobalSlots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type bridgeControlGateRegistry struct {
	mu    sync.Mutex
	gates map[string]*bridgeControlGate
}

type bridgeControlGate struct {
	token chan struct{}
	refs  int
}

func (r *bridgeControlGateRegistry) acquire(ctx context.Context, key string) (func(), error) {
	gate := r.retain(key)
	select {
	case <-gate.token:
		return func() {
			gate.token <- struct{}{}
			r.release(key, gate)
		}, nil
	case <-ctx.Done():
		r.release(key, gate)
		return nil, ctx.Err()
	}
}

func (r *bridgeControlGateRegistry) retain(key string) *bridgeControlGate {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gates == nil {
		r.gates = make(map[string]*bridgeControlGate)
	}
	gate := r.gates[key]
	if gate == nil {
		gate = &bridgeControlGate{token: make(chan struct{}, 1)}
		gate.token <- struct{}{}
		r.gates[key] = gate
	}
	gate.refs++
	return gate
}

func (r *bridgeControlGateRegistry) release(key string, gate *bridgeControlGate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gate.refs--
	if gate.refs == 0 && r.gates[key] == gate {
		delete(r.gates, key)
	}
}
