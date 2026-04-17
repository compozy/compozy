package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
)

// TransportError is the canonical non-2xx JSON error envelope.
type TransportError struct {
	RequestID string         `json:"request_id"`
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   map[string]any `json:"details,omitempty"`
}

// Problem carries a transport status, code, and detail payload for one failure.
type Problem struct {
	Status  int
	Code    string
	Message string
	Details map[string]any
	Err     error
}

// NewProblem returns a transport-aware error wrapper.
func NewProblem(status int, code string, message string, details map[string]any, err error) *Problem {
	return &Problem{
		Status:  status,
		Code:    strings.TrimSpace(code),
		Message: strings.TrimSpace(message),
		Details: details,
		Err:     err,
	}
}

func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	if strings.TrimSpace(p.Message) != "" {
		return p.Message
	}
	if p.Err != nil {
		return p.Err.Error()
	}
	if text := http.StatusText(p.Status); text != "" {
		return text
	}
	return "transport error"
}

func (p *Problem) Unwrap() error {
	if p == nil {
		return nil
	}
	return p.Err
}

func statusForError(err error) int {
	if err == nil {
		return http.StatusOK
	}

	var problem *Problem
	if errors.As(err, &problem) && problem != nil && problem.Status > 0 {
		return problem.Status
	}

	switch {
	case errors.Is(err, os.ErrNotExist),
		errors.Is(err, globaldb.ErrWorkspaceNotFound),
		errors.Is(err, globaldb.ErrWorkflowNotFound),
		errors.Is(err, globaldb.ErrRunNotFound):
		return http.StatusNotFound
	case errors.Is(err, globaldb.ErrWorkspaceHasActiveRuns),
		errors.Is(err, globaldb.ErrWorkflowSlugConflict),
		errors.Is(err, globaldb.ErrRunAlreadyExists),
		errors.Is(err, globaldb.ErrSchemaTooNew),
		errors.Is(err, rundb.ErrSchemaTooNew):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func codeForError(status int, err error) string {
	var problem *Problem
	if errors.As(err, &problem) && problem != nil && strings.TrimSpace(problem.Code) != "" {
		return problem.Code
	}

	switch {
	case errors.Is(err, globaldb.ErrSchemaTooNew), errors.Is(err, rundb.ErrSchemaTooNew):
		return "schema_too_new"
	default:
		return defaultCodeForStatus(status)
	}
}

func detailsForError(err error) map[string]any {
	var problem *Problem
	if errors.As(err, &problem) && problem != nil && len(problem.Details) > 0 {
		return problem.Details
	}

	var globalSchemaErr globaldb.SchemaTooNewError
	if errors.As(err, &globalSchemaErr) {
		return map[string]any{
			"database":        "globaldb",
			"current_version": globalSchemaErr.CurrentVersion,
			"known_version":   globalSchemaErr.KnownVersion,
			"remediation":     "upgrade this Compozy binary before opening the daemon catalog",
		}
	}

	var runSchemaErr rundb.SchemaTooNewError
	if errors.As(err, &runSchemaErr) {
		return map[string]any{
			"database":        "rundb",
			"current_version": runSchemaErr.CurrentVersion,
			"known_version":   runSchemaErr.KnownVersion,
			"remediation":     "upgrade this Compozy binary before opening the run database",
		}
	}

	return nil
}

func messageForError(status int, err error) string {
	var problem *Problem
	if errors.As(err, &problem) && problem != nil && strings.TrimSpace(problem.Message) != "" {
		return problem.Message
	}

	switch {
	case err == nil:
		return http.StatusText(status)
	case status >= http.StatusInternalServerError:
		if text := http.StatusText(status); text != "" {
			return text
		}
		return "internal server error"
	default:
		return err.Error()
	}
}

func defaultCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid_request"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnprocessableEntity:
		return "validation_error"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	default:
		return "internal_error"
	}
}

// RespondError writes a transport error response for one request.
func RespondError(c *gin.Context, err error) {
	if c == nil {
		return
	}

	status := statusForError(err)
	c.AbortWithStatusJSON(status, TransportError{
		RequestID: RequestIDFromContext(c.Request.Context()),
		Code:      codeForError(status, err),
		Message:   messageForError(status, err),
		Details:   detailsForError(err),
	})
}

func invalidJSONProblem(transportName string, action string, err error) error {
	return NewProblem(
		http.StatusBadRequest,
		"invalid_request",
		fmt.Sprintf("%s: %s: %v", transportName, strings.TrimSpace(action), err),
		nil,
		err,
	)
}

func validationProblem(code string, message string, details map[string]any) error {
	return NewProblem(http.StatusUnprocessableEntity, code, message, details, nil)
}

func serviceUnavailableProblem(resource string) error {
	message := strings.TrimSpace(resource)
	if message == "" {
		message = "service"
	}
	return NewProblem(
		http.StatusServiceUnavailable,
		"service_unavailable",
		fmt.Sprintf("%s unavailable", message),
		nil,
		nil,
	)
}

func requestCanceled(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}
