package events

const (
	GatewayExposureChanged      = "gateway.exposure.changed"
	GatewayConsentRecorded      = "gateway.consent.recorded"
	GatewayDevicePaired         = "gateway.device.paired"
	GatewayDeviceRevoked        = "gateway.device.revoked"
	GatewayDeviceRemoved        = "gateway.device.removed"
	GatewayProviderStateChanged = "gateway.provider.state_changed"
	GatewayEndpointVerified     = "gateway.endpoint.verified"
	GatewayEndpointRejected     = "gateway.endpoint.rejected"
	GatewayIngressBound         = "gateway.ingress.bound"
	GatewayIngressUnbound       = "gateway.ingress.unbound"
	GatewayAuditRun             = "gateway.audit.run"
)
