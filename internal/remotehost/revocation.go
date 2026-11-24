package remotehost

import (
	"context"
	"fmt"
	"time"
)

const connectionTerminationTimeout = 5 * time.Second

type selfRevocationContextKey struct{}

// PrepareRevocation releases a caller's own lease so a self-revocation can drain and commit.
func PrepareRevocation(ctx context.Context, deviceID string) (context.Context, bool) {
	if releaseMutationForDevice(ctx, deviceID) {
		return context.WithValue(context.WithoutCancel(ctx), selfRevocationContextKey{}, deviceID), true
	}
	return ctx, false
}

// Revoke seals mutation admission until the durable change and stream teardown finish.
func (g *MutationGate) Revoke(
	ctx context.Context,
	deviceID string,
	connections ConnectionRegistry,
	revoke func(context.Context) error,
) (int, error) {
	operationCtx, _ := PrepareRevocation(ctx, deviceID)
	finishRevocation, err := g.beginRevocation(operationCtx, deviceID)
	if err != nil {
		return 0, err
	}
	revocationCommitted := false
	defer func() { finishRevocation(revocationCommitted) }()
	if err := revoke(operationCtx); err != nil {
		return 0, fmt.Errorf("remotehost: revoke credential: %w", err)
	}
	revocationCommitted = true
	terminationCtx, cancelTermination := context.WithTimeout(
		context.WithoutCancel(operationCtx), connectionTerminationTimeout,
	)
	defer cancelTermination()
	canceled, err := connections.CancelDevice(terminationCtx, deviceID)
	if err != nil {
		return canceled, fmt.Errorf("remotehost: terminate revoked credential connections: %w", err)
	}
	return canceled, nil
}
