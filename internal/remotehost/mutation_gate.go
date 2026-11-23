package remotehost

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// MutationGate drains admitted writes before credential revocation commits.
type MutationGate struct {
	validateEpoch func(context.Context, string, uint64) error
	mu            sync.Mutex
	next          uint64
	devices       map[string]*deviceMutationState
}

type deviceMutationState struct {
	active      map[uint64]context.CancelFunc
	drained     chan struct{}
	revocations int
	revoked     bool
}

type mutationLease struct {
	once     sync.Once
	gate     *MutationGate
	deviceID string
	handle   uint64
}

type mutationLeaseContextKey struct{}

func NewMutationGate(validateEpoch func(context.Context, string, uint64) error) *MutationGate {
	return &MutationGate{validateEpoch: validateEpoch, devices: make(map[string]*deviceMutationState)}
}

func (g *MutationGate) RevalidateForCommit(ctx context.Context, deviceID string, epoch uint64) error {
	// A self-revocation has surrendered its valid lease and sealed this identity before its final write.
	if revokingID, ok := ctx.Value(selfRevocationContextKey{}).(string); ok && revokingID == deviceID {
		return nil
	}
	if err := g.validate(deviceID); err != nil {
		return fmt.Errorf("gateway: validate device mutation gate: %w", err)
	}
	if err := g.validateEpoch(ctx, deviceID, epoch); err != nil {
		return fmt.Errorf("gateway: revalidate device epoch: %w", err)
	}
	return nil
}

// AcquireMutation admits one remote mutation and carries its revocation-aware lease in the returned context.
func (g *MutationGate) AcquireMutation(
	ctx context.Context,
	deviceID string,
	epoch uint64,
) (context.Context, error) {
	mutationCtx, err := g.acquire(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	if err := g.validateEpoch(mutationCtx, deviceID, epoch); err != nil {
		ReleaseMutation(mutationCtx)
		return nil, fmt.Errorf("gateway: acquire device mutation: %w", err)
	}
	return mutationCtx, nil
}

func (g *MutationGate) acquire(ctx context.Context, deviceID string) (context.Context, error) {
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

func (g *MutationGate) validate(deviceID string) error {
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

func (g *MutationGate) beginRevocation(ctx context.Context, deviceID string) (func(bool), error) {
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

func (g *MutationGate) finishRevocation(deviceID string, committed bool) {
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
	if state.revocations == 0 && len(state.active) == 0 {
		delete(g.devices, deviceID)
	}
}

func (g *MutationGate) release(deviceID string, handle uint64) {
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
	if state.revocations == 0 && len(state.active) == 0 {
		delete(g.devices, deviceID)
	}
}

func (g *MutationGate) deviceState(deviceID string) *deviceMutationState {
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
