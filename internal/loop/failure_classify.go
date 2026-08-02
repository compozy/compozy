package loop

import (
	"strings"
	"time"

	"github.com/compozy/compozy/internal/tools"
)

// ClassifyNodeFailure classifies immutable evidence without IO or shared state.
func ClassifyNodeFailure(ev FailureEvidence) ClassifiedFailure {
	class := classifyFailureClass(ev)
	code, cause, hint := failureEvidenceDetail(ev, class)
	result := ClassifiedFailure{
		Class:         class,
		Code:          code,
		Cause:         cause,
		Hint:          hint,
		Target:        strings.TrimSpace(ev.Target),
		RetryEligible: class.RetryEligible(),
	}
	if retryAfter := clampedRetryAfter(ev.RetryAfter, ev.BackoffMax); retryAfter != nil {
		result.RetryAfter = retryAfter
	}
	return result
}

func classifyFailureClass(ev FailureEvidence) FailureClass {
	toolCode := toolErrorCode(ev.ToolError)
	if ev.CancelRequested || toolCode == tools.ErrorCodeCanceled {
		return FailureCancellation
	}
	if ev.PayloadFailure != nil {
		return FailurePayloadDeclared
	}
	if ev.QualityRejected {
		return FailureQualityRejection
	}
	if ev.Authoring {
		return FailureAuthoring
	}
	if ev.AttemptTimedOut {
		return FailureAttemptTimeout
	}
	if ev.BudgetExhausted {
		return FailureBudgetExhausted
	}
	if ev.TargetUnavailable {
		return FailureTargetUnavailable
	}
	if ev.Transport || ev.LeaseExpired || isTransportToolCode(toolCode) {
		return FailureTransport
	}
	return FailurePayloadDeclared
}

func failureEvidenceDetail(ev FailureEvidence, class FailureClass) (string, string, string) {
	if class == FailurePayloadDeclared && ev.PayloadFailure != nil {
		return ev.PayloadFailure.Code, ev.PayloadFailure.Cause, ev.PayloadFailure.Recovery
	}
	code := strings.TrimSpace(ev.Code)
	cause := strings.TrimSpace(ev.Cause)
	hint := strings.TrimSpace(ev.Hint)
	if ev.ToolError != nil && (class != FailureCancellation || !ev.CancelRequested) {
		if code == "" {
			code = strings.TrimSpace(string(ev.ToolError.Code))
		}
		if cause == "" {
			cause = strings.TrimSpace(ev.ToolError.Message)
		}
		if ev.ToolError.Operator != nil {
			if operatorCause := strings.TrimSpace(ev.ToolError.Operator.Cause); operatorCause != "" {
				cause = operatorCause
			}
			if hint == "" {
				hint = strings.TrimSpace(ev.ToolError.Operator.Recovery)
			}
		}
	}
	if code == "" {
		code = string(class)
	}
	if cause == "" {
		cause = defaultFailureCause(class)
	}
	return code, cause, hint
}

func defaultFailureCause(class FailureClass) string {
	switch class {
	case FailureCancellation:
		return "node execution was canceled"
	case FailureQualityRejection:
		return "node output did not meet the quality contract"
	case FailureAuthoring:
		return "node authoring could not be evaluated"
	case FailureAttemptTimeout:
		return "node attempt exceeded its timeout"
	case FailureBudgetExhausted:
		return "node deadline or run budget was exhausted"
	case FailureTargetUnavailable:
		return "node target is unavailable"
	case FailureTransport:
		return "node transport failed"
	default:
		return "node result declared an application failure"
	}
}

func toolErrorCode(toolErr *tools.ToolError) tools.ErrorCode {
	if toolErr == nil {
		return ""
	}
	return toolErr.Code
}

func isTransportToolCode(code tools.ErrorCode) bool {
	switch code {
	case tools.ErrorCodeUnavailable, tools.ErrorCodeBackendFailed, tools.ErrorCodeTimedOut:
		return true
	default:
		return false
	}
}

func clampedRetryAfter(value *time.Duration, maxValue time.Duration) *time.Duration {
	if value == nil {
		return nil
	}
	clamped := max(*value, 0)
	if maxValue > 0 && clamped > maxValue {
		clamped = maxValue
	}
	return &clamped
}
