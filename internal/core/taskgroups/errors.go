package taskgroups

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrInvalidReference reports a malformed or unsafe public reference.
	ErrInvalidReference = errors.New("task group reference invalid")
	// ErrContainment reports a path that escapes its allowed workspace boundary.
	ErrContainment = errors.New("task group path escapes workspace containment")
	// ErrInitiativeNotFound reports an unknown initiative.
	ErrInitiativeNotFound = errors.New("task group initiative not found")
	// ErrTaskGroupNotFound reports an unknown stable Task Group ID.
	ErrTaskGroupNotFound = errors.New("task group not found")
	// ErrInvalidPlan reports a present but malformed Task Group marker.
	ErrInvalidPlan = errors.New("task group plan invalid")
	// ErrSelectionRequired reports an initiative target where a task group is required.
	ErrSelectionRequired = errors.New("task group selection required")
	// ErrDependenciesUnmet reports an attempted run with unmet prerequisites.
	ErrDependenciesUnmet = errors.New("task group dependencies unmet")
	// ErrCompletionConflict reports a missing, duplicate, or incompatible selected heading.
	ErrCompletionConflict = errors.New("task group completion conflict")
	// ErrPlanReadOnly reports a plan write that the filesystem refused.
	ErrPlanReadOnly = errors.New("task group plan read only")
)

// Error contains safe structured details for a Task Group domain failure.
type Error struct {
	Cause             error
	Initiative        string
	TaskGroupID       string
	PlanPath          string
	ValidTaskGroupIDs []string
	Issues            []Issue
}

// Error returns a concise diagnostic with the typed cause first.
func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	parts := []string{errorText(e.Cause)}
	if e.Initiative != "" {
		parts = append(parts, fmt.Sprintf("initiative=%q", e.Initiative))
	}
	if e.TaskGroupID != "" {
		parts = append(parts, fmt.Sprintf("task_group_id=%q", e.TaskGroupID))
	}
	if len(e.Issues) > 0 {
		parts = append(parts, e.Issues[0].Field+": "+e.Issues[0].Message)
	}
	return strings.Join(parts, ": ")
}

// Unwrap exposes the typed cause to errors.Is and errors.As callers.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// newError creates a deterministic typed domain error.
func newError(cause error, initiative, taskGroupID, planPath string, issues []Issue) *Error {
	validIssues := slices.Clone(issues)
	slices.SortFunc(validIssues, compareIssue)
	return &Error{
		Cause:       cause,
		Initiative:  initiative,
		TaskGroupID: taskGroupID,
		PlanPath:    planPath,
		Issues:      validIssues,
	}
}

func errorText(err error) string {
	if err == nil {
		return "task group error"
	}
	return err.Error()
}

func compareIssue(left, right Issue) int {
	if result := strings.Compare(left.Path, right.Path); result != 0 {
		return result
	}
	if result := strings.Compare(left.Field, right.Field); result != 0 {
		return result
	}
	return strings.Compare(left.Message, right.Message)
}
