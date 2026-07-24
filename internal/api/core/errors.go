package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/taskgroups"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/rundb"
)

const codeSchemaTooNew = string(contract.CodeSchemaTooNew)

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

	if status, ok := statusForKnownClientError(err); ok {
		return status
	}

	if status, ok := statusForArtifactError(err); ok {
		return status
	}
	return statusForWorkflowConflict(err)
}

func statusForKnownClientError(err error) (int, bool) {
	switch {
	case errors.Is(err, os.ErrNotExist),
		errors.Is(err, globaldb.ErrWorkspaceNotFound),
		errors.Is(err, globaldb.ErrWorkflowNotFound),
		errors.Is(err, globaldb.ErrRunNotFound),
		errors.Is(err, model.ErrJobControlNotFound),
		errors.Is(err, taskgroups.ErrTaskGroupNotFound),
		errors.Is(err, taskgroups.ErrInitiativeNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, model.ErrJobControlMessageRequired):
		return http.StatusBadRequest, true
	case errors.Is(err, model.ErrJobControlMessageTooLarge):
		return http.StatusRequestEntityTooLarge, true
	case errors.Is(err, taskgroups.ErrPlanReadOnly):
		return http.StatusForbidden, true
	case errors.Is(err, taskgroups.ErrDependenciesUnmet),
		errors.Is(err, taskgroups.ErrCompletionConflict):
		return http.StatusConflict, true
	case errors.Is(err, taskgroups.ErrInvalidPlan),
		errors.Is(err, taskgroups.ErrSelectionRequired),
		errors.Is(err, taskgroups.ErrInvalidReference),
		errors.Is(err, taskgroups.ErrContainment),
		errors.Is(err, tasks.ErrLegacyTaskMetadata),
		errors.Is(err, tasks.ErrV1TaskMetadata),
		errors.Is(err, reviews.ErrLegacyReviewMetadata):
		return http.StatusUnprocessableEntity, true
	default:
		return 0, false
	}
}

func statusForArtifactError(err error) (int, bool) {
	var taskParseErr *tasks.ArtifactParseError
	if errors.As(err, &taskParseErr) {
		return http.StatusUnprocessableEntity, true
	}
	var reviewParseErr *reviews.ArtifactParseError
	if errors.As(err, &reviewParseErr) {
		return http.StatusUnprocessableEntity, true
	}
	return 0, false
}

func statusForWorkflowConflict(err error) int {
	switch {
	case errors.Is(err, globaldb.ErrWorkspaceHasActiveRuns),
		errors.Is(err, model.ErrJobControlConflict),
		errors.Is(err, globaldb.ErrWorkflowArchived),
		errors.Is(err, globaldb.ErrWorkflowHasActiveRuns),
		errors.Is(err, globaldb.ErrWorkflowNotArchivable),
		errors.Is(err, globaldb.ErrWorkflowSlugConflict),
		errors.Is(err, globaldb.ErrWorkflowSyncInvalid),
		errors.Is(err, globaldb.ErrRunAlreadyExists),
		errors.Is(err, globaldb.ErrSchemaTooNew),
		errors.Is(err, rundb.ErrSchemaTooNew):
		if errors.Is(err, globaldb.ErrWorkflowSyncInvalid) {
			return http.StatusUnprocessableEntity
		}
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
	case errors.Is(err, taskgroups.ErrTaskGroupNotFound),
		errors.Is(err, taskgroups.ErrInitiativeNotFound):
		return "task_group_not_found"
	case errors.Is(err, taskgroups.ErrDependenciesUnmet):
		return "task_group_dependencies_unmet"
	case errors.Is(err, taskgroups.ErrCompletionConflict):
		return "task_group_completion_conflict"
	case errors.Is(err, taskgroups.ErrInvalidPlan):
		return "task_group_plan_invalid"
	case errors.Is(err, taskgroups.ErrSelectionRequired):
		return "task_group_selection_required"
	case errors.Is(err, taskgroups.ErrPlanReadOnly):
		return "task_group_plan_read_only"
	case errors.Is(err, taskgroups.ErrInvalidReference), errors.Is(err, taskgroups.ErrContainment):
		return "task_group_invalid_reference"
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

	var taskGroupErr *taskgroups.Error
	if errors.As(err, &taskGroupErr) && taskGroupErr != nil {
		details := make(map[string]any)
		if initiative := strings.TrimSpace(taskGroupErr.Initiative); initiative != "" {
			details["initiative_slug"] = initiative
		}
		if taskGroupID := strings.TrimSpace(taskGroupErr.TaskGroupID); taskGroupID != "" {
			details["task_group_id"] = taskGroupID
		}
		if len(taskGroupErr.ValidTaskGroupIDs) > 0 {
			details["valid_task_group_ids"] = append([]string(nil), taskGroupErr.ValidTaskGroupIDs...)
		}
		if len(taskGroupErr.Issues) > 0 {
			issues := make([]map[string]string, 0, len(taskGroupErr.Issues))
			for _, issue := range taskGroupErr.Issues {
				issues = append(issues, map[string]string{
					"field":   strings.TrimSpace(issue.Field),
					"message": strings.TrimSpace(issue.Message),
				})
			}
			details["issues"] = issues
		}
		if taskGroupErr.PlanPath != "" {
			details["plan"] = taskgroups.ManifestFileName
		}
		if len(details) > 0 {
			return details
		}
	}

	return nil
}

func messageForError(status int, err error) string {
	return contract.MessageForStatus(status, err, true)
}

func defaultCodeForStatus(status int) string {
	switch status {
	case http.StatusPreconditionFailed:
		return "precondition_failed"
	default:
		return contract.DefaultCodeForStatus(status)
	}
}

// RespondError writes a transport error response for one request.
func RespondError(c *gin.Context, err error) {
	if c == nil {
		return
	}

	status := statusForError(err)
	payload := contract.TransportErrorEnvelope(
		RequestIDFromContext(c.Request.Context()),
		status,
		err,
		detailsForError(err),
		true,
	)
	payload.Code = codeForError(status, err)
	c.AbortWithStatusJSON(
		status,
		payload,
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

func workspaceContextProblem(code string, message string, details map[string]any, err error) error {
	return NewProblem(http.StatusPreconditionFailed, code, message, details, err)
}

func WorkspacePathMissingProblem(workspaceID string, rootDir string, err error) error {
	return workspaceContextProblem(
		"workspace_path_missing",
		"workspace path is missing",
		map[string]any{
			"workspace": strings.TrimSpace(workspaceID),
			"root_dir":  strings.TrimSpace(rootDir),
		},
		err,
	)
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
