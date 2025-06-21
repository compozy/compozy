package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SuccessResponse represents a standardized success response
type SuccessResponse struct {
	Data    any    `json:"data"`
	Message string `json:"message"`
}

// SendErrorResponse sends a standardized error response
func SendErrorResponse(c *gin.Context, statusCode int, errorMsg string, details string) {
	response := ErrorResponse{
		Error: errorMsg,
	}
	if details != "" {
		response.Details = details
	}
	c.JSON(statusCode, response)
}

// SendSuccessResponse sends a standardized success response
func SendSuccessResponse(c *gin.Context, statusCode int, data any, message string) {
	c.JSON(statusCode, SuccessResponse{
		Data:    data,
		Message: message,
	})
}

// Common error responses

// SendUnauthorizedError sends a 401 unauthorized error
func SendUnauthorizedError(c *gin.Context, details string) {
	SendErrorResponse(c, http.StatusUnauthorized, "Unauthorized", details)
	c.Abort()
}

// SendForbiddenError sends a 403 forbidden error
func SendForbiddenError(c *gin.Context, details string) {
	SendErrorResponse(c, http.StatusForbidden, "Forbidden", details)
	c.Abort()
}

// SendBadRequestError sends a 400 bad request error
func SendBadRequestError(c *gin.Context, details string) {
	SendErrorResponse(c, http.StatusBadRequest, "Bad Request", details)
	c.Abort()
}

// SendNotFoundError sends a 404 not found error
func SendNotFoundError(c *gin.Context, details string) {
	SendErrorResponse(c, http.StatusNotFound, "Not Found", details)
	c.Abort()
}

// SendInternalServerError sends a 500 internal server error
func SendInternalServerError(c *gin.Context, details string) {
	SendErrorResponse(c, http.StatusInternalServerError, "Internal Server Error", details)
	c.Abort()
}

// SendRateLimitError sends a 429 too many requests error
func SendRateLimitError(c *gin.Context, details string) {
	SendErrorResponse(c, http.StatusTooManyRequests, "Rate Limit Exceeded", details)
	c.Abort()
}
