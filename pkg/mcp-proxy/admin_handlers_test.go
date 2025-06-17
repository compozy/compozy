package mcpproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHandlers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	setupTest := func() (*gin.Engine, *MCPService) {
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

	t.Run("Add MCP Definition", func(t *testing.T) {
		router, _ := setupTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		jsonData, err := json.Marshal(mcpDef)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(context.Background(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
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

	t.Run("List MCP Definitions", func(t *testing.T) {
		router, service := setupTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(context.Background(), &mcpDef)
		req, err := http.NewRequestWithContext(context.Background(), "GET", "/admin/mcps", http.NoBody)
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

	t.Run("Get Specific MCP Definition", func(t *testing.T) {
		router, service := setupTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(context.Background(), &mcpDef)
		req, err := http.NewRequestWithContext(context.Background(), "GET", "/admin/mcps/test-mcp", http.NoBody)
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

	t.Run("Update MCP Definition", func(t *testing.T) {
		router, service := setupTest()
		originalMCP := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(context.Background(), &originalMCP)
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
			context.Background(),
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

	t.Run("Delete MCP Definition", func(t *testing.T) {
		router, service := setupTest()
		mcpDef := MCPDefinition{
			Name:        "test-mcp",
			Description: "Test MCP server",
			Transport:   TransportStdio,
			Command:     "node",
			Args:        []string{"test-server.js"},
		}
		_ = service.CreateMCP(context.Background(), &mcpDef)
		req, err := http.NewRequestWithContext(context.Background(), "DELETE", "/admin/mcps/test-mcp", http.NoBody)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Add Duplicate MCP", func(t *testing.T) {
		router, _ := setupTest()
		mcpDef := MCPDefinition{
			Name:      "duplicate-test",
			Transport: TransportStdio,
			Command:   "node",
			Args:      []string{"test.js"},
		}
		jsonData, err := json.Marshal(mcpDef)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(context.Background(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)
		req, err = http.NewRequestWithContext(context.Background(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
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

	t.Run("Invalid MCP Definition", func(t *testing.T) {
		router, _ := setupTest()
		invalidMCP := MCPDefinition{
			Name:      "",
			Transport: TransportStdio,
		}
		jsonData, err := json.Marshal(invalidMCP)
		require.NoError(t, err)
		req, err := http.NewRequestWithContext(context.Background(), "POST", "/admin/mcps", bytes.NewBuffer(jsonData))
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

	t.Run("Get Non-existent MCP", func(t *testing.T) {
		router, _ := setupTest()
		req, err := http.NewRequestWithContext(context.Background(), "GET", "/admin/mcps/non-existent", http.NoBody)
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
