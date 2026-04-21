package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
)

type TransportError = contract.TransportError
type Problem = contract.Problem

func NewProblem(status int, code string, message string, details map[string]any, err error) *Problem {
	return contract.NewProblem(status, code, message, details, err)
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
		errors.Is(err, globaldb.ErrWorkflowArchived),
		errors.Is(err, globaldb.ErrWorkflowHasActiveRuns),
		errors.Is(err, globaldb.ErrWorkflowNotArchivable),
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
		return string(contract.CodeSchemaTooNew)
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
	return contract.MessageForStatus(status, err, true)
}

func defaultCodeForStatus(status int) string {
	return contract.DefaultCodeForStatus(status)
}

// RespondError writes a transport error response for one request.
func RespondError(c *gin.Context, err error) {
	if c == nil {
		return
	}

	status := statusForError(err)
	c.AbortWithStatusJSON(
		status,
		contract.TransportErrorEnvelope(
			RequestIDFromContext(c.Request.Context()),
			status,
			err,
			detailsForError(err),
			true,
		),
	)
}

func invalidJSONProblem(transportName string, action string, err error) error {
	return NewProblem(
		http.StatusBadRequest,
		string(contract.CodeInvalidRequest),
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
		string(contract.CodeServiceUnavailable),
		fmt.Sprintf("%s unavailable", message),
		nil,
		nil,
	)
}

func requestCanceled(ctx context.Context) bool {
	return ctx != nil && ctx.Err() != nil
}
