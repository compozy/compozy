package gateway

import (
	"context"
	"fmt"
)

type selfRevocationContextKey struct{}

func (s *DeviceService) revokeSession(
	ctx context.Context,
	deviceID string,
	revoke func(context.Context) (DeviceSession, bool, error),
) (RevokeResult, error) {
	operationCtx := ctx
	if releaseMutationForDevice(ctx, deviceID) {
		operationCtx = context.WithValue(context.WithoutCancel(ctx), selfRevocationContextKey{}, deviceID)
	}
	finishRevocation, err := s.mutations.beginRevocation(operationCtx, deviceID)
	if err != nil {
		return RevokeResult{}, err
	}
	revocationCommitted := false
	defer func() { finishRevocation(revocationCommitted) }()
	device, changed, err := revoke(operationCtx)
	if err != nil {
		return RevokeResult{}, fmt.Errorf("gateway: revoke credential: %w", err)
	}
	revocationCommitted = true
	terminationCtx, cancelTermination := context.WithTimeout(
		context.WithoutCancel(operationCtx), defaultConnectionTerminationTimeout,
	)
	defer cancelTermination()
	canceled, cancelErr := s.connections.CancelDevice(terminationCtx, deviceID)
	result := RevokeResult{Device: device, Changed: changed, Canceled: canceled}
	if cancelErr != nil {
		return result, fmt.Errorf("gateway: terminate revoked credential connections: %w", cancelErr)
	}
	return result, nil
}
