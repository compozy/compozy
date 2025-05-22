package router

import (
	"net/http"

	"github.com/compozy/compozy/pkg/logger"
	"github.com/gin-gonic/gin"
)

type Response struct {
	Status  int        `json:"status"`
	Message string     `json:"message,omitempty"`
	Data    any        `json:"data,omitempty"`
	Error   *ErrorInfo `json:"error,omitempty"`
}

func NewResponse(status int, message string, data any) Response {
	return Response{
		Status:  status,
		Message: message,
		Data:    data,
	}
}

func NewErrorResponse(status int, code, message string, details string) Response {
	return Response{
		Status: status,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

func RespondWithSuccess(c *gin.Context, statusCode int, message string, data any) {
	c.JSON(statusCode, NewResponse(statusCode, message, data))
}

func RespondWithError(c *gin.Context, statusCode int, err *RequestError) {
	errorInfo := err.GetErrorInfo()
	resp := Response{
		Status: statusCode,
		Error:  errorInfo,
	}
	logger.Error("Request error",
		"error_code", errorInfo.Code,
		"error_message", errorInfo.Message,
		"details", errorInfo.Details,
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
		"status_code", statusCode,
	)
	c.JSON(statusCode, resp)
}

func RespondWithServerError(c *gin.Context, code string, message string, err error) {
	var details string
	if err != nil {
		details = err.Error()
	}
	statusCode := getStatusCode(code)
	logger.Error("Server error",
		"error_code", code,
		"error_message", message,
		"error", err,
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
	)
	c.JSON(statusCode, NewErrorResponse(statusCode, code, message, details))
}

func RespondOK(c *gin.Context, message string, data any) {
	RespondWithSuccess(c, http.StatusOK, message, data)
}

func RespondCreated(c *gin.Context, message string, data any) {
	RespondWithSuccess(c, http.StatusCreated, message, data)
}

func RespondAccepted(c *gin.Context, message string, data any) {
	RespondWithSuccess(c, http.StatusAccepted, message, data)
}

func RespondNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			var statusCode int
			var response Response
			if reqErr, ok := err.(*RequestError); ok {
				errorInfo := reqErr.GetErrorInfo()
				statusCode = reqErr.StatusCode
				response = NewErrorResponse(statusCode, errorInfo.Code, errorInfo.Message, errorInfo.Details)
			} else if serverErr, ok := err.(*Error); ok {
				statusCode = getStatusCode(serverErr.Code)
				var details string
				if serverErr.Err != nil {
					details = serverErr.Err.Error()
				}
				response = NewErrorResponse(statusCode, serverErr.Code, serverErr.Message, details)
			} else {
				statusCode = http.StatusInternalServerError
				response = NewErrorResponse(statusCode, ErrInternalCode, "An unexpected error occurred", err.Error())
			}

			logger.Error("Request failed",
				"error", err,
				"path", c.Request.URL.Path,
				"method", c.Request.Method,
				"status_code", statusCode,
			)

			c.JSON(statusCode, response)
		}
	}
}
