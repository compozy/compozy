package gateway

import (
	"context"
	"fmt"
	"maps"
	"sync"
)

type liveConnection struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

// LiveConnectionRegistry indexes live stream cancellation by durable device ID.
type LiveConnectionRegistry struct {
	mu      sync.Mutex
	next    uint64
	devices map[string]map[uint64]liveConnection
}

func NewConnectionRegistry() *LiveConnectionRegistry {
	return &LiveConnectionRegistry{devices: make(map[string]map[uint64]liveConnection)}
}

func (r *LiveConnectionRegistry) Register(deviceID string, cancel context.CancelFunc) uint64 {
	return r.register(deviceID, cancel, nil)
}

// RegisterWaitable adds a completion signal so revocation can wait for stream teardown.
func (r *LiveConnectionRegistry) RegisterWaitable(
	deviceID string,
	cancel context.CancelFunc,
	done <-chan struct{},
) uint64 {
	return r.register(deviceID, cancel, done)
}

func (r *LiveConnectionRegistry) register(
	deviceID string,
	cancel context.CancelFunc,
	done <-chan struct{},
) uint64 {
	if cancel == nil {
		cancel = func() {}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	if r.devices[deviceID] == nil {
		r.devices[deviceID] = make(map[uint64]liveConnection)
	}
	r.devices[deviceID][r.next] = liveConnection{cancel: cancel, done: done}
	return r.next
}

func (r *LiveConnectionRegistry) Deregister(deviceID string, handle uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	connections := r.devices[deviceID]
	delete(connections, handle)
	if len(connections) == 0 {
		delete(r.devices, deviceID)
	}
}

func (r *LiveConnectionRegistry) CancelDevice(ctx context.Context, deviceID string) (int, error) {
	if ctx == nil {
		return 0, fmt.Errorf("gateway: cancel device connections: context is required")
	}
	r.mu.Lock()
	registered := r.devices[deviceID]
	connections := make(map[uint64]liveConnection, len(registered))
	maps.Copy(connections, registered)
	delete(r.devices, deviceID)
	r.mu.Unlock()
	count := len(connections)

	for _, connection := range connections {
		connection.cancel()
	}
	for _, connection := range connections {
		if connection.done != nil {
			select {
			case <-connection.done:
			case <-ctx.Done():
				return count, fmt.Errorf("gateway: wait for device connection teardown: %w", ctx.Err())
			}
		}
	}
	return count, nil
}

var _ ConnectionRegistry = (*LiveConnectionRegistry)(nil)
