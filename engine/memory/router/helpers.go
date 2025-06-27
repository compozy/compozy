package memrouter

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/engine/infra/server/router"
	"github.com/compozy/compozy/engine/llm"
	memuc "github.com/compozy/compozy/engine/memory/uc"
	"github.com/gin-gonic/gin"
)

// handleMemoryError processes memory-related errors and sends appropriate responses
func handleMemoryError(c *gin.Context, err error, message string) {
	var statusCode int
	var errorMessage string

	// Check for specific error types using switch
	switch {
	case errors.Is(err, memuc.ErrMemoryNotFound):
		statusCode = http.StatusNotFound
		errorMessage = "memory not found"
	case errors.Is(err, memuc.ErrInvalidMemoryRef):
		statusCode = http.StatusBadRequest
		errorMessage = "invalid memory reference"
	case errors.Is(err, memuc.ErrInvalidKey):
		statusCode = http.StatusBadRequest
		errorMessage = "invalid memory key"
	case errors.Is(err, memuc.ErrInvalidPayload):
		statusCode = http.StatusBadRequest
		errorMessage = "invalid payload"
	case errors.Is(err, memuc.ErrMemoryManagerNotAvailable):
		statusCode = http.StatusInternalServerError
		errorMessage = "memory manager not available"
	default:
		// Default to internal server error
		statusCode = http.StatusInternalServerError
		errorMessage = message
	}

	reqErr := router.NewRequestError(statusCode, errorMessage, err)
	router.RespondWithError(c, reqErr.StatusCode, reqErr)
}

// convertMessagesToResponse converts LLM messages to response format
func convertMessagesToResponse(messages []llm.Message) []map[string]any {
	result := make([]map[string]any, 0, len(messages))

	for _, msg := range messages {
		msgMap := map[string]any{
			"role":    string(msg.Role),
			"content": msg.Content,
		}
		result = append(result, msgMap)
	}

	return result
}
