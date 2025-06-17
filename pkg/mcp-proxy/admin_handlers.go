package mcpproxy

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPClientInterface defines the interface for MCP client operations
type MCPClientInterface interface {
	GetDefinition() *MCPDefinition
	GetStatus() *MCPStatus
	IsConnected() bool
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	Health(ctx context.Context) error
	WaitUntilConnected(ctx context.Context) error
	ListTools(ctx context.Context) ([]mcp.Tool, error)
	CallTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	ListPrompts(ctx context.Context) ([]mcp.Prompt, error)
	GetPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
	ListResources(ctx context.Context) ([]mcp.Resource, error)
	ReadResource(ctx context.Context, request mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error)
	ListResourceTemplates(ctx context.Context) ([]mcp.ResourceTemplate, error)
	ListPromptsWithCursor(ctx context.Context, cursor string) ([]mcp.Prompt, string, error)
	ListResourcesWithCursor(ctx context.Context, cursor string) ([]mcp.Resource, string, error)
	ListResourceTemplatesWithCursor(ctx context.Context, cursor string) ([]mcp.ResourceTemplate, string, error)
}

// ClientManager defines the interface for managing MCP clients
type ClientManager interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	AddClient(ctx context.Context, def *MCPDefinition) error
	RemoveClient(ctx context.Context, name string) error
	GetClientStatus(name string) (*MCPStatus, error)
	GetClient(name string) (MCPClientInterface, error)
	GetMetrics() map[string]any
}

// errorClassification represents different types of errors for switch statements
type errorClassification int

const (
	errorUnknown errorClassification = iota
	errorNotFound
	errorAlreadyExists
	errorHotReloadFailed
	errorInvalidDefinition
	errorClientNotConnected
)

// classifyError determines the type of error for switch statement usage
func classifyError(err error) errorClassification {
	switch {
	case errors.Is(err, ErrNotFound):
		return errorNotFound
	case errors.Is(err, ErrAlreadyExists):
		return errorAlreadyExists
	case errors.Is(err, ErrHotReloadFailed):
		return errorHotReloadFailed
	case errors.Is(err, ErrInvalidDefinition):
		return errorInvalidDefinition
	case errors.Is(err, ErrClientNotConnected):
		return errorClientNotConnected
	default:
		return errorUnknown
	}
}

// AdminHandlers provides HTTP handlers for MCP management operations
type AdminHandlers struct {
	mcpService *MCPService
}

// NewAdminHandlers creates a new AdminHandlers instance
func NewAdminHandlers(mcpService *MCPService) *AdminHandlers {
	return &AdminHandlers{
		mcpService: mcpService,
	}
}

// parseAndValidateMCP parses and validates the incoming MCP definition
func (h *AdminHandlers) parseAndValidateMCP(c *gin.Context) (*MCPDefinition, error) {
	var mcpDef MCPDefinition
	if err := c.ShouldBindJSON(&mcpDef); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}
	if err := mcpDef.Validate(); err != nil {
		return nil, fmt.Errorf("invalid MCP definition: %w", err)
	}
	return &mcpDef, nil
}

// writeErrorResponse writes an error response to the client
func (h *AdminHandlers) writeErrorResponse(c *gin.Context, statusCode int, message string, details error) {
	response := gin.H{"error": message}
	if details != nil {
		response["details"] = details.Error()
	}
	c.JSON(statusCode, response)
}

// writeSuccessResponse writes a success response to the client
func (h *AdminHandlers) writeSuccessResponse(c *gin.Context, statusCode int, message string, name string) {
	c.JSON(statusCode, gin.H{
		"message": message,
		"name":    name,
	})
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
	mcpDef, err := h.parseAndValidateMCP(c)
	if err != nil {
		h.writeErrorResponse(c, http.StatusBadRequest, "Invalid request", err)
		return
	}
	if err := h.mcpService.CreateMCP(c.Request.Context(), mcpDef); err != nil {
		switch classifyError(err) {
		case errorAlreadyExists:
			h.writeErrorResponse(c, http.StatusConflict, "MCP already exists", err)
		case errorInvalidDefinition:
			h.writeErrorResponse(c, http.StatusBadRequest, "Invalid MCP definition", err)
		default:
			h.writeErrorResponse(c, http.StatusInternalServerError, "Failed to create MCP", err)
		}
		return
	}
	h.writeSuccessResponse(c, http.StatusCreated, "MCP definition added successfully", mcpDef.Name)
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
	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "MCP name is required",
		})
		return
	}
	var mcpDef MCPDefinition
	if err := c.ShouldBindJSON(&mcpDef); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid JSON payload",
			"details": err.Error(),
		})
		return
	}
	updated, err := h.mcpService.UpdateMCP(c.Request.Context(), name, &mcpDef)
	if err != nil {
		switch classifyError(err) {
		case errorNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "MCP not found",
				"name":  name,
			})
		case errorHotReloadFailed:
			c.JSON(http.StatusAccepted, gin.H{
				"message": "MCP definition updated but connection failed",
				"name":    name,
				"warning": err.Error(),
			})
		case errorInvalidDefinition:
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid MCP definition",
				"details": err.Error(),
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to update MCP",
				"details": err.Error(),
			})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "MCP definition updated successfully",
		"name":    updated.Name,
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
	if err := h.mcpService.DeleteMCP(c.Request.Context(), name); err != nil {
		switch classifyError(err) {
		case errorNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "MCP not found",
				"name":  name,
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to delete MCP",
				"details": err.Error(),
			})
		}
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

