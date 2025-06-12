package mcpproxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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

// isNotFoundError checks if an error is a "not found" error
func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// validateAndPrepareMCP validates the incoming MCP definition and checks for conflicts
func (h *AdminHandlers) validateAndPrepareMCP(c *gin.Context) (*MCPDefinition, bool) {
	var mcpDef MCPDefinition
	if err := c.ShouldBindJSON(&mcpDef); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
		return nil, false
	}

	// Validate the MCP definition
	if err := mcpDef.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid MCP definition",
			"details": err.Error(),
		})
		return nil, false
	}

	// Check if MCP with same name already exists
	existing, err := h.storage.LoadMCP(c.Request.Context(), mcpDef.Name)
	if err != nil && !isNotFoundError(err) {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Storage error",
			"details": err.Error(),
		})
		return nil, false
	}
	if existing != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "MCP with this name already exists",
			"name":  mcpDef.Name,
		})
		return nil, false
	}

	// Set timestamps
	now := time.Now()
	mcpDef.CreatedAt = now
	mcpDef.UpdatedAt = now

	return &mcpDef, true
}

// connectMCPWithFallback attempts immediate connection with async fallback
func (h *AdminHandlers) connectMCPWithFallback(c *gin.Context, mcpDef *MCPDefinition) bool {
	// Try immediate connection first, fall back to async on timeout
	connectCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	immediateErr := h.clientManager.AddClient(connectCtx, mcpDef)
	if immediateErr != nil {
		// Check if it was a timeout - if so, proceed async
		if connectCtx.Err() == context.DeadlineExceeded {
			// Background connection attempt
			go func() {
				h.handleAsyncConnection(mcpDef)
			}()
			return true
		}
		// Immediate failure - return error to user
		c.JSON(http.StatusBadGateway, gin.H{
			"error":   "Failed to connect to MCP server",
			"details": immediateErr.Error(),
		})
		return false
	}

	// Immediate success - register proxy synchronously
	if h.proxyHandlers != nil {
		if err := h.proxyHandlers.RegisterMCPProxy(context.Background(), mcpDef.Name, mcpDef); err != nil {
			logger.Warn("Proxy registration failed but client is connected", "name", mcpDef.Name, "error", err)
		}
	}
	return true
}

// handleAsyncConnection handles background MCP connection and proxy registration
func (h *AdminHandlers) handleAsyncConnection(mcpDef *MCPDefinition) {
	if err := h.clientManager.AddClient(context.Background(), mcpDef); err != nil {
		status := &MCPStatus{
			Name:      mcpDef.Name,
			Status:    StatusError,
			LastError: err.Error(),
		}
		if saveErr := h.storage.SaveStatus(context.Background(), status); saveErr != nil {
			logger.Error("Failed to save error status", "name", mcpDef.Name, "error", saveErr)
		}
		return
	}

	if h.proxyHandlers != nil {
		// Register the MCP as a proxy endpoint
		if err := h.proxyHandlers.RegisterMCPProxy(context.Background(), mcpDef.Name, mcpDef); err != nil {
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
}

// AddMCPHandler handles POST /admin/mcps - adds a new MCP definition
// @Summary Add a new MCP definition
// @Description Add a new Model Context Protocol server configuration
// @Tags MCP Management
// @Accept json
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Param mcp body MCPDefinition true "MCP definition to add"
// @Success 201 {object} map[string]interface{} "MCP added successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 409 {object} map[string]interface{} "MCP already exists"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/mcps [post]
func (h *AdminHandlers) AddMCPHandler(c *gin.Context) {
	mcpDef, valid := h.validateAndPrepareMCP(c)
	if !valid {
		return
	}

	// Save MCP definition to storage
	if err := h.storage.SaveMCP(c.Request.Context(), mcpDef); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to save MCP definition",
			"details": err.Error(),
		})
		return
	}

	// Attempt connection with fallback strategy
	if !h.connectMCPWithFallback(c, mcpDef) {
		return
	}

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
	if err != nil {
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "MCP not found",
				"name":  name,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Storage error",
				"details": err.Error(),
			})
		}
		return nil, nil, false
	}

	return &mcpDef, existing, true
}

