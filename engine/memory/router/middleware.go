package memrouter

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/worker"
	"github.com/gin-gonic/gin"
)

// MemoryContext contains common extracted parameters for memory operations
type MemoryContext struct {
	MemoryRef       string
	Key             string
	Manager         *memory.Manager
	Worker          *worker.Worker
	TokenCounter    memcore.TokenCounter
	WorkflowContext map[string]any
}

const memoryContextKey = "memoryContext"

// ExtractMemoryContext is a middleware that extracts common memory parameters
func ExtractMemoryContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract and validate memory reference
		memoryRef := c.Param("memory_ref")
		if err := validateMemoryRef(c, memoryRef); err != nil {
			return
		}

		// Extract key based on HTTP method
		key := extractKey(c)

		// Get app state and validate dependencies
		appState, memoryManager, tokenCounter, err := getDependencies(c)
		if err != nil {
			return
		}

		// Build workflow context
		workflowContext := buildWorkflowContext(appState)

		// Create and store memory context
		ctx := &MemoryContext{
			MemoryRef:       memoryRef,
			Key:             key,
			Manager:         memoryManager,
			Worker:          appState.Worker,
			TokenCounter:    tokenCounter,
			WorkflowContext: workflowContext,
		}
		c.Set(memoryContextKey, ctx)
		c.Next()
	}
}

// validateMemoryRef validates the memory reference parameter
func validateMemoryRef(c *gin.Context, memoryRef string) error {
	if memoryRef == "" {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"memory_ref is required",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return reqErr
	}
	return nil
}

// extractKey extracts and validates the key parameter based on HTTP method
func extractKey(c *gin.Context) string {
	if c.Request.Method != http.MethodGet {
		return ""
	}
	key := c.Query("key")
	requestPath := c.Request.URL.Path
	// Validate key for GET requests that require it
	if requiresKey(requestPath) && key == "" {
		reqErr := router.NewRequestError(
			http.StatusBadRequest,
			"key query parameter is required",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		c.Abort()
		return ""
	}
	return key
}

// requiresKey checks if the endpoint requires a key parameter
func requiresKey(path string) bool {
	return strings.HasSuffix(path, "/read") ||
		strings.HasSuffix(path, "/health") ||
		strings.HasSuffix(path, "/stats")
}

// getDependencies retrieves and validates required dependencies
func getDependencies(c *gin.Context) (*appstate.State, *memory.Manager, memcore.TokenCounter, error) {
	appState := router.GetAppState(c)
	if appState == nil {
		return nil, nil, nil, fmt.Errorf("app state not available")
	}
	if appState.Worker == nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"worker not available",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, nil, nil, reqErr
	}
	memoryManager := appState.Worker.GetMemoryManager()
	if memoryManager == nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"memory manager not configured",
			nil,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, nil, nil, reqErr
	}
	tokenCounter, err := memoryManager.GetTokenCounter(c.Request.Context())
	if err != nil {
		reqErr := router.NewRequestError(
			http.StatusInternalServerError,
			"failed to get token counter",
			err,
		)
		router.RespondWithError(c, reqErr.StatusCode, reqErr)
		return nil, nil, nil, reqErr
	}
	return appState, memoryManager, tokenCounter, nil
}

// buildWorkflowContext builds the workflow context for REST API operations
func buildWorkflowContext(appState *appstate.State) map[string]any {
	workflowContext := map[string]any{"api_operation": "memory"}
	if appState.ProjectConfig != nil {
		workflowContext["project"] = map[string]any{
			"id":          appState.ProjectConfig.Name,
			"name":        appState.ProjectConfig.Name,
			"version":     appState.ProjectConfig.Version,
			"description": appState.ProjectConfig.Description,
		}
	}
	return workflowContext
}

// GetMemoryContext retrieves the memory context from gin context
func GetMemoryContext(c *gin.Context) (*MemoryContext, bool) {
	value, exists := c.Get(memoryContextKey)
	if !exists {
		return nil, false
	}
	ctx, ok := value.(*MemoryContext)
	return ctx, ok
}
