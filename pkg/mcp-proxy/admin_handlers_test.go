package mcpproxy

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/compozy/compozy/test/helpers/ginmode"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAdminHandlerTest creates a common test setup for admin handler tests
func setupAdminHandlerTest() (*gin.Engine, *MCPService) {
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()
	service := NewMCPService(storage, clientManager, nil)
	handlers := NewAdminHandlers(service)
	router := gin.New()
	admin := router.Group("/admin")
	{
		admin.POST("/mcps", handlers.AddMCPHandler)
		admin.PUT("/mcps/:name", handlers.UpdateMCPHandler)
		admin.DELETE("/mcps/:name", handlers.RemoveMCPHandler)
		admin.GET("/mcps", handlers.ListMCPsHandler)
		admin.GET("/mcps/:name", handlers.GetMCPHandler)
	}
	return router, service
}

func TestAdminHandlers_AddMCP(t *testing.T) {
	ginmode.EnsureGinTestMode()

	t.Run("Should create new MCP definition and return success response", func(t *testing.T) {
		router, _ := setupAdminHandlerTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		jsonData, err := json.Marshal(mcpDef)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "MCP definition added successfully", response["message"])
		assert.Equal(t, "test-mcp", response["name"])
	})
}

func TestAdminHandlers_ListMCPs(t *testing.T) {
	ginmode.EnsureGinTestMode()

	t.Run("Should return list of all MCP definitions with count", func(t *testing.T) {
		router, service := setupAdminHandlerTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(t.Context(), &mcpDef)
		req, err := http.NewRequestWithContext(t.Context(), "GET", "/admin/mcps", http.NoBody)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		mcps := response["mcps"].([]any)
		assert.Equal(t, 1, len(mcps))
		assert.Equal(t, float64(1), response["count"])
	})
}

func TestAdminHandlers_GetMCP(t *testing.T) {
	ginmode.EnsureGinTestMode()

	t.Run("Should return specific MCP definition by name", func(t *testing.T) {
		router, service := setupAdminHandlerTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(t.Context(), &mcpDef)
		req, err := http.NewRequestWithContext(t.Context(), "GET", "/admin/mcps/test-mcp", http.NoBody)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		definition := response["definition"].(map[string]any)
		assert.Equal(t, "test-mcp", definition["name"])
		assert.Equal(t, "Test MCP server", definition["description"])
	})

	t.Run("Should return 404 for non-existent MCP definition", func(t *testing.T) {
		router, _ := setupAdminHandlerTest()
		req, err := http.NewRequestWithContext(t.Context(), "GET", "/admin/mcps/non-existent", http.NoBody)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "MCP not found", response["error"])
	})
}

func TestAdminHandlers_UpdateMCP(t *testing.T) {
	ginmode.EnsureGinTestMode()

	t.Run("Should update existing MCP definition successfully", func(t *testing.T) {
		router, service := setupAdminHandlerTest()
		originalMCP := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(t.Context(), &originalMCP)
		updatedMCP := MCPDefinition{
			Name:        "test-mcp",
			Description: "Updated test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"updated-server.js"},
		}
		jsonData, err := json.Marshal(updatedMCP)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(
			t.Context(),
			"PUT",
			"/admin/mcps/test-mcp",
			bytes.NewBuffer(jsonData),
		)
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "MCP definition updated successfully", response["message"])
	})
}

func TestAdminHandlers_DeleteMCP(t *testing.T) {
	ginmode.EnsureGinTestMode()

	t.Run("Should delete existing MCP definition successfully", func(t *testing.T) {
		router, service := setupAdminHandlerTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(t.Context(), &mcpDef)
		req, err := http.NewRequestWithContext(t.Context(), "DELETE", "/admin/mcps/test-mcp", http.NoBody)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestAdminHandlers_ErrorCases(t *testing.T) {
	ginmode.EnsureGinTestMode()

	t.Run("Should reject duplicate MCP definition with conflict error", func(t *testing.T) {
		router, _ := setupAdminHandlerTest()
		mcpDef := MCPDefinition{
			Name:      "duplicate-test",
			Transport: TransportStdio,
			Command:   "node",
			Args:      []string{"test.js"},
		}
		jsonData, err := json.Marshal(mcpDef)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		req, err = http.NewRequestWithContext(t.Context(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "MCP already exists", response["error"])
	})

	t.Run("Should reject invalid MCP definition with validation error", func(t *testing.T) {
		router, _ := setupAdminHandlerTest()
		invalidMCP := MCPDefinition{
			Name:      "",
			Transport: TransportStdio,
		}
		jsonData, err := json.Marshal(invalidMCP)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(t.Context(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response map[string]any
		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "Invalid request", response["error"])
	})
}
