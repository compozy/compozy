package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/admission"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const statusClientClosedRequest = 499

// WorkspaceGetter resolves registered workspaces by reference.
type WorkspaceGetter interface {
	Get(ctx context.Context, ref string) (workspacepkg.Workspace, error)
}

// ValidateCreateSessionRequest enforces the shared session workspace contract.
func validateCreateSessionRequest(prefix string, workspaceRef string, workspacePath string) error {
	trimmedRef := strings.TrimSpace(workspaceRef)
	trimmedPath := strings.TrimSpace(workspacePath)

	switch {
	case trimmedRef == "" && trimmedPath == "":
		return prefixedError(prefix, "workspace or workspace_path is required")
	case trimmedRef != "" && trimmedPath != "":
		return prefixedError(prefix, "workspace and workspace_path are mutually exclusive")
	case trimmedPath != "":
		return validateAbsolutePathInternal(prefix, "workspace_path", trimmedPath)
	default:
		return nil
	}
}

// LookupWorkspaceID resolves a workspace reference into a stable workspace ID.
func lookupWorkspaceID(ctx context.Context, prefix string, workspaces WorkspaceGetter, ref string) (string, error) {
	if workspaces == nil {
		return "", prefixedError(prefix, "workspace resolver is required")
	}

	workspace, err := workspaces.Get(ctx, ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(workspace.ID), nil
}

// FilterSessionInfosByWorkspaceID filters the session info list by workspace ID.
func filterSessionInfosByWorkspaceIDInternal(infos []*session.Info, workspaceID string) []*session.Info {
	trimmedID := strings.TrimSpace(workspaceID)
	if trimmedID == "" {
		return infos
	}

	filtered := make([]*session.Info, 0, len(infos))
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.WorkspaceID) != trimmedID {
			continue
		}
		filtered = append(filtered, info)
	}
	return filtered
}

// ValidateAbsolutePath ensures a field carries an absolute filesystem path.
func validateAbsolutePathInternal(prefix string, field string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return prefixedError(prefix, field+" is required")
	}
	if !filepath.IsAbs(trimmed) {
		return prefixedError(prefix, field+" must be an absolute path")
	}
	return nil
}

// ValidateAbsolutePaths ensures every populated entry in a list is absolute.
func validateAbsolutePathsInternal(prefix string, field string, values []string) error {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if !filepath.IsAbs(trimmed) {
			return prefixedError(prefix, field+" entries must be absolute paths")
		}
	}
	return nil
}

// TrimStringSlice trims all entries while preserving order and cardinality.
func trimStringSliceInternal(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		trimmed = append(trimmed, strings.TrimSpace(value))
	}
	return trimmed
}

