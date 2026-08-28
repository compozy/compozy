package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

// StatusForToolError maps registry failures to stable transport statuses.
func StatusForToolError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if validation, ok := errors.AsType[*toolspkg.ValidationError](err); ok && validation != nil {
		return http.StatusBadRequest
	}
	if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok {
		return statusForToolCode(toolErr.Code, toolErr.ReasonCodes)
	}
	switch {
	case errors.Is(err, toolspkg.ErrToolInvalidInput):
		return http.StatusBadRequest
	case errors.Is(err, toolspkg.ErrToolNotFound):
		return http.StatusNotFound
	case errors.Is(err, toolspkg.ErrToolDenied):
		return http.StatusForbidden
	case errors.Is(err, toolspkg.ErrToolApprovalRequired):
		return http.StatusAccepted
	case errors.Is(err, toolspkg.ErrToolConflict):
		return http.StatusConflict
	case errors.Is(err, toolspkg.ErrToolUnavailable),
		errors.Is(err, toolspkg.ErrToolResultTooLarge):
		return http.StatusUnprocessableEntity
	case errors.Is(err, toolspkg.ErrToolResultPersistence):
		return http.StatusInsufficientStorage
	case errors.Is(err, toolspkg.ErrToolBackendFailed),
		errors.Is(err, toolspkg.ErrToolCanceled),
		errors.Is(err, toolspkg.ErrToolTimedOut):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func statusForToolCode(code toolspkg.ErrorCode, reasons []toolspkg.ReasonCode) int {
	if status, ok := TerminalDomainStatus(terminalpkg.ErrorCode(code)); ok {
		return status
	}
	switch code {
	case toolspkg.ErrorCodeInvalidInput:
		return http.StatusBadRequest
	case toolspkg.ErrorCodeNotFound:
		return http.StatusNotFound
	case toolspkg.ErrorCodeDenied:
		return http.StatusForbidden
	case toolspkg.ErrorCodeApprovalRequired:
		if hasToolReason(reasons, toolspkg.ReasonApprovalTokenExpired, toolspkg.ReasonApprovalTokenMismatch,
			toolspkg.ReasonApprovalTokenReplayed) {
			return http.StatusForbidden
		}
		return http.StatusAccepted
	case toolspkg.ErrorCodeConflict:
		return http.StatusConflict
	case toolspkg.ErrorCodeUnavailable,
		toolspkg.ErrorCodeResultTooLarge,
		toolspkg.ErrorCodeModelNotFound,
		toolspkg.ErrorCodeReasoningEffortUnsupported,
		toolspkg.ErrorCodeAgentNameReserved:
		return http.StatusUnprocessableEntity
	case toolspkg.ErrorCodeResultPersistenceFailed:
		return http.StatusInsufficientStorage
	case toolspkg.ErrorCodeBackendFailed, toolspkg.ErrorCodeCanceled, toolspkg.ErrorCodeTimedOut:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

func hasToolReason(reasons []toolspkg.ReasonCode, want ...toolspkg.ReasonCode) bool {
	for _, reason := range reasons {
		if slices.Contains(want, reason) {
			return true
		}
	}
	return false
}

// respondToolError serializes stable tool errors without backend error text.
func (h *BaseHandlers) respondToolError(c *gin.Context, err error) {
	RespondToolError(c, err, h.MaskInternalErrors)
}

// RespondToolError serializes stable tool errors without backend error text.
func RespondToolError(c *gin.Context, err error, maskInternal bool) {
	status := StatusForToolError(err)
	response := ToolErrorResponseForError(err, status, maskInternal)
	c.JSON(status, response)
}

// ToolErrorResponseForError builds the stable tool error response payload.
func ToolErrorResponseForError(err error, status int, maskInternal bool) contract.ToolErrorResponse {
	payload := contract.ToolErrorPayload{
		Code:    toolspkg.ErrorCodeBackendFailed,
		Message: http.StatusText(status),
	}
	if err == nil {
		payload.Code = toolErrorCodeForStatus(status)
		payload.Message = http.StatusText(status)
	} else if toolErr, ok := errors.AsType[*toolspkg.ToolError](err); ok {
		payload.Code = toolErr.Code
		payload.ToolID = toolErr.ToolID
		payload.ReasonCodes = append([]toolspkg.ReasonCode(nil), toolErr.ReasonCodes...)
		payload.Message = safeToolErrorMessage(status, toolErr.Code, payload.ReasonCodes)
		payload.Layer = toolErrorLayer(toolErr.ReasonCodes)
		payload.Details = contract.ToolOperatorFailureDetails(toolErr.Operator)
		if terminalErr, terminal := errors.AsType[*terminalpkg.Error](err); terminal &&
			contract.IsTerminalErrorCode(contract.TerminalErrorCode(terminalErr.Code)) {
			for key, value := range contract.TerminalErrorToolDetailsFromDomain(terminalErr) {
				if payload.Details == nil {
					payload.Details = make(map[string]json.RawMessage)
				}
				payload.Details[key] = value
			}
		}
		if toolErr.PartialResult != nil {
			partial := *toolErr.PartialResult
			payload.PartialResult = &partial
		}
	} else {
		payload.Code = toolErrorCodeForStatus(status)
		if reason, ok := toolspkg.ReasonOf(err); ok {
			payload.ReasonCodes = []toolspkg.ReasonCode{reason}
			payload.Layer = toolErrorLayer(payload.ReasonCodes)
		}
		payload.Message = safeToolErrorMessage(status, payload.Code, payload.ReasonCodes)
	}
	if maskInternal && status >= http.StatusInternalServerError &&
		payload.Code != toolspkg.ErrorCodeResultPersistenceFailed {
		payload.Message = http.StatusText(status)
	}
	if strings.TrimSpace(payload.Message) == "" {
		payload.Message = http.StatusText(status)
	}
	return contract.ToolErrorResponse{Error: payload}
}

func safeToolErrorMessage(status int, code toolspkg.ErrorCode, reasons []toolspkg.ReasonCode) string {
	switch code {
	case toolspkg.ErrorCodeNotFound:
		if slices.Contains(reasons, toolspkg.ReasonConfigPathNotFound) {
			return "config path not found"
		}
		return "tool not found"
	case toolspkg.ErrorCodeConflict:
		return "tool conflict"
	case toolspkg.ErrorCodeUnavailable:
		return "tool unavailable"
	case toolspkg.ErrorCodeDenied:
		return "tool invocation denied"
	case toolspkg.ErrorCodeApprovalRequired:
		return "tool approval required"
	case toolspkg.ErrorCodeInvalidInput:
		return "invalid tool request"
	case toolspkg.ErrorCodeModelNotFound:
		return "provider model not found"
	case toolspkg.ErrorCodeReasoningEffortUnsupported:
		return "reasoning effort unsupported"
	case toolspkg.ErrorCodeResultTooLarge:
		return "tool result too large"
	case toolspkg.ErrorCodeResultPersistenceFailed:
		return "tool result could not be retained"
	case toolspkg.ErrorCodeCanceled:
		return "tool call canceled"
	case toolspkg.ErrorCodeTimedOut:
		return "tool call timed out"
	case toolspkg.ErrorCodeBackendFailed:
		return "tool backend failed"
	default:
		message := http.StatusText(status)
		if strings.TrimSpace(message) == "" {
			return "tool request failed"
		}
		return message
	}
}

func toolErrorCodeForStatus(status int) toolspkg.ErrorCode {
	switch status {
	case http.StatusBadRequest:
		return toolspkg.ErrorCodeInvalidInput
	case http.StatusNotFound:
		return toolspkg.ErrorCodeNotFound
	case http.StatusForbidden:
		return toolspkg.ErrorCodeDenied
	case http.StatusAccepted:
		return toolspkg.ErrorCodeApprovalRequired
	case http.StatusConflict:
		return toolspkg.ErrorCodeConflict
	case http.StatusUnprocessableEntity:
		return toolspkg.ErrorCodeUnavailable
	case http.StatusInsufficientStorage:
		return toolspkg.ErrorCodeResultPersistenceFailed
	default:
		return toolspkg.ErrorCodeBackendFailed
	}
}

func toolErrorLayer(reasons []toolspkg.ReasonCode) string {
	for _, reason := range reasons {
		switch reason {
		case toolspkg.ReasonSourceDisabled,
			toolspkg.ReasonMCPAuthUnconfigured,
			toolspkg.ReasonMCPAuthRequired,
			toolspkg.ReasonMCPAuthExpired,
			toolspkg.ReasonMCPAuthInvalid,
			toolspkg.ReasonMCPAuthRefreshFailed:
			return "source_policy"
		case toolspkg.ReasonSessionDenied:
			return "session_lineage"
		case toolspkg.ReasonPolicyDenied, toolspkg.ReasonVisibilityDenied:
			return "registry_policy"
		case toolspkg.ReasonHookDenied:
			return "hook"
		case toolspkg.ReasonApprovalRequired,
			toolspkg.ReasonApprovalUnreachable,
			toolspkg.ReasonApprovalTimedOut,
			toolspkg.ReasonApprovalCanceled,
			toolspkg.ReasonApprovalTokenMissing,
			toolspkg.ReasonApprovalTokenExpired,
			toolspkg.ReasonApprovalTokenMismatch,
			toolspkg.ReasonApprovalTokenReplayed:
			return "approval"
		case toolspkg.ReasonBackendUnhealthy,
			toolspkg.ReasonBackendNotExecutable,
			toolspkg.ReasonConflictedID,
			toolspkg.ReasonConflictedSanitizedName:
			return "availability"
		}
	}
	return ""
}
