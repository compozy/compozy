package mcpproxy

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

// ClientManager defines the interface for managing MCP clients
type ClientManager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	AddClient(ctx context.Context, def *MCPDefinition) error
	RemoveClient(ctx context.Context, name string) error
	GetClientStatus(name string) (*MCPStatus, error)
	GetClient(name string) (*MCPClient, error)
	GetMetrics() map[string]any
}

// AdminHandlers provides HTTP handlers for MCP management operations
type AdminHandlers struct {
	storage       Storage
	clientManager ClientManager
	proxyHandlers *ProxyHandlers
}

// NewAdminHandlers creates a new AdminHandlers instance
func NewAdminHandlers(storage Storage, clientManager ClientManager, proxyHandlers *ProxyHandlers) *AdminHandlers {
	return &AdminHandlers{
		storage:       storage,
		clientManager: clientManager,
		proxyHandlers: proxyHandlers,
	}
}

// AddMCPHandler handles POST /admin/mcps - adds a new MCP definition
func (h *AdminHandlers) AddMCPHandler(c *gin.Context) {
	var mcpDef MCPDefinition
	if err := c.ShouldBindJSON(&mcpDef); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
		return
	}

	// Validate the MCP definition
	if err := mcpDef.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid MCP definition",
			"details": err.Error(),
		})
		return
	}

	// Check if MCP with same name already exists
	existing, err := h.storage.LoadMCP(context.Background(), mcpDef.Name)
	if err == nil && existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "MCP with this name already exists",
			"name":  mcpDef.Name,
		})
		return
	}

	// Set timestamps
	now := time.Now()
	mcpDef.CreatedAt = now
	mcpDef.UpdatedAt = now

	// Save MCP definition to storage
	if err := h.storage.SaveMCP(context.Background(), &mcpDef); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save MCP definition",
			"details": err.Error(),
		})
		return
	}

	// Add MCP client asynchronously
	go func() {
		if err := h.clientManager.AddClient(context.Background(), &mcpDef); err != nil {
			// Update status to reflect connection error
			status := &MCPStatus{
				Name:      mcpDef.Name,
				Status:    StatusError,
				LastError: err.Error(),
			}
			if saveErr := h.storage.SaveStatus(context.Background(), status); saveErr != nil {
				logger.Error("Failed to save error status", "name", mcpDef.Name, "error", saveErr)
			}
		} else if h.proxyHandlers != nil {
			// Register the MCP as a proxy endpoint
			if err := h.proxyHandlers.RegisterMCPProxy(context.Background(), mcpDef.Name, &mcpDef); err != nil {
				// Log error but don't fail the operation
				// The client is connected but proxy registration failed
				status := &MCPStatus{
					Name:      mcpDef.Name,
					Status:    StatusError,
					LastError: fmt.Sprintf("proxy registration failed: %v", err),
				}
				if saveErr := h.storage.SaveStatus(context.Background(), status); saveErr != nil {
					logger.Error("Failed to save proxy registration error status", "name", mcpDef.Name, "error", saveErr)
				}
			}
		}
	}()

	c.JSON(http.StatusCreated, gin.H{
		"message": "MCP definition added successfully",
		"name":    mcpDef.Name,
	})
}

// validateUpdateRequest validates the request parameters and MCP definition for update
func (h *AdminHandlers) validateUpdateRequest(c *gin.Context) (*MCPDefinition, *MCPDefinition, bool) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP name is required",
		})
		return nil, nil, false
	}

	var mcpDef MCPDefinition
	if err := c.ShouldBindJSON(&mcpDef); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
		return nil, nil, false
	}

	// Ensure the name in the URL matches the name in the payload
	mcpDef.Name = name

	// Validate the updated MCP definition
	if err := mcpDef.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid MCP definition",
			"details": err.Error(),
		})
		return nil, nil, false
	}

	// Check if MCP exists
	existing, err := h.storage.LoadMCP(context.Background(), name)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "MCP not found",
			"name":  name,
		})
		return nil, nil, false
	}

	return &mcpDef, existing, true
}

