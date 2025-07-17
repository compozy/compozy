package helpers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/tidwall/pretty"
)

// JSONResponse represents a standardized JSON response structure
type JSONResponse struct {
	Success  bool               `json:"success"`
	Data     any                `json:"data,omitempty"`
	Error    *JSONError         `json:"error,omitempty"`
	Metadata *FormatterMetadata `json:"metadata,omitempty"`
}

// JSONError represents error information in JSON responses
type JSONError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// FormatterMetadata represents response metadata
type FormatterMetadata struct {
	Timestamp  time.Time       `json:"timestamp"`
	RequestID  string          `json:"request_id,omitempty"`
	Pagination *PaginationInfo `json:"pagination,omitempty"`
}

// PaginationInfo represents pagination metadata
type PaginationInfo struct {
	Limit     int `json:"limit"`
	Offset    int `json:"offset"`
	Total     int `json:"total"`
	PageCount int `json:"page_count"`
}

// JSONFormatter handles JSON output formatting
type JSONFormatter struct {
	Pretty bool
}

// NewJSONFormatter creates a new JSON formatter
func NewJSONFormatter(pretty bool) *JSONFormatter {
	return &JSONFormatter{Pretty: pretty}
}

// FormatSuccess formats a successful response
func (f *JSONFormatter) FormatSuccess(data any, metadata *FormatterMetadata) (string, error) {
	response := JSONResponse{
		Success:  true,
		Data:     data,
		Metadata: metadata,
	}
	return f.marshal(response)
}

// FormatError formats an error response
func (f *JSONFormatter) FormatError(err error, code string, details string) (string, error) {
	response := JSONResponse{
		Success: false,
		Error: &JSONError{
			Code:    code,
			Message: err.Error(),
			Details: details,
		},
		Metadata: &FormatterMetadata{
			Timestamp: time.Now(),
		},
	}
	return f.marshal(response)
}

// FormatWorkflowList formats workflow list response
func (f *JSONFormatter) FormatWorkflowList(workflows any, total int, limit int, offset int) (string, error) {
	pageCount := 0
	if limit > 0 {
		pageCount = (total + limit - 1) / limit
	}

	data := map[string]any{
		"workflows": workflows,
		"total":     total,
	}

	metadata := &FormatterMetadata{
		Timestamp: time.Now(),
		Pagination: &PaginationInfo{
			Limit:     limit,
			Offset:    offset,
			Total:     total,
			PageCount: pageCount,
		},
	}

	return f.FormatSuccess(data, metadata)
}

// marshal handles JSON marshaling with optional pretty printing
func (f *JSONFormatter) marshal(response JSONResponse) (string, error) {
	jsonData, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if f.Pretty {
		prettyJSON := pretty.Pretty(jsonData)
		return string(prettyJSON), nil
	}

	return string(jsonData), nil
}
