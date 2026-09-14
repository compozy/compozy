package cli

import (
	"encoding/json"
	"errors"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func marshalStructuredPayload(args []string, payload any) ([]byte, bool) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, false
	}
	switch requestedOutputFormat(args) {
	case OutputJSON:
		return encoded, true
	case OutputJSONL:
		return append(encoded, '\n'), true
	default:
		return nil, false
	}
}

func marshalStructuredExecutionError(args []string, err error) ([]byte, bool) {
	if payload, ok := marshalSkillExposureExecutionError(args, err); ok {
		return payload, true
	}
	if payload, ok := marshalMarketplaceSourceExecutionError(args, err); ok {
		return payload, true
	}
	if payload, ok := marshalSkillSourceExecutionError(args, err); ok {
		return payload, true
	}
	if profileErr, ok := errors.AsType[interface {
		error
		profileErrorPayload() contract.ProfileErrorPayload
	}](err); ok {
		return marshalProfileExecutionError(args, profileErr.profileErrorPayload())
	}
	if appErr, ok := errors.AsType[*appCommandError](err); ok {
		return marshalAppCommandExecutionError(args, appErr)
	}
	if extensionErr, ok := errors.AsType[interface {
		error
		extensionOperationErrorPayload() contract.ExtensionOperationErrorPayload
	}](err); ok {
		return marshalExtensionOperationExecutionError(args, extensionErr.extensionOperationErrorPayload())
	}
	if validationErr, ok := errors.AsType[interface {
		error
		extensionValidationErrorPayload() contract.ExtensionValidationErrorPayload
	}](err); ok {
		return marshalExtensionValidationExecutionError(args, validationErr.extensionValidationErrorPayload())
	}
	if windowManagerErr, ok := errors.AsType[interface {
		error
		windowManagerErrorPayload() contract.WindowManagerErrorPayload
	}](err); ok {
		return marshalWindowManagerExecutionError(args, windowManagerErr.windowManagerErrorPayload())
	}
	if worktreeRemovalErr, ok := errors.AsType[interface {
		error
		worktreeRemovalErrorPayload() contract.WorktreeRemovalRefusalPayload
	}](err); ok {
		return marshalWorktreeRemovalExecutionError(args, worktreeRemovalErr.worktreeRemovalErrorPayload())
	}
	if goalErr, ok := errors.AsType[*goalCommandAPIError](err); ok {
		return marshalGoalCommandExecutionError(args, goalErr)
	}
	if terminalErr, ok := errors.AsType[interface {
		error
		TerminalErrorEnvelope() contract.TerminalErrorResponse
	}](err); ok {
		return marshalTerminalExecutionError(args, terminalErr.TerminalErrorEnvelope())
	}
	if apiErr, ok := errors.AsType[interface {
		error
		errorPayload() contract.ErrorPayload
	}](err); ok {
		return marshalDaemonAPIExecutionError(args, apiErr.errorPayload())
	}
	if terminalErr, ok := errors.AsType[*terminalpkg.Error](err); ok &&
		contract.IsTerminalErrorCode(contract.TerminalErrorCode(terminalErr.Code)) {
		return marshalTerminalExecutionError(args, terminalExecutionErrorPayload(terminalErr))
	}
	if !isStructuredAgentCommandError(err) {
		return marshalDiagnosticExecutionError(args, err)
	}

	return marshalAgentIdentityExecutionError(args, err)
}

func marshalAgentIdentityExecutionError(args []string, err error) ([]byte, bool) {
	switch requestedOutputFormat(args) {
	case OutputJSON:
		payload, marshalErr := agentidentity.MarshalErrorJSON(err)
		if marshalErr != nil {
			return nil, false
		}
		return payload, true
	case OutputJSONL:
		payload, marshalErr := agentidentity.MarshalErrorJSONL(err)
		if marshalErr != nil {
			return nil, false
		}
		return payload, true
	default:
		return nil, false
	}
}
