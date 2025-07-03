package memrouter

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/compozy/compozy/engine/worker"
	"github.com/gin-gonic/gin"
)

// MemoryContext contains common extracted parameters for memory operations
type MemoryContext struct {
	MemoryRef    string
	Key          string
	Manager      *memory.Manager
	Worker       *worker.Worker
	TokenCounter memcore.TokenCounter
}

const memoryContextKey = "memoryContext"

// ExtractMemoryContext is a middleware that extracts common memory parameters
func ExtractMemoryContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract memory reference from path
		memoryRef := c.Param("memory_ref")

		// Validate memory reference
		if memoryRef == "" {
			reqErr := router.NewRequestError(
				http.StatusBadRequest,
				"memory_ref is required",
				nil,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}

		// Extract key based on HTTP method
		// For GET requests, key comes from query parameter
		// For POST requests, key will come from request body (handled in handlers)
		var key string
		if c.Request.Method == http.MethodGet {
			key = c.Query("key")
			// Validate key for GET requests that require it
			// Check the actual endpoint path to determine if key is required
			requestPath := c.Request.URL.Path
			if (strings.HasSuffix(requestPath, "/read") ||
				strings.HasSuffix(requestPath, "/health") ||
				strings.HasSuffix(requestPath, "/stats")) && key == "" {
				reqErr := router.NewRequestError(
					http.StatusBadRequest,
					"key query parameter is required",
					nil,
				)
				router.RespondWithError(c, reqErr.StatusCode, reqErr)
				c.Abort()
				return
			}
		}
		// For POST requests, key validation will be done in individual handlers
		// as it will be part of the request body

		// Get app state
		appState := router.GetAppState(c)
		if appState == nil {
			return // GetAppState handles the error response
		}

		// Check worker
		if appState.Worker == nil {
			reqErr := router.NewRequestError(
				http.StatusInternalServerError,
				"worker not available",
				nil,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}

		// Get memory manager
		memoryManager := appState.Worker.GetMemoryManager()
		if memoryManager == nil {
			reqErr := router.NewRequestError(
				http.StatusInternalServerError,
				"memory manager not configured",
				nil,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}

		// Get token counter
		tokenCounter, err := memoryManager.GetTokenCounter(c.Request.Context())
		if err != nil {
			reqErr := router.NewRequestError(
				http.StatusInternalServerError,
				"failed to get token counter",
				err,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}

		// Create memory context
		ctx := &MemoryContext{
			MemoryRef:    memoryRef,
			Key:          key,
			Manager:      memoryManager,
			Worker:       appState.Worker,
			TokenCounter: tokenCounter,
		}

		// Store in gin context
		c.Set(memoryContextKey, ctx)
		c.Next()
	}
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
