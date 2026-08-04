package daemon

import (
	"encoding/json"
	"errors"
	"strings"

	looppkg "github.com/compozy/compozy/internal/loop"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	loopActionFailureCode = "loop_action_failed"
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
	} else if reasonErr, ok := errors.AsType[*looppkg.ReasonError](cause); ok {
		if provided := strings.TrimSpace(string(reasonErr.Code)); provided != "" {
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
		if provided := strings.TrimSpace(failure.Code); provided != "" {
			code = provided
		}
		if provided := strings.TrimSpace(failure.Cause); provided != "" {
			message = provided
		}
		if provided := strings.TrimSpace(failure.Recovery); provided != "" {
			recovery = provided
		}
	} else if reasonErr, ok := errors.AsType[*looppkg.ReasonError](cause); ok {
		if provided := strings.TrimSpace(string(reasonErr.Code)); provided != "" {
			code = provided
		}
		switch reasonErr.Code {
		case looppkg.ReasonCodeActionTimeout:
			message = "The action exceeded its configured attempt timeout."
			recovery = "Review the action timeout and target health before retrying."
		default:
			message = "The action could not complete its runtime contract."
		}
	} else if toolErr, ok := errors.AsType[*toolspkg.ToolError](cause); ok {
		if provided := strings.TrimSpace(string(toolErr.Code)); provided != "" {
			code = provided
		}
		if toolErr.Operator != nil {
			if provided := strings.TrimSpace(toolErr.Operator.Cause); provided != "" {
				message = provided
			}
			if provided := strings.TrimSpace(toolErr.Operator.Recovery); provided != "" {
				recovery = provided
			}
		} else if strings.TrimSpace(toolErr.Message) != "" {
			message = toolErr.Message
			recovery = recoveryForToolError(toolErr.Code)
		}
	}

	return looppkg.NewActionFailure(
		code,
		message,
		recovery,
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
