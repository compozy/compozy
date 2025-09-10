package routes

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersion(t *testing.T) {
	t.Run("Should return the API version", func(t *testing.T) {
		version := Version()
		assert.NotEmpty(t, version, "Version should not be empty")
		assert.Contains(t, version, "v", "Version should contain 'v' prefix")
	})
}

func TestBase(t *testing.T) {
	t.Run("Should return versioned API base path", func(t *testing.T) {
		base := Base()
		expected := "/api/" + Version()
		assert.Equal(t, expected, base, "Base should be composed of '/api/' + Version()")
		assert.Contains(t, base, "/api/v", "Base should contain '/api/v' prefix")
	})
}

func TestHooks(t *testing.T) {
	t.Run("Should return hooks base path", func(t *testing.T) {
		hooks := Hooks()
		expected := Base() + "/hooks"
		assert.Equal(t, expected, hooks, "Hooks should be composed of Base() + '/hooks'")
		assert.Contains(t, hooks, "/hooks", "Hooks path should contain '/hooks'")
	})
}

func TestAuth(t *testing.T) {
	t.Run("Should return auth base path", func(t *testing.T) {
		auth := Auth()
		expected := Base() + "/auth"
		assert.Equal(t, expected, auth, "Auth should be composed of Base() + '/auth'")
		assert.Contains(t, auth, "/auth", "Auth path should contain '/auth'")
	})
}

func TestUsers(t *testing.T) {
	t.Run("Should return users base path", func(t *testing.T) {
		users := Users()
		expected := Base() + "/users"
		assert.Equal(t, expected, users, "Users should be composed of Base() + '/users'")
		assert.Contains(t, users, "/users", "Users path should contain '/users'")
	})
}

func TestExecutions(t *testing.T) {
	t.Run("Should return executions base path", func(t *testing.T) {
		executions := Executions()
		expected := Base() + "/executions"
		assert.Equal(t, expected, executions, "Executions should be composed of Base() + '/executions'")
		assert.Contains(t, executions, "/executions", "Executions path should contain '/executions'")
	})
}

func TestWorkflows(t *testing.T) {
	t.Run("Should return workflows base path", func(t *testing.T) {
		workflows := Workflows()
		expected := Base() + "/workflows"
		assert.Equal(t, expected, workflows, "Workflows should be composed of Base() + '/workflows'")
		assert.Contains(t, workflows, "/workflows", "Workflows path should contain '/workflows'")
	})
}

func TestHealthVersioned(t *testing.T) {
	t.Run("Should return versioned health path", func(t *testing.T) {
		health := HealthVersioned()
		expected := Base() + "/health"
		assert.Equal(t, expected, health, "HealthVersioned should be composed of Base() + '/health'")
		assert.Contains(t, health, "/health", "HealthVersioned path should contain '/health'")
	})
}

func TestPathCompositionConsistency(t *testing.T) {
	t.Run("Should ensure all paths are consistently composed from Base()", func(t *testing.T) {
		base := Base()
		version := Version()

		// Verify Base() composition
		assert.Equal(t, "/api/"+version, base)

		// Verify all other paths build on Base()
		assert.Equal(t, base+"/hooks", Hooks())
		assert.Equal(t, base+"/auth", Auth())
		assert.Equal(t, base+"/users", Users())
		assert.Equal(t, base+"/executions", Executions())
		assert.Equal(t, base+"/workflows", Workflows())
		assert.Equal(t, base+"/health", HealthVersioned())
	})
}

func TestPathFormatConsistency(t *testing.T) {
	t.Run("Should ensure all paths follow consistent format", func(t *testing.T) {
		// All paths should start with "/api/v"
		assert.Contains(t, Base(), "/api/v")
		assert.Contains(t, Hooks(), "/api/v")
		assert.Contains(t, Auth(), "/api/v")
		assert.Contains(t, Users(), "/api/v")
		assert.Contains(t, Executions(), "/api/v")
		assert.Contains(t, Workflows(), "/api/v")
		assert.Contains(t, HealthVersioned(), "/api/v")

		// None should have double slashes (except the leading one)
		paths := []string{Base(), Hooks(), Auth(), Users(), Executions(), Workflows(), HealthVersioned()}
		for _, path := range paths {
			assert.NotContains(t, path, "//", "Path %s should not contain double slashes", path)
		}
	})
}
