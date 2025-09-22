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

func buildResourceRoute(resource string) string {
	return Base() + "/" + resource
}

func Project() string  { return buildResourceRoute("project") }
func Schemas() string  { return buildResourceRoute("schemas") }
func Models() string   { return buildResourceRoute("models") }
func Memories() string { return buildResourceRoute("memories") }
func Agents() string   { return buildResourceRoute("agents") }
func Tools() string    { return buildResourceRoute("tools") }
func Tasks() string    { return buildResourceRoute("tasks") }
func Mcps() string     { return buildResourceRoute("mcps") }

// HealthVersioned returns the versioned health path (e.g., "/api/v0/health").
// The primary health endpoint is versioned and mounted under the API base path.
func HealthVersioned() string {
	return Base() + "/health"
}
