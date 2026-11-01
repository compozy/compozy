package acp

import "time"

// PermissionWaitResolver resolves the current wait using the daemon session identity.
type PermissionWaitResolver func(sessionID string) time.Duration

func WithPermissionWaitResolver(resolver PermissionWaitResolver) Option {
	return func(driver *Driver) {
		driver.permissionWaitResolver = resolver
	}
}
