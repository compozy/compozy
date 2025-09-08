package routes

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
)

// Version returns the current API version string used in routing (e.g., "v0").
func Version() string {
	return core.GetVersion()
}

// Base returns the versioned API base path (e.g., "/api/v0").
func Base() string {
	return fmt.Sprintf("/api/%s", Version())
}

// Hooks returns the public webhooks base path (e.g., "/api/v0/hooks").
func Hooks() string {
	return Base() + "/hooks"
}

// HealthVersioned returns the versioned health path (e.g., "/api/v0/health").
// Note: The primary health endpoint is currently unversioned at "/health".
func HealthVersioned() string {
	return Base() + "/health"
}
