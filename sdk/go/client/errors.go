package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError preserves a daemon's non-success response and machine-readable details.
type APIError struct {
	Status        int
	Code          string
	Message       string
	Details       map[string]any
	CurrentTurnID string
}

// Error returns the daemon's status and error message.
func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("compozy API %d (%s): %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("compozy API %d: %s", e.Status, e.Message)
}

// StreamError reports an interrupted or malformed event stream.
type StreamError struct {
	Op  string
	Err error
}

// Error identifies the operation whose stream failed.
func (e *StreamError) Error() string { return fmt.Sprintf("compozy stream %s: %v", e.Op, e.Err) }

// Unwrap exposes the underlying decoding, transport, or context error.
func (e *StreamError) Unwrap() error { return e.Err }

func responseError(response *http.Response) error {
	result := &APIError{Status: response.StatusCode, Message: http.StatusText(response.StatusCode)}
	data, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("%w: read daemon error response: %w", result, err)
	}
	var body struct {
		Error         string         `json:"error"`
		Message       string         `json:"message"`
		Code          string         `json:"code"`
		Details       map[string]any `json:"details"`
		CurrentTurnID string         `json:"current_turn_id"`
	}
	if err := json.Unmarshal(data, &body); err == nil {
		result.Code, result.Details, result.CurrentTurnID = body.Code, body.Details, body.CurrentTurnID
		if body.Error != "" {
			result.Message = body.Error
		} else if body.Message != "" {
			result.Message = body.Message
		}
	}
	return result
}
