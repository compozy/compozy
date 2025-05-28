package swagger

import (
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router"
)

// StandardResponse represents the standard API response format
type StandardResponse struct {
	Status  int    `json:"status" example:"200"`
	Message string `json:"message" example:"Success"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents an error response
type Error struct {
	Code    string `json:"code" example:"VALIDATION_ERROR"`
	Message string `json:"message" example:"Invalid input provided"`
	Details string `json:"details,omitempty" example:"Field 'name' is required"`
}

// PaginationMeta represents pagination metadata
type PaginationMeta struct {
	Page       int `json:"page" example:"1"`
	PerPage    int `json:"per_page" example:"20"`
	Total      int `json:"total" example:"100"`
	TotalPages int `json:"total_pages" example:"5"`
}

// ListResponse represents a paginated list response
type ListResponse struct {
	StandardResponse
	Meta *PaginationMeta `json:"meta,omitempty"`
}

// ExecutionResponse represents execution data in responses
type ExecutionResponse struct {
	ID             core.ID         `json:"id" example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	Status         core.StatusType `json:"status" example:"completed"`
	Component      string          `json:"component" example:"workflow"`
	WorkflowID     string          `json:"workflow_id" example:"data-processing"`
	WorkflowExecID core.ID         `json:"workflow_exec_id" example:"2Z4PVTL6K27XVT4A3NPKMDD5BG"`
	Input          core.Input      `json:"input,omitempty"`
	Output         core.Output     `json:"output,omitempty"`
	Error          *core.Error     `json:"error,omitempty"`
	StartTime      string          `json:"start_time" example:"2024-01-15T10:30:00Z"`
	EndTime        string          `json:"end_time" example:"2024-01-15T10:35:00Z"`
	Duration       string          `json:"duration" example:"5m0s"`
}

// WorkflowExecutionResponse represents a workflow execution with children
type WorkflowExecutionResponse struct {
	ExecutionResponse
	Tasks  []ExecutionResponse `json:"tasks,omitempty"`
	Agents []ExecutionResponse `json:"agents,omitempty"`
	Tools  []ExecutionResponse `json:"tools,omitempty"`
}

// ErrorResponse creates a standardized error response
func ErrorResponse(code, message, details string) router.Response {
	return router.Response{
		Status: getStatusCodeFromErrorCode(code),
		Error: &router.ErrorInfo{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

func getStatusCodeFromErrorCode(code string) int {
	switch code {
	case "VALIDATION_ERROR", "INVALID_INPUT":
		return 400
	case "NOT_FOUND":
		return 404
	case "CONFLICT":
		return 409
	default:
		return 500
	}
}
