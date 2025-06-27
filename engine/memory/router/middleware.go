package memrouter

import (
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/memory"
	"github.com/compozy/compozy/engine/worker"
	"github.com/gin-gonic/gin"
)

// MemoryContext contains common extracted parameters for memory operations
type MemoryContext struct {
	MemoryRef string
	Key       string
	Manager   *memory.Manager
	Worker    *worker.Worker
}

const memoryContextKey = "memoryContext"

// ExtractMemoryContext is a middleware that extracts common memory parameters
func ExtractMemoryContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract parameters
		memoryRef := c.Param("memory_ref")
		key := c.Param("key")

		// Validate parameters
		if memoryRef == "" {
			reqErr := router.NewRequestError(
				http.StatusBadRequest,
				"memory_ref is required",
				nil,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}

		if key == "" {
			reqErr := router.NewRequestError(
				http.StatusBadRequest,
				"key is required",
				nil,
			)
			router.RespondWithError(c, reqErr.StatusCode, reqErr)
			return
		}

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

		// Create memory context
		ctx := &MemoryContext{
			MemoryRef: memoryRef,
			Key:       key,
			Manager:   memoryManager,
			Worker:    appState.Worker,
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