// ListMCPsHandler handles GET /admin/mcps - lists all MCP definitions with status
// @Summary List all MCP definitions
// @Description Get a list of all configured Model Context Protocol servers
// @Tags MCP Management
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Success 200 {object} ListMCPsResponse "List of MCPs with their status"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /admin/mcps [get]
func (h *AdminHandlers) ListMCPsHandler(c *gin.Context) {
	result, err := h.mcpService.ListMCPs(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve MCP definitions",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ListMCPsResponse{
		MCPs:  result,
		Count: len(result),
	})
}

// GetMCPHandler handles GET /admin/mcps/{name} - gets a specific MCP definition
// @Summary Get an MCP definition
// @Description Get details of a specific Model Context Protocol server configuration
// @Tags MCP Management
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Param name path string true "MCP name"
// @Success 200 {object} MCPDetailsResponse "MCP details with status"
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
	result, err := h.mcpService.GetMCP(c.Request.Context(), name)
	if err != nil {
		switch classifyError(err) {
		case errorNotFound:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "MCP not found",
				"name":  name,
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Storage error",
				"details": err.Error(),
			})
		}
		return
	}
	c.JSON(http.StatusOK, result)
}

// MCPToolDefinition represents a tool definition for the API response
type MCPToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	MCPName     string         `json:"mcpName"`
}

// MCPDetailsResponse represents the response structure for MCP details with status
type MCPDetailsResponse struct {
	Definition *MCPDefinition `json:"definition"`
	Status     *MCPStatus     `json:"status"`
}

// ListMCPsResponse represents the response structure for listing MCPs
type ListMCPsResponse struct {
	MCPs  []MCPDetailsResponse `json:"mcps"`
	Count int                  `json:"count"`
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
	log := logger.FromContext(c.Request.Context())
	tools, err := h.mcpService.ListAllTools(c.Request.Context())
	if err != nil {
		log.Error("Failed to list tools", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve tools",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// CallToolRequest represents a tool execution request
type CallToolRequest struct {
	MCPName   string         `json:"mcpName"`
	ToolName  string         `json:"toolName"`
	Arguments map[string]any `json:"arguments"`
}

// validateCallToolRequest validates and parses the tool call request
func (h *AdminHandlers) validateCallToolRequest(c *gin.Context) (*CallToolRequest, error) {
	var request CallToolRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return nil, fmt.Errorf("invalid JSON payload: %w", err)
	}
	if request.MCPName == "" || request.ToolName == "" {
		return nil, errors.New("mcpName and toolName are required")
	}
	return &request, nil
}

// extractToolResult extracts content from tool result
func (h *AdminHandlers) extractToolResult(result *mcp.CallToolResult) any {
	if result == nil {
		return nil
	}
	if len(result.Content) == 0 {
		return map[string]any{
			"error": "No content in tool result",
		}
	}
	content := result.Content[0]
	switch typedContent := content.(type) {
	case mcp.TextContent:
		return map[string]any{
			"type": "text",
			"text": typedContent.Text,
		}
	case mcp.ImageContent:
		return map[string]any{
			"type":     "image",
			"data":     typedContent.Data,
			"mimeType": typedContent.MIMEType,
		}
	default:
		return map[string]any{
			"type": "unknown",
			"data": content,
		}
	}
}

// CallToolHandler executes a tool on a specific MCP server
// @Summary Call a tool on an MCP server
// @Description Execute a specific tool with provided arguments on the specified MCP server
// @Tags MCP Tools
// @Accept json
// @Produce json
// @Param Authorization header string true "Admin authorization token"
// @Param request body CallToolRequest true "Tool call request"
// @Success 200 {object} map[string]interface{} "Tool execution result"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 404 {object} map[string]interface{} "MCP or tool not found"
// @Failure 500 {object} map[string]interface{} "Tool execution failed"
// @Router /admin/tools/call [post]
func (h *AdminHandlers) CallToolHandler(c *gin.Context) {
	request, err := h.validateCallToolRequest(c)
	if err != nil {
		h.writeErrorResponse(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	result, err := h.mcpService.CallTool(c.Request.Context(), request.MCPName, request.ToolName, request.Arguments)
	if err != nil {
		switch classifyError(err) {
		case errorClientNotConnected:
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "MCP not found or not connected",
				"details": err.Error(),
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Tool execution failed",
				"details": err.Error(),
			})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result": h.extractToolResult(result),
	})
}
