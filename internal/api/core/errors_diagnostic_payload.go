package core

import (
	"errors"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/worktree"
)

func errorPayloadForMessage(message string, err error) contract.ErrorPayload {
	message = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(message))
	payload := contract.ErrorPayload{Error: message}
	if busy, ok := errors.AsType[*workspace.BusyError](err); ok {
		payload.Code = "workspace_busy"
		for _, blocker := range busy.Blockers {
			payload.Blockers = append(payload.Blockers, contract.WorkspaceArchiveBlocker{
				Kind: blocker.Kind, ID: blocker.ID, Name: blocker.Name,
			})
		}
	}
	if errors.Is(err, workspace.ErrWorkspaceArchived) {
		payload.Code = "workspace_archived"
	}
	if breadth, ok := errors.AsType[*workspaceBreadthError](err); ok {
		payload.Code = breadth.code
	}
	if errors.Is(err, compozyconfig.ErrWorkspaceConfigInvalid) {
		payload.Code = "workspace_config_invalid"
		if unknown, ok := errors.AsType[*compozyconfig.UnknownKeysError](err); ok {
			payload.ConfigPath = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(unknown.Path))
			payload.UnknownKeys = make([]string, len(unknown.Keys))
			for i, key := range unknown.Keys {
				payload.UnknownKeys[i] = diagnosticspkg.Redact(taskpkg.RedactClaimTokens(key))
			}
		}
	}
	for _, sentinel := range []error{
		session.ErrCreateScopeConflict,
		workspace.ErrWorkspaceNameReserved,
		workspace.ErrUserScopeUnsupported,
		workspace.ErrUserScopeWorktreeUnsupported,
		workspace.ErrUserScopeWorkDirInvalid,
	} {
		if errors.Is(err, sentinel) {
			payload.Code = sentinel.Error()
			break
		}
	}
	enrichOperationError(&payload, err)
	if errors.Is(err, session.ErrBoardFilterInvalid) {
		payload.Code = "session_list_query_invalid"
	}
	if code := DeriveErrorCode(err); code != "" {
		payload.Code = code
	}
	if code := SessionReparentErrorCode(err); code != "" {
		payload.Code = code
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
	enrichWorktreeError(&payload, err)
	if reason, ok := errors.AsType[*looppkg.ReasonError](err); ok {
		payload.Code = string(reason.Code)
		payload.Details = lifecycleReasonDetails(reason.Meta)
	}
	payload.Diagnostic = errorDiagnosticItem(message, err)
	enrichSessionInputError(&payload, err)
	return payload
}

func enrichOperationError(payload *contract.ErrorPayload, err error) {
	if validation, ok := errors.AsType[compozyconfig.ValidationError](
		err,
	); ok &&
		validation.Code == "shell_config_invalid" {
		payload.Code = validation.Code
	}
	switch {
	case errors.Is(err, session.ErrSessionNotFound), errors.Is(err, store.ErrSessionNotFound):
		payload.Code = "session_not_found"
	case errors.Is(err, workspace.ErrWorkspaceReserved), errors.Is(err, session.ErrWorkspaceSelectionReserved):
		payload.Code = "workspace_reserved"
	case errors.Is(err, session.ErrWorkspaceSelectionInvalid):
		payload.Code = workspaceBreadthInvalidCode
	case errors.Is(err, session.ErrListQueryInvalid):
		payload.Code = cmdPaletteInvalidRequestError
	case errors.Is(err, session.ErrSubagentArchiveFollows):
		payload.Code = "subagent_archive_follows_parent"
	case errors.Is(err, session.ErrSessionArchiveStopFailed):
		payload.Code = "session_archive_stop_failed"
	case errors.Is(err, session.ErrCompactionUnsupported):
		payload.Code = "compaction_unsupported"
	case errors.Is(err, session.ErrPromptInProgress):
		payload.Code = "session_busy"
	case errors.Is(err, settingspkg.ErrMCPServerNameTaken):
		payload.Code = "mcp_server_name_taken"
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
	case errors.Is(err, store.ErrSessionInputAgentAuthored):
		payload.Code = "input_agent_authored"
	case errors.Is(err, session.ErrSessionMessageHopLimit):
		payload.Code = "message_hop_limit"
	case errors.Is(err, session.ErrSessionMessageRateLimited):
		payload.Code = "message_rate_limited"
		payload.Details = sessionMessageRateLimitDetails(err)
	case errors.Is(err, session.ErrPromptNotInProgress), errors.Is(err, session.ErrSessionNotActive),
		errors.Is(err, session.ErrSessionArchived), errors.Is(err, store.ErrSessionArchived):
		payload.Code = "session_not_promptable"
	}
}

func enrichWorktreeError(payload *contract.ErrorPayload, err error) {
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
}

// errorDiagnosticItem returns the structured diagnostic an error carries, if any.
func errorDiagnosticItem(message string, err error) *contract.DiagnosticItem {
	if item, ok := diagnosticspkg.ItemFromError(err); ok {
		return &item
	}
	if errors.Is(err, compozyconfig.ErrAgentNameReserved) {
		item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
			ID:            "agent.name.reserved",
			Code:          contract.CodeAgentNameReserved,
			Category:      contract.CategoryConfig,
			Title:         "Agent name is reserved",
			Message:       message,
			Severity:      contract.SeverityError,
			DataFreshness: contract.FreshnessLive,
		})
		return &item
	}
	code := diagnosticCodeFromError(err)
	if code == "" {
		return nil
	}
	category, ok := contract.DiagnosticCodeCategory(code)
	if !ok {
		return nil
	}
	item := diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
		ID:            strings.ReplaceAll(code, "_", "."),
		Code:          code,
		Category:      category,
		Title:         "Role operation failed",
		Message:       message,
		Severity:      contract.SeverityError,
		DataFreshness: contract.FreshnessLive,
	})
	return &item
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

// sessionMessageRateLimitDetails exposes the refused budget so callers can back off without parsing text.
func sessionMessageRateLimitDetails(err error) map[string]string {
	limit, ok := errors.AsType[*session.MessageRateLimitError](err)
	if !ok {
		return nil
	}
	return map[string]string{
		"target_session_id":   limit.TargetSessionID,
		"limit":               strconv.Itoa(limit.Limit),
		"window":              limit.Window.String(),
		"retry_after_seconds": strconv.FormatInt(int64(limit.RetryAfter.Seconds()), 10),
	}
}
