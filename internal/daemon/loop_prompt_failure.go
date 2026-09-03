package daemon

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
)

func classifyProviderEventErrorCode(eventError string) string {
	normalized := strings.ToLower(eventError)
	switch {
	case strings.Contains(normalized, "usage limit"),
		strings.Contains(normalized, "quota"),
		strings.Contains(normalized, "rate limit"),
		strings.Contains(normalized, "usagelimitexceeded"):
		return "quota_exceeded"
	case strings.Contains(normalized, "authenticate"),
		strings.Contains(normalized, "unauthorized"),
		strings.Contains(normalized, "oauth"),
		strings.Contains(normalized, "forbidden"),
		strings.Contains(normalized, "token"):
		return "provider_auth_failure"
	case strings.Contains(normalized, "timeout"),
		strings.Contains(normalized, "deadline"):
		return roleFieldTimeout
	case strings.Contains(normalized, "disconnect"),
		strings.Contains(normalized, "connection"),
		strings.Contains(normalized, "transport"),
		strings.Contains(normalized, "eof"):
		return "transport_failure"
	default:
		return "provider_error"
	}
}

func evaluatePromptProviderFailure(
	promptFailure *store.SessionFailure,
	eventError string,
	stopReason acp.PromptStopReason,
) error {
	if promptFailure != nil {
		code := string(promptFailure.Kind)
		if code == "" {
			code = "provider_failure"
		}
		cause := promptFailure.Summary
		if cause == "" {
			cause = "provider session failure"
		}
		return looppkg.NewSafeActionFailureError(
			fmt.Errorf("daemon: prompt session failure (%s): %s", code, cause),
			looppkg.NewActionFailure(
				code,
				cause,
				"check provider credentials, quota, or network connectivity",
			),
		)
	}

	if eventError != "" {
		code := classifyProviderEventErrorCode(eventError)
		return looppkg.NewSafeActionFailureError(
			fmt.Errorf("daemon: prompt provider error (%s): %s", code, eventError),
			looppkg.NewActionFailure(
				code,
				eventError,
				"check provider service status, quota, or credentials",
			),
		)
	}

	if stopReason == acp.PromptStopReasonRefusal {
		return looppkg.NewSafeActionFailureError(
			errors.New("daemon: prompt model refusal"),
			looppkg.NewActionFailure(
				"model_refusal",
				"the model refused the prompt request",
				"adjust prompt parameters or safety boundaries",
			),
		)
	}

	return nil
}
