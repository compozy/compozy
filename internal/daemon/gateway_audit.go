package daemon

import "time"

const (
	maxGatewayAuditFieldBytes = 2048
	gatewayAuditWriteTimeout  = 5 * time.Second
)
