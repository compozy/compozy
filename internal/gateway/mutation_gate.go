package gateway

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type mutationGate struct {
	mu      sync.Mutex
	next    uint64
	devices map[string]*deviceMutationState
}

type deviceMutationState struct {
	active      map[uint64]context.CancelFunc
	drained     chan struct{}
	revocations int
	revoked     bool
}

type mutationLease struct {
	once     sync.Once
	gate     *mutationGate
	deviceID string
	handle   uint64
}

type mutationLeaseContextKey struct{}

func newMutationGate() *mutationGate {
	return &mutationGate{devices: make(map[string]*deviceMutationState)}
}

func (g *mutationGate) acquire(ctx context.Context, deviceID string) (context.Context, error) {
	if g == nil || ctx == nil || deviceID == "" {
		return nil, errors.New("gateway: mutation gate dependencies are required")
	}
	g.mu.Lock()
	state := g.deviceState(deviceID)
	if state.revoked || state.revocations > 0 {
		g.mu.Unlock()
		return nil, ErrDeviceRevoked
	}
	if len(state.active) == 0 {
		state.drained = make(chan struct{})
	}
	g.next++
	handle := g.next
	// #nosec G118 -- the lease owns cancel and invokes it on release or revocation.
	mutationCtx, cancel := context.WithCancel(ctx)
	state.active[handle] = cancel
	g.mu.Unlock()

	lease := &mutationLease{gate: g, deviceID: deviceID, handle: handle}
	return context.WithValue(mutationCtx, mutationLeaseContextKey{}, lease), nil
}

func (g *mutationGate) validate(deviceID string) error {
	if g == nil || deviceID == "" {
		return ErrDeviceRevoked
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.devices[deviceID]
	if state != nil && (state.revoked || state.revocations > 0) {
		return ErrDeviceRevoked
	}
	return nil
}

func (g *mutationGate) beginRevocation(ctx context.Context, deviceID string) (func(bool), error) {
	if g == nil || ctx == nil || deviceID == "" {
		return nil, errors.New("gateway: revocation gate dependencies are required")
	}
	g.mu.Lock()
	state := g.deviceState(deviceID)
	state.revocations++
	cancellations := make([]context.CancelFunc, 0, len(state.active))
	for _, cancel := range state.active {
		cancellations = append(cancellations, cancel)
	}
	drained := state.drained
	g.mu.Unlock()
	for _, cancel := range cancellations {
		cancel()
	}

	if drained != nil {
		select {
		case <-drained:
		case <-ctx.Done():
			g.finishRevocation(deviceID, false)
			return nil, fmt.Errorf("gateway: wait for in-flight device mutations: %w", ctx.Err())
		}
	}
	var once sync.Once
	return func(committed bool) {
		once.Do(func() { g.finishRevocation(deviceID, committed) })
	}, nil
}

func (g *mutationGate) finishRevocation(deviceID string, committed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.devices[deviceID]
	if state == nil {
		return
	}
	if committed {
		state.revoked = true
	}
	if state.revocations > 0 {
		state.revocations--
	}
	if !state.revoked && state.revocations == 0 && len(state.active) == 0 {
		delete(g.devices, deviceID)
	}
}

func (g *mutationGate) release(deviceID string, handle uint64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	state := g.devices[deviceID]
	if state == nil {
		return
	}
	cancel, ok := state.active[handle]
	if !ok {
		return
	}
	delete(state.active, handle)
	cancel()
	if len(state.active) == 0 && state.drained != nil {
		close(state.drained)
		state.drained = nil
	}
	if !state.revoked && state.revocations == 0 && len(state.active) == 0 {
		delete(g.devices, deviceID)
	}
}

func (g *mutationGate) deviceState(deviceID string) *deviceMutationState {
	state := g.devices[deviceID]
	if state == nil {
		state = &deviceMutationState{active: make(map[uint64]context.CancelFunc)}
		g.devices[deviceID] = state
	}
	return state
}

// ReleaseMutation releases the authenticated mutation lease carried by ctx.
func ReleaseMutation(ctx context.Context) {
	if ctx == nil {
		return
	}
	lease, ok := ctx.Value(mutationLeaseContextKey{}).(*mutationLease)
	if !ok || lease == nil {
		return
	}
	lease.once.Do(func() { lease.gate.release(lease.deviceID, lease.handle) })
}

func releaseMutationForDevice(ctx context.Context, deviceID string) bool {
	if ctx == nil {
		return false
	}
	lease, ok := ctx.Value(mutationLeaseContextKey{}).(*mutationLease)
	if ok && lease != nil && lease.deviceID == deviceID {
		ReleaseMutation(ctx)
		return true
	}
	return false
}
