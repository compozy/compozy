package extensionpkg

import (
	"context"
	"sync"
)

const maxConcurrentViewCallsPerInstance = 16

type viewCallGateRegistry struct {
	mu    sync.Mutex
	gates map[InstanceKey]*viewCallGate
}

type viewCallGate struct {
	tokens chan struct{}
	refs   int
}

func (r *viewCallGateRegistry) acquire(ctx context.Context, key InstanceKey) (func(), error) {
	gate := r.retain(key.Normalize())
	select {
	case <-gate.tokens:
		var once sync.Once
		return func() {
			once.Do(func() {
				gate.tokens <- struct{}{}
				r.release(key.Normalize(), gate)
			})
		}, nil
	case <-ctx.Done():
		r.release(key.Normalize(), gate)
		return nil, ctx.Err()
	}
}

func (r *viewCallGateRegistry) retain(key InstanceKey) *viewCallGate {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.gates == nil {
		r.gates = make(map[InstanceKey]*viewCallGate)
	}
	gate := r.gates[key]
	if gate == nil {
		gate = &viewCallGate{tokens: make(chan struct{}, maxConcurrentViewCallsPerInstance)}
		for range maxConcurrentViewCallsPerInstance {
			gate.tokens <- struct{}{}
		}
		r.gates[key] = gate
	}
	gate.refs++
	return gate
}

func (r *viewCallGateRegistry) release(key InstanceKey, gate *viewCallGate) {
	r.mu.Lock()
	defer r.mu.Unlock()
	gate.refs--
	if gate.refs == 0 && r.gates[key] == gate {
		delete(r.gates, key)
	}
}
