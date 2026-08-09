package gateway

import "context"

type authenticatedDeviceContextKey struct{}

// ContextWithDevice carries the authenticated identity and its revocation fence.
func ContextWithDevice(ctx context.Context, device DeviceSession) context.Context {
	return context.WithValue(ctx, authenticatedDeviceContextKey{}, device)
}

// DeviceFromContext returns the device authenticated by the transport.
func DeviceFromContext(ctx context.Context) (DeviceSession, bool) {
	device, ok := ctx.Value(authenticatedDeviceContextKey{}).(DeviceSession)
	return device, ok
}
