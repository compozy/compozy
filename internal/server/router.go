package server

import (
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/logger"
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/trigger"
	"github.com/compozy/compozy/internal/parser/workflow"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Route defines a server route
type Route struct {
	Path     string
	Workflow *workflow.Config
}

// normalizePath ensures the path starts with a single slash and preserves trailing slashes
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}

	hasTrailingSlash := strings.HasSuffix(p, "/")
	cleanPath := path.Clean(p)

	// Ensure path starts with a single slash
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	// Restore trailing slash if it was present in the original path
	if hasTrailingSlash {
		cleanPath = cleanPath + "/"
	}

	return cleanPath
}

// RouteFromWorkflow creates a Route from a WorkflowConfig
func RouteFromWorkflow(workflow *workflow.Config) (*Route, error) {
	t := workflow.Trigger
	if t.Type != trigger.TriggerTypeWebhook {
		return nil, ErrRouteNotDefined
	}

	// Get URL from webhook config
	if t.Config == nil {
		return nil, ErrRouteNotDefined
	}

	url := t.Config.URL
	if url == "" {
		return nil, ErrRouteNotDefined
	}

	return &Route{
		Path:     normalizePath(url),
		Workflow: workflow,
	}, nil
}

// handleRequest handles an incoming webhook request
func handleRequest(c *gin.Context, workflow *workflow.Config) {
	start := time.Now()
	execID := uuid.New().String()
	_, err := GetAppState(c.Request.Context())
	if err != nil {
		logger.Error("Failed to get app state",
			"exec_id", execID,
			"error", err,
		)
		reqErr := NewRequestError(http.StatusInternalServerError, "Failed to get app state", err)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	// Parse the input JSON
	var inputData common.Input
	if err := c.ShouldBindJSON(&inputData); err != nil {
		logger.Error("Failed to parse JSON input",
			"exec_id", execID,
			"workflow_id", workflow.ID,
			"error", err,
		)
		reqErr := NewRequestError(http.StatusBadRequest, "Invalid JSON input: "+err.Error(), err)
		c.JSON(reqErr.StatusCode, reqErr.ToErrorResponse())
		return
	}

	// Return success response
	duration := time.Since(start)
	logger.Debug("Webhook request completed successfully",
		"exec_id", execID,
		"workflow_id", workflow.ID,
		"duration_ms", duration.Milliseconds(),
	)

	c.JSON(http.StatusOK, gin.H{
		"duration": duration.Milliseconds(),
		"status":   "success",
		"message":  "Workflow triggered successfully",
		"data":     map[string]any{},
	})
}

// RegisterRoutes registers all workflow routes with the given router
func RegisterRoutes(router *gin.Engine, state *AppState) error {
	registeredRoutes := make(map[string]bool)
	routeCount := 0

	for _, workflow := range state.Workflows {
		route, err := RouteFromWorkflow(workflow)
		if err != nil {
			if err == ErrRouteNotDefined {
				logger.Debug("Skipping workflow without webhook trigger",
					"workflow_id", workflow.ID,
					"trigger_type", workflow.Trigger.Type,
				)
			} else {
				logger.Error("Failed to create route from workflow",
					"workflow_id", workflow.ID,
					"error", err,
				)
			}
			continue // Skip workflows without webhook triggers
		}

		// Normalize the path
		path := route.Path
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		if _, exists := registeredRoutes[path]; exists {
			logger.Error("Detected route conflict",
				"path", path,
				"workflow_id", workflow.ID,
			)
			return fmt.Errorf("%w: %s", ErrRouteConflict, path)
		}

		registeredRoutes[path] = true
		routeCount++

		logger.Debug("Registering webhook route",
			"path", path,
			"workflow_id", workflow.ID,
			"method", "POST",
		)

		router.POST(path, func(c *gin.Context) {
			handleRequest(c, route.Workflow)
		})
	}

	logger.Info("Completed route registration",
		"total_routes", routeCount,
		"total_workflows", len(state.Workflows),
	)

	return nil
}
