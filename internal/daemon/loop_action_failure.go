package daemon

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	looppkg "github.com/compozy/agh/internal/loop"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	loopActionFailureCode      = "loop_action_failed"
	loopActionFailureTextLimit = 2 * 1024
)

type loopActionFailureMetadata struct {
	ReasonCode string                `json:"reason_code"`
	Reason     string                `json:"reason"`
	Failure    looppkg.ActionFailure `json:"failure"`
}

type loopActionReasonCodeProvider interface {
	error
	loopActionReasonCode() string
}

func marshalLoopActionFailureMetadata(reason string, cause error) ([]byte, error) {
	reasonCode := loopActionFailureCode
	if provider, ok := errors.AsType[loopActionReasonCodeProvider](cause); ok {
		if provided := strings.TrimSpace(provider.loopActionReasonCode()); provided != "" {
			reasonCode = provided
		}
	}
	return json.Marshal(loopActionFailureMetadata{
		ReasonCode: reasonCode,
		Reason:     strings.TrimSpace(reason),
		Failure:    operatorSafeActionFailure(cause),
	})
}

func operatorSafeActionFailure(cause error) looppkg.ActionFailure {
	code := loopActionFailureCode
	message := "The action failed before producing an output."
	recovery := "Review the action input and required services, then retry the run."

	if provider, ok := errors.AsType[looppkg.SafeActionFailureProvider](cause); ok {
		failure := provider.SafeActionFailure()
		if strings.TrimSpace(failure.Code) != "" && strings.TrimSpace(failure.Cause) != "" &&
			strings.TrimSpace(failure.Recovery) != "" {
			code = failure.Code
			message = failure.Cause
			recovery = failure.Recovery
		}
	} else if toolErr, ok := errors.AsType[*toolspkg.ToolError](cause); ok {
		code = strings.TrimSpace(string(toolErr.Code))
		if code == "" {
			code = loopActionFailureCode
		}
		if toolErr.Operator != nil {
			message = toolErr.Operator.Cause
			recovery = toolErr.Operator.Recovery
		} else if strings.TrimSpace(toolErr.Message) != "" {
			message = toolErr.Message
			recovery = recoveryForToolError(toolErr.Code)
		}
	}

	return looppkg.NewActionFailure(
		code,
		diagnostics.RedactAndBound(message, loopActionFailureTextLimit),
		diagnostics.RedactAndBound(recovery, loopActionFailureTextLimit),
	)
}

func recoveryForToolError(code toolspkg.ErrorCode) string {
	switch code {
	case toolspkg.ErrorCodeInvalidInput:
		return "Correct the action input or required workspace state, then retry the run."
	case toolspkg.ErrorCodeUnavailable:
		return "Restore the required tool or extension, then retry the run."
	case toolspkg.ErrorCodeDenied, toolspkg.ErrorCodeApprovalRequired:
		return "Update the tool permission or approval, then retry the run."
	case toolspkg.ErrorCodeTimedOut, toolspkg.ErrorCodeCanceled:
		return "Check the tool runtime and run the action again when it is available."
	default:
		return "Check the tool or extension health and configuration, then retry the run."
	}
}
