package bridgesdk

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

// ErrorClass is the shared bridge-sdk provider failure classification.
type ErrorClass string

// HTTPStatusOverloaded is the non-standard provider overload response status.
const HTTPStatusOverloaded = 529

const (
	ErrorClassAuth        ErrorClass = "auth"
	ErrorClassRateLimit   ErrorClass = "rate_limit"
	ErrorClassTimeout     ErrorClass = "timeout"
	ErrorClassOverloaded  ErrorClass = "overloaded"
	ErrorClassServerError ErrorClass = "server_error"
	ErrorClassTransient   ErrorClass = "transient"
	ErrorClassPermanent   ErrorClass = "permanent"
)

// HTTPError captures provider HTTP failures with optional Retry-After guidance.
type HTTPError struct {
	StatusCode int
	Message    string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) == "" {
		return fmt.Sprintf("http %d", e.StatusCode)
	}
	return e.Message
}

// AuthError marks an authentication failure explicitly.
type AuthError struct {
	Err error
}

func (e *AuthError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *AuthError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// RateLimitError marks a rate-limit failure explicitly.
type RateLimitError struct {
	Err        error
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *RateLimitError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// TransientError marks a retryable provider failure explicitly.
type TransientError struct {
	Err error
}

func (e *TransientError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *TransientError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// PermanentError marks a non-retryable provider failure explicitly.
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CommittedMutationError marks a remote mutation that the provider accepted,
// but whose required result could not be materialized locally. Retrying such
// an operation could duplicate the remote side effect.
type CommittedMutationError struct {
	Err error
}

const committedMutationFallbackMessage = "remote mutation committed but its result is unavailable"

func (e *CommittedMutationError) Error() string {
	if e == nil || e.Err == nil {
		return committedMutationFallbackMessage
	}
	message := e.Err.Error()
	if strings.TrimSpace(message) == "" {
		return committedMutationFallbackMessage
	}
	return message
}

func (e *CommittedMutationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// MarkCommittedMutation preserves an existing committed-mutation marker or
// wraps an error so retry layers can distinguish it from a pre-commit failure.
func MarkCommittedMutation(err error) error {
	if err == nil {
		return nil
	}
	if committed, ok := errors.AsType[*CommittedMutationError](err); ok && committed != nil {
		return err
	}
	return &CommittedMutationError{Err: err}
}

// IsCommittedMutation reports whether a remote mutation crossed its commit boundary.
func IsCommittedMutation(err error) bool {
	var committed *CommittedMutationError
	return errors.As(err, &committed)
}

// ShouldContinueTextDeliveryAfterProgress reports whether a progress failure is
// safe to consume before the independently addressed text mutation. A committed
// progress mutation must never be misreported as the outcome of the text event.
func ShouldContinueTextDeliveryAfterProgress(err error) bool {
	return err == nil || IsCommittedMutation(err)
}

// ClassifiedError is one actionable provider failure classification.
type ClassifiedError struct {
	Class      ErrorClass
	Err        error
	RetryAfter time.Duration
	Message    string
}

// RecoveryDecision is the runtime action derived from one classified error.
type RecoveryDecision struct {
	Retry       bool
	RetryAfter  time.Duration
	Status      bridgepkg.BridgeStatus
	Degradation *bridgepkg.BridgeDegradation
}

// ClassifyError maps one provider failure into the shared recovery classes.
func ClassifyError(err error) ClassifiedError {
	if err == nil {
		return ClassifiedError{}
	}
	if classified, ok := classifyTypedProviderError(err); ok {
		return classified
	}
	if classified, ok := classifyHTTPProviderError(err); ok {
		return classified
	}
	if classified, ok := classifyRuntimeProviderError(err); ok {
		return classified
	}
	return classifyProviderErrorText(err)
}

func classifyTypedProviderError(err error) (ClassifiedError, bool) {
	if IsCommittedMutation(err) {
		return classifiedProviderError(err, ErrorClassPermanent, 0), true
	}

	if authErr, ok := errors.AsType[*AuthError](err); ok && authErr != nil {
		return classifiedProviderError(err, ErrorClassAuth, 0), true
	}

	if rateLimitErr, ok := errors.AsType[*RateLimitError](err); ok {
		return classifiedProviderError(err, ErrorClassRateLimit, rateLimitErr.RetryAfter), true
	}

	if permanentErr, ok := errors.AsType[*PermanentError](err); ok && permanentErr != nil {
		return classifiedProviderError(err, ErrorClassPermanent, 0), true
	}

	if transientErr, ok := errors.AsType[*TransientError](err); ok && transientErr != nil {
		return classifiedProviderError(err, ErrorClassTransient, 0), true
	}

	return ClassifiedError{}, false
}

func classifyHTTPProviderError(err error) (ClassifiedError, bool) {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return ClassifiedError{}, false
	}

	switch httpErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return classifiedProviderError(err, ErrorClassAuth, 0), true
	case http.StatusTooManyRequests:
		return classifiedProviderError(err, ErrorClassRateLimit, httpErr.RetryAfter), true
	case HTTPStatusOverloaded:
		return classifiedProviderError(err, ErrorClassOverloaded, httpErr.RetryAfter), true
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return classifiedProviderError(err, ErrorClassTimeout, 0), true
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusInternalServerError:
		return classifiedProviderError(err, ErrorClassServerError, httpErr.RetryAfter), true
	default:
		return classifiedProviderError(err, ErrorClassPermanent, 0), true
	}
}

func classifyRuntimeProviderError(err error) (ClassifiedError, bool) {
	if errors.Is(err, context.DeadlineExceeded) {
		return classifiedProviderError(err, ErrorClassTimeout, 0), true
	}

	var netErr net.Error
	if !errors.As(err, &netErr) {
		return ClassifiedError{}, false
	}
	if netErr.Timeout() {
		return classifiedProviderError(err, ErrorClassTimeout, 0), true
	}
	return classifiedProviderError(err, ErrorClassTransient, 0), true
}

func classifyProviderErrorText(err error) ClassifiedError {
	text := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(text, "auth"),
		strings.Contains(text, "forbidden"),
		strings.Contains(text, "unauthorized"),
		strings.Contains(text, "token"):
		return classifiedProviderError(err, ErrorClassAuth, 0)
	case strings.Contains(text, "rate limit"), strings.Contains(text, "too many requests"):
		return classifiedProviderError(err, ErrorClassRateLimit, 0)
	case strings.Contains(text, "timeout"), strings.Contains(text, "deadline exceeded"):
		return classifiedProviderError(err, ErrorClassTimeout, 0)
	case strings.Contains(text, "temporary"),
		strings.Contains(text, "unavailable"),
		strings.Contains(text, "connection reset"),
		strings.Contains(text, "broken pipe"),
		strings.Contains(text, "eof"):
		return classifiedProviderError(err, ErrorClassTransient, 0)
	default:
		return classifiedProviderError(err, ErrorClassPermanent, 0)
	}
}

func classifiedProviderError(err error, class ErrorClass, retryAfter time.Duration) ClassifiedError {
	return ClassifiedError{
		Class:      class,
		Err:        err,
		RetryAfter: retryAfter,
		Message:    errorMessage(err),
	}
}

// Recovery maps the classified provider failure into runtime actions.
func (c ClassifiedError) Recovery() RecoveryDecision {
	switch c.Class {
	case ErrorClassAuth:
		return RecoveryDecision{
			Status: bridgepkg.BridgeStatusAuthRequired,
			Degradation: &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
				Message: c.Message,
			},
		}
	case ErrorClassRateLimit:
		return RecoveryDecision{
			Retry:      true,
			RetryAfter: c.RetryAfter,
			Status:     bridgepkg.BridgeStatusDegraded,
			Degradation: &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonRateLimited,
				Message: c.Message,
			},
		}
	case ErrorClassTimeout:
		return RecoveryDecision{
			Retry:  true,
			Status: bridgepkg.BridgeStatusDegraded,
			Degradation: &bridgepkg.BridgeDegradation{
				Reason:  bridgepkg.BridgeDegradationReasonProviderTimeout,
				Message: c.Message,
			},
		}
	case ErrorClassOverloaded, ErrorClassServerError:
		return RecoveryDecision{
			Retry:      true,
			RetryAfter: c.RetryAfter,
			Status:     bridgepkg.BridgeStatusDegraded,
		}
	case ErrorClassTransient:
		return RecoveryDecision{
			Retry:  true,
			Status: bridgepkg.BridgeStatusDegraded,
		}
	case ErrorClassPermanent:
		return RecoveryDecision{
			Status: bridgepkg.BridgeStatusError,
		}
	default:
		return RecoveryDecision{}
	}
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}
