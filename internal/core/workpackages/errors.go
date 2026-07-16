package workpackages

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

var (
	// ErrInvalidReference reports a malformed or unsafe public reference.
	ErrInvalidReference = errors.New("work package reference invalid")
	// ErrContainment reports a path that escapes its allowed workspace boundary.
	ErrContainment = errors.New("work package path escapes workspace containment")
	// ErrInitiativeNotFound reports an unknown initiative.
	ErrInitiativeNotFound = errors.New("work package initiative not found")
	// ErrPackageNotFound reports an unknown stable Work Package ID.
	ErrPackageNotFound = errors.New("work package not found")
	// ErrInvalidPlan reports a present but malformed Work Package marker.
	ErrInvalidPlan = errors.New("work package plan invalid")
	// ErrSelectionRequired reports an initiative target where a package is required.
	ErrSelectionRequired = errors.New("work package selection required")
	// ErrDependenciesUnmet reports an attempted run with unmet prerequisites.
	ErrDependenciesUnmet = errors.New("work package dependencies unmet")
	// ErrCompletionConflict reports a missing, duplicate, or incompatible selected heading.
	ErrCompletionConflict = errors.New("work package completion conflict")
	// ErrPlanReadOnly reports a plan write that the filesystem refused.
	ErrPlanReadOnly = errors.New("work package plan read only")
)

// Error contains safe structured details for a Work Package domain failure.
type Error struct {
	Cause           error
	Initiative      string
	PackageID       string
	PlanPath        string
	ValidPackageIDs []string
	Issues          []Issue
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
	if e.PackageID != "" {
		parts = append(parts, fmt.Sprintf("package_id=%q", e.PackageID))
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
func newError(cause error, initiative, packageID, planPath string, issues []Issue) *Error {
	validIssues := slices.Clone(issues)
	slices.SortFunc(validIssues, compareIssue)
	return &Error{
		Cause:      cause,
		Initiative: initiative,
		PackageID:  packageID,
		PlanPath:   planPath,
		Issues:     validIssues,
	}
}

func errorText(err error) string {
	if err == nil {
		return "work package error"
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
