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

// Auth returns the authentication base path (e.g., "/api/v0/auth").
func Auth() string {
	return Base() + "/auth"
}

// Users returns the users base path (e.g., "/api/v0/users").
func Users() string {
	return Base() + "/users"
}

// Executions returns the executions base path (e.g., "/api/v0/executions").
func Executions() string {
	return Base() + "/executions"
}

// Workflows returns the workflows base path (e.g., "/api/v0/workflows").
func Workflows() string {
	return Base() + "/workflows"
}

// HealthVersioned returns the versioned health path (e.g., "/api/v0/health").
// Note: The primary health endpoint is currently unversioned at "/health".
func HealthVersioned() string {
	return Base() + "/health"
}