// performHotReload removes old client and adds updated client with proper error handling
func (h *AdminHandlers) performHotReload(name string, mcpDef *MCPDefinition) error {
	// Remove old client and unregister proxy
	if err := h.clientManager.RemoveClient(context.Background(), name); err != nil {
		logger.Error("Failed to remove client during update", "name", name, "error", err)
	}
	if h.proxyHandlers != nil {
		if err := h.proxyHandlers.UnregisterMCPProxy(name); err != nil {
			logger.Error("Failed to unregister proxy during update", "name", name, "error", err)
		}
	}

	// Try immediate connection first with timeout
	connectCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	immediateErr := h.clientManager.AddClient(connectCtx, mcpDef)
	if immediateErr != nil {
		// Check if it was a timeout - if so, proceed async
		if connectCtx.Err() == context.DeadlineExceeded {
			// Background connection attempt for timeout case
			go func() {
				if err := h.clientManager.AddClient(context.Background(), mcpDef); err != nil {
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
			return nil // Async connection in progress
		}
		// Immediate failure
		return fmt.Errorf("failed to reconnect: %w", immediateErr)
	}

	if h.proxyHandlers != nil {
		// Register the updated MCP as a proxy endpoint (synchronous since connection succeeded)
		if err := h.proxyHandlers.RegisterMCPProxy(context.Background(), mcpDef.Name, mcpDef); err != nil {
			logger.Warn(
				"Proxy registration failed but client is connected during update",
				"name",
				mcpDef.Name,
				"error",
				err,
			)
		}
	}

	return nil
}

// UpdateMCPHandler handles PUT /admin/mcps/{name} - updates an existing MCP definition
// @Summary Update an MCP definition
// @Description Update an existing Model Context Protocol server configuration
// @Tags MCP Management
// @Accept json
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Param name path string true "MCP name"
// @Param mcp body MCPDefinition true "Updated MCP definition"
// @Success 200 {object} map[string]interface{} "MCP updated successfully"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "MCP not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/mcps/{name} [put]
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

	// Now perform the hot reload operations
	if err := h.performHotReload(mcpDef.Name, mcpDef); err != nil {
		// Hot reload failed, but definition was saved - return partial success
		c.JSON(http.StatusAccepted, gin.H{
			"message": "MCP definition updated but connection failed",
			"name":    mcpDef.Name,
			"warning": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "MCP definition updated successfully",
		"name":    mcpDef.Name,
	})
}

// RemoveMCPHandler handles DELETE /admin/mcps/{name} - removes an MCP definition
// @Summary Remove an MCP definition
// @Description Remove a Model Context Protocol server configuration
// @Tags MCP Management
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Param name path string true "MCP name"
// @Success 200 {object} map[string]interface{} "MCP removed successfully"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "MCP not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/mcps/{name} [delete]
func (h *AdminHandlers) RemoveMCPHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP name is required",
		})
		return
	}

	// Check if MCP exists
	_, err := h.storage.LoadMCP(context.Background(), name)
	if err != nil {
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "MCP not found",
				"name":  name,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Storage error",
				"details": err.Error(),
			})
		}
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
// @Summary List all MCP definitions
// @Description Get a list of all configured Model Context Protocol servers
// @Tags MCP Management
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Success 200 {object} map[string]interface{} "List of MCPs with their status"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/mcps [get]
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
// @Summary Get an MCP definition
// @Description Get details of a specific Model Context Protocol server configuration
// @Tags MCP Management
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Param name path string true "MCP name"
// @Success 200 {object} map[string]interface{} "MCP details with status"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "MCP not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/mcps/{name} [get]
func (h *AdminHandlers) GetMCPHandler(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP name is required",
		})
		return
	}

	mcp, err := h.storage.LoadMCP(context.Background(), name)
	if err != nil {
		if isNotFoundError(err) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "MCP not found",
				"name":  name,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Storage error",
				"details": err.Error(),
			})
		}
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

// MCPToolDefinition represents a tool definition for the API response
type MCPToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	MCPName     string         `json:"mcpName"`
}

// ListToolsHandler returns all available tools from registered MCPs
// @Summary List all available tools
// @Description Get a list of all tools available from all connected MCP servers
// @Tags MCP Tools
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Success 200 {object} map[string]interface{} "List of available tools"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/tools [get]
func (h *AdminHandlers) ListToolsHandler(c *gin.Context) {
	logger.Debug("Listing all available tools from registered MCPs")

	// Get all registered MCPs
	mcps, err := h.storage.ListMCPs(c.Request.Context())
	if err != nil {
		logger.Error("Failed to list MCPs for tools discovery", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve MCPs",
			"details": err.Error(),
		})
		return
	}

	var allTools []MCPToolDefinition

	// Iterate through each MCP and get its tools
	for _, mcpDef := range mcps {
		client, err := h.clientManager.GetClient(mcpDef.Name)
		if err != nil {
			logger.Warn("Failed to get client for MCP, skipping", "mcp_name", mcpDef.Name, "error", err)
			continue
		}

		// Get tools from this MCP client
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		tools, err := client.ListTools(ctx)
		if err != nil {
			logger.Warn("Failed to list tools for MCP, skipping", "mcp_name", mcpDef.Name, "error", err)
			cancel()
			continue
		}

		// Convert tools to our API format
		for i := range tools {
			// Convert the tool's input schema to a generic map using JSON marshaling
			tool := &tools[i]
			var inputSchema map[string]any
			if schemaBytes, err := json.Marshal(tool.InputSchema); err == nil {
				if err := json.Unmarshal(schemaBytes, &inputSchema); err != nil {
					logger.Warn("Failed to unmarshal tool input schema", "mcp_name", mcpDef.Name, "error", err)
				}
			}

			toolDef := MCPToolDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: inputSchema,
				MCPName:     mcpDef.Name,
			}
			allTools = append(allTools, toolDef)
		}

		logger.Debug("Listed tools for MCP", "mcp_name", mcpDef.Name, "tool_count", len(tools))
		cancel()
	}

	logger.Info("Listed all available tools", "total_tools", len(allTools), "total_mcps", len(mcps))

	c.JSON(http.StatusOK, gin.H{
		"tools": allTools,
	})
}