// StatusForWorkspaceError maps workspace-domain errors to transport statuses.
func statusForWorkspaceError(err error) int {
	switch {
	case errors.Is(err, context.Canceled):
		return statusClientClosedRequest
	case errors.Is(err, workspacepkg.ErrWorkspaceNotFound):
		return http.StatusNotFound
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone
	case errors.Is(err, workspacepkg.ErrWorkspaceNameTaken),
		errors.Is(err, workspacepkg.ErrWorkspacePathTaken),
		errors.Is(err, workspacepkg.ErrWorkspaceHasSessions),
		errors.Is(err, workspacepkg.ErrWorkspaceHasActiveSessions):
		return http.StatusConflict
	case errors.Is(err, compozyconfig.ErrSandboxProfileNotFound):
		return http.StatusBadRequest
	case errors.Is(err, workspacepkg.ErrWorkspaceResolverUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// StatusForSessionError maps session and workspace-domain errors to transport statuses.
func statusForSessionError(err error) int {
	if status, ok := statusForSessionParticipationError(err); ok {
		return status
	}
	switch {
	case errors.Is(err, context.Canceled):
		return statusClientClosedRequest
	case errors.Is(err, admission.ErrDraining):
		return http.StatusServiceUnavailable
	}
	if status, ok := statusForSessionLookupError(err); ok {
		return status
	}
	if status, ok := statusForSessionValidationError(err); ok {
		return status
	}
	if status, ok := statusForSessionConflictError(err); ok {
		return status
	}
	if status, ok := statusForSessionAvailabilityError(err); ok {
		return status
	}
	return http.StatusInternalServerError
}

func statusForSessionLookupError(err error) (int, bool) {
	switch {
	case errors.Is(err, session.ErrSessionNotFound),
		errors.Is(err, store.ErrSessionNotFound),
		errors.Is(err, store.ErrSessionInputQueueEntryNotFound),
		errors.Is(err, os.ErrNotExist),
		errors.Is(err, workspacepkg.ErrWorkspaceNotFound):
		return http.StatusNotFound, true
	case errors.Is(err, workspacepkg.ErrWorkspaceRootMissing):
		return http.StatusGone, true
	default:
		return 0, false
	}
}

func statusForSessionValidationError(err error) (int, bool) {
	switch {
	case errors.Is(err, workspacepkg.ErrAgentNotAvailable),
		errors.Is(err, compozyconfig.ErrSandboxProfileNotFound),
		errors.Is(err, compozyconfig.ErrProviderUnavailable),
		errors.Is(err, session.ErrInvalidRuntimeOverride),
		errors.Is(err, session.ErrInvalidPermissionDecision),
		errors.Is(err, session.ErrSessionNotActive):
		return http.StatusBadRequest, true
	case isProviderNegotiationFailure(err),
		isProviderAuthFailure(err),
		isReasoningEffortUnsupportedFailure(err):
		return http.StatusUnprocessableEntity, true
	default:
		return 0, false
	}
}

func statusForSessionConflictError(err error) (int, bool) {
	switch {
	case errors.Is(err, session.ErrPromptInProgress),
		errors.Is(err, session.ErrRuntimeSelectionConflict),
		errors.Is(err, session.ErrPromptNotInProgress),
		errors.Is(err, session.ErrActiveTurnMismatch),
		errors.Is(err, store.ErrSessionInputQueueEntryNotQueued),
		errors.Is(err, store.ErrSessionInputMutationConflict),
		errors.Is(err, session.ErrPendingPermissionNotFound),
		errors.Is(err, session.ErrPendingPermissionConflict),
		errors.Is(err, store.ErrSessionAttachLocked),
		errors.Is(err, store.ErrSessionNotAttachable),
		errors.Is(err, workspacepkg.ErrWorkspaceNameTaken),
		errors.Is(err, workspacepkg.ErrWorkspacePathTaken),
		errors.Is(err, store.ErrSessionPromptAdmissionInProgress),
		errors.Is(err, store.ErrSessionPromptIdempotencyConflict),
		errors.Is(err, store.ErrSessionPromptMessageConflict),
		errors.Is(err, store.ErrSessionPromptDispatchIndeterminate):
		return http.StatusConflict, true
	default:
		return 0, false
	}
}

func statusForSessionAvailabilityError(err error) (int, bool) {
	switch {
	case errors.Is(err, store.ErrSessionInputQueueFull):
		return http.StatusRequestEntityTooLarge, true
	case errors.Is(err, transcript.ErrProjectionIncompatible):
		return http.StatusServiceUnavailable, true
	case errors.Is(err, transcript.ErrProjectionCorrupt):
		return http.StatusInternalServerError, true
	default:
		return 0, false
	}
}

func statusForSessionParticipationError(err error) (int, bool) {
	switch {
	case errors.Is(err, participation.ErrStrategyInvalid),
		errors.Is(err, participation.ErrStrategyChannelConflict):
		return http.StatusBadRequest, true
	case errors.Is(err, participation.ErrChannelUnknown):
		return http.StatusNotFound, true
	case errors.Is(err, participation.ErrUnavailable):
		return http.StatusConflict, true
	case errors.Is(err, participation.ErrLiveUnsupported):
		return http.StatusUnprocessableEntity, true
	default:
		return 0, false
	}
}

func statusForCreateSessionValidationError(err error) int {
	if isReasoningEffortUnsupportedFailure(err) {
		return http.StatusUnprocessableEntity
	}
	return http.StatusBadRequest
}

func isReasoningEffortUnsupportedFailure(err error) bool {
	item, ok := diagnostics.ItemFromError(err)
	return ok && item.Code == diagnosticcontract.CodeReasoningEffortUnsupported
}

func isProviderAuthFailure(err error) bool {
	failure, ok := errors.AsType[*acp.FailureError](err)
	return ok && failure != nil && failure.Kind == store.FailureProviderAuth
}

func isProviderNegotiationFailure(err error) bool {
	negotiationErr, ok := errors.AsType[*acp.NegotiationError](err)
	return ok && negotiationErr != nil
}

func prefixedError(prefix string, message string) error {
	label := strings.TrimSpace(prefix)
	if label == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %s", label, message)
}
