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
	initLogger()
	// Set gin to test mode
	gin.SetMode(gin.TestMode)

	// Create storage and mock client manager for testing
	storage := NewMemoryStorage()
	clientManager := NewMockClientManager()

	// Create admin handlers (proxy handlers set to nil for testing)
	handlers := NewAdminHandlers(storage, clientManager, nil)

	// Create router
	router := gin.New()
	admin := router.Group("/admin")
	{
		admin.POST("/mcps", handlers.AddMCPHandler)
		admin.PUT("/mcps/:name", handlers.UpdateMCPHandler)
		admin.DELETE("/mcps/:name", handlers.RemoveMCPHandler)
		admin.GET("/mcps", handlers.ListMCPsHandler)
		admin.GET("/mcps/:name", handlers.GetMCPHandler)
	}

	t.Run("Add MCP Definition", func(t *testing.T) {
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
		req, err := http.NewRequestWithContext(context.Background(), "DELETE", "/admin/mcps/test-mcp", http.NoBody)
		require.NoError(t, err)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("Add Duplicate MCP", func(t *testing.T) {
		// First add an MCP
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

		// Try to add the same MCP again
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
		invalidMCP := MCPDefinition{
			Name:      "", // Invalid: empty name
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
