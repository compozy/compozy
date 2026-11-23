package gateway

import "context"

func (s *DeviceService) revokeSession(
	ctx context.Context,
	deviceID string,
	revoke func(context.Context) (DeviceSession, bool, error),
) (RevokeResult, error) {
	var result RevokeResult
	canceled, err := s.mutations.Revoke(ctx, deviceID, s.connections, func(operationCtx context.Context) error {
		device, changed, err := revoke(operationCtx)
		if err == nil {
			result.Device, result.Changed = device, changed
		}
		return err
	})
	result.Canceled = canceled
	return result, err
}
