package core

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

func errorPayloadForMessage(message string, err error) contract.ErrorPayload {
	message = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(message))
	payload := contract.ErrorPayload{Error: message}
	switch {
	case errors.Is(err, session.ErrActiveTurnMismatch):
		payload.Code = "active_turn_mismatch"
		if fence, ok := errors.AsType[*session.ActiveTurnMismatchError](err); ok {
			payload.CurrentTurnID = fence.CurrentTurnID
		}
	case errors.Is(err, store.ErrSessionPromptIdempotencyConflict),
		errors.Is(err, store.ErrSessionPromptMessageConflict), errors.Is(err, store.ErrSessionInputMutationConflict):
		payload.Code = "send_conflict"
	case errors.Is(err, store.ErrSessionInputQueueFull):
		payload.Code = "queue_full"
	case errors.Is(err, store.ErrSessionInputSteerTextOnly):
		payload.Code = "steer_attachments_unsupported"
	case errors.Is(err, session.ErrPromptNotInProgress), errors.Is(err, session.ErrSessionNotActive),
		errors.Is(err, session.ErrSessionArchived), errors.Is(err, store.ErrSessionArchived):
		payload.Code = "session_not_promptable"
	}
	if errors.Is(err, workspace.ErrOperatorHomeWorkspace) {
		payload.Code = "workspace_home_forbidden"
	}
	if code := GatewayErrorCode(err); code != "" {
		payload.Code = code
	}
	if code := WorktreeErrorCode(err); code != "" {
		payload.Code = code
	}
	if refusal, ok := errors.AsType[*worktree.RefusalError](err); ok && refusal.Detail != "" {
		detail := diagnosticspkg.Redact(taskpkg.RedactClaimTokens(refusal.Detail))
		payload.Details = map[string]string{"detail": detail}
		if errors.Is(err, worktree.ErrBranchHeld) {
			payload.Details["worktree_path"] = detail
		}
	}
	if cause := worktree.ForgeFailureCause(err); cause != "" {
		payload.Details = map[string]string{"cause": cause}
	}
	if reason, ok := errors.AsType[*looppkg.ReasonError](err); ok {
		payload.Code = string(reason.Code)
		payload.Details = lifecycleReasonDetails(reason.Meta)
	}
	if item, ok := diagnosticspkg.ItemFromError(err); ok {
		payload.Diagnostic = &item
	} else if errors.Is(err, compozyconfig.ErrAgentNameReserved) {
		item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
			ID:            "agent.name.reserved",
			Code:          contract.CodeAgentNameReserved,
			Category:      contract.CategoryConfig,
			Title:         "Agent name is reserved",
			Message:       message,
			Severity:      contract.SeverityError,
			DataFreshness: contract.FreshnessLive,
		})
		payload.Diagnostic = &item
	} else if code := diagnosticCodeFromError(err); code != "" {
		if category, ok := contract.DiagnosticCodeCategory(code); ok {
			item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
				ID:            strings.ReplaceAll(code, "_", "."),
				Code:          code,
				Category:      category,
				Title:         "Role operation failed",
				Message:       message,
				Severity:      contract.SeverityError,
				DataFreshness: contract.FreshnessLive,
			})
			payload.Diagnostic = &item
		}
	}
	return payload
}

func lifecycleReasonDetails(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	details := make(map[string]string, len(meta))
	for key, value := range meta {
		details[key] = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(value))
	}
	return details
}

type diagnosticCodeCarrier interface {
	error
	DiagnosticCode() string
}

func diagnosticCodeFromError(err error) string {
	switch {
	case errors.Is(err, store.ErrSessionPromptDispatchIndeterminate):
		return contract.CodePromptDispatchIndeterminate
	case errors.Is(err, store.ErrSessionPromptIdempotencyConflict):
		return contract.CodePromptIdempotencyConflict
	case errors.Is(err, store.ErrSessionPromptMessageConflict):
		return contract.CodePromptMessageIdentityConflict
	}
	carrier, ok := errors.AsType[diagnosticCodeCarrier](err)
	if !ok {
		return ""
	}
	return strings.TrimSpace(carrier.DiagnosticCode())
}