// performHotReload removes old client and adds updated client asynchronously
func (h *AdminHandlers) performHotReload(name string, mcpDef *MCPDefinition) {
	// Remove old client and unregister proxy
	if err := h.clientManager.RemoveClient(context.Background(), name); err != nil {
		logger.Error("Failed to remove client during update", "name", name, "error", err)
	}
	if h.proxyHandlers != nil {
		if err := h.proxyHandlers.UnregisterMCPProxy(name); err != nil {
			logger.Error("Failed to unregister proxy during update", "name", name, "error", err)
		}
	}

	// Add updated MCP client asynchronously (hot reload)
	go func() {
		if err := h.clientManager.AddClient(context.Background(), mcpDef); err != nil {
			// Update status to reflect connection error
			status := &MCPStatus{
				Name:      mcpDef.Name,
				Status:    StatusError,
				LastError: err.Error(),
			}
			if saveErr := h.storage.SaveStatus(context.Background(), status); saveErr != nil {
				logger.Error("Failed to save error status during update", "name", mcpDef.Name, "error", saveErr)
			}
		} else if h.proxyHandlers != nil {
			// Register the updated MCP as a proxy endpoint
			if err := h.proxyHandlers.RegisterMCPProxy(context.Background(), mcpDef.Name, mcpDef); err != nil {
				status := &MCPStatus{
					Name:      mcpDef.Name,
					Status:    StatusError,
					LastError: fmt.Sprintf("proxy registration failed: %v", err),
				}
				if saveErr := h.storage.SaveStatus(context.Background(), status); saveErr != nil {
					logger.Error("Failed to save proxy registration error status during update",
						"name", mcpDef.Name, "error", saveErr)
				}
			}
		}
	}()
}

// UpdateMCPHandler handles PUT /admin/mcps/{name} - updates an existing MCP definition
func (h *AdminHandlers) UpdateMCPHandler(c *gin.Context) {
	mcpDef, existing, valid := h.validateUpdateRequest(c)
	if !valid {
		return
	}

	// Set updated timestamp before any operations
	mcpDef.UpdatedAt = time.Now()
	mcpDef.CreatedAt = existing.CreatedAt // Keep original creation time

	// Save updated MCP definition FIRST to ensure consistency
	if err := h.storage.SaveMCP(context.Background(), mcpDef); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to update MCP definition",
			"details": err.Error(),
		})
		return
	}

	// Now perform the hot reload operations atomically
	h.performHotReload(mcpDef.Name, mcpDef)

	c.JSON(http.StatusOK, gin.H{
		"message": "MCP definition updated successfully",
		"name":    mcpDef.Name,
	})
}

// RemoveMCPHandler handles DELETE /admin/mcps/{name} - removes an MCP definition
func (h *AdminHandlers) RemoveMCPHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP name is required",
		})
		return
	}

	// Check if MCP exists
	existing, err := h.storage.LoadMCP(context.Background(), name)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "MCP not found",
			"name":  name,
		})
		return
	}

	// Remove from storage FIRST to ensure consistency
	if err := h.storage.DeleteMCP(context.Background(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to delete MCP definition",
			"details": err.Error(),
		})
		return
	}

	// Now remove runtime components
	if err := h.clientManager.RemoveClient(context.Background(), name); err != nil {
		logger.Error("Failed to remove client during deletion", "name", name, "error", err)
	}
	if h.proxyHandlers != nil {
		if err := h.proxyHandlers.UnregisterMCPProxy(name); err != nil {
			logger.Error("Failed to unregister proxy during deletion", "name", name, "error", err)
		}
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListMCPsHandler handles GET /admin/mcps - lists all MCP definitions with status
func (h *AdminHandlers) ListMCPsHandler(c *gin.Context) {
	mcps, err := h.storage.ListMCPs(context.Background())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve MCP definitions",
			"details": err.Error(),
		})
		return
	}

	// Enrich with current connection status
	result := make([]map[string]any, len(mcps))
	for i, mcp := range mcps {
		status, statusErr := h.clientManager.GetClientStatus(mcp.Name)
		if statusErr != nil {
			// Client not found, set default status
			status = &MCPStatus{
				Name:   mcp.Name,
				Status: StatusDisconnected,
			}
		}

		result[i] = map[string]any{
			"definition": mcp,
			"status":     status,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"mcps":  result,
		"count": len(result),
	})
}

// GetMCPHandler handles GET /admin/mcps/{name} - gets a specific MCP definition
func (h *AdminHandlers) GetMCPHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP name is required",
		})
		return
	}

	mcp, err := h.storage.LoadMCP(context.Background(), name)
	if err != nil || mcp == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "MCP not found",
			"name":  name,
		})
		return
	}

	// Enrich with current connection status
	status, statusErr := h.clientManager.GetClientStatus(name)
	if statusErr != nil {
		// Client not found, set default status
		status = &MCPStatus{
			Name:   name,
			Status: StatusDisconnected,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"definition": mcp,
		"status":     status,
	})
}
