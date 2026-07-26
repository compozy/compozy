package acp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/store"
)

const (
	maxFailureSummaryBytes   = 2048
	requestErrorCanceledCode = -32800
)

// FailureError carries a typed lifecycle classification beside the wrapped ACP
// error so session orchestration can persist the failure without parsing text.
type FailureError struct {
	Kind    store.FailureKind
	Summary string
	Err     error
}

func (e *FailureError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return strings.TrimSpace(e.Summary)
}

func (e *FailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// WrapFailure annotates err with a typed ACP/session failure classification.
func WrapFailure(kind store.FailureKind, summary string, err error) error {
	if err == nil {
		return nil
	}
	if existing, ok := errors.AsType[*FailureError](err); ok {
		// Preserve the original wrapper chain even for typed-nil FailureError matches.
		if existing == nil {
			return err
		}
		return err
	}
	if !store.ValidFailureKind(kind) {
		kind = failureKindForError(err, store.FailureUnknown)
	}
	summary = providerFailureDiagnosticSummary(err, failureDiagnosticSummary(summary, err.Error()))
	return &FailureError{
		Kind:    kind,
		Summary: diagnostics.RedactAndBound(summary, maxFailureSummaryBytes),
		Err:     err,
	}
}

// FailureFromError extracts a redacted session failure from a typed or
// classifiable error.
func FailureFromError(err error, fallback store.FailureKind) (*store.SessionFailure, bool) {
	if err == nil {
		return nil, false
	}
	var failureErr *FailureError
	if errors.As(err, &failureErr) && failureErr != nil {
		kind := failureErr.Kind
		if !store.ValidFailureKind(kind) {
			kind = store.FailureUnknown
		}
		failure := store.SessionFailure{
			Kind: kind,
			Summary: diagnostics.RedactAndBound(
				providerFailureDiagnosticSummary(
					err,
					firstNonEmptyFailureText(failureErr.Summary, err.Error()),
				),
				maxFailureSummaryBytes,
			),
		}
		return &failure, true
	}

	kind := failureKindForError(err, fallback)
	if !store.ValidFailureKind(kind) || kind == "" {
		return nil, false
	}
	failure := store.SessionFailure{
		Kind:    kind,
		Summary: diagnostics.RedactAndBound(providerFailureDiagnosticSummary(err, err.Error()), maxFailureSummaryBytes),
	}
	return &failure, true
}

func failureKindForError(err error, fallback store.FailureKind) store.FailureKind {
	switch {
	case errors.Is(err, context.Canceled):
		return store.FailureCanceled
	case errors.Is(err, context.DeadlineExceeded):
		return store.FailureTimeout
	case errors.Is(err, io.ErrClosedPipe), errors.Is(err, io.EOF):
		return store.FailureTransport
	}

	if reqErr, ok := errors.AsType[*acpsdk.RequestError](err); ok {
		if requestErrorIndicatesCancellation(reqErr) {
			return store.FailureCanceled
		}
		if fallback == store.FailurePrompt && requestErrorIndicatesSessionLoss(reqErr) {
			return store.FailureProcess
		}
		switch fallback {
		case store.FailurePrompt, store.FailureLoad:
			return fallback
		default:
			return store.FailureProtocol
		}
	}

	if store.ValidFailureKind(fallback) {
		return fallback
	}
	return store.FailureUnknown
}

func requestErrorIndicatesCancellation(reqErr *acpsdk.RequestError) bool {
	if reqErr == nil {
		return false
	}
	if reqErr.Code == requestErrorCanceledCode {
		return true
	}

	text := strings.ToLower(strings.TrimSpace(requestErrorDiagnosticText(reqErr)))
	return strings.Contains(text, "request canceled") || strings.Contains(text, "context canceled")
}

func requestErrorIndicatesSessionLoss(reqErr *acpsdk.RequestError) bool {
	if reqErr == nil {
		return false
	}

	text := strings.ToLower(strings.TrimSpace(requestErrorDiagnosticText(reqErr)))
	switch {
	case strings.Contains(text, "process exited unexpectedly"):
		return true
	case strings.Contains(text, "peer disconnected before response"):
		return true
	case strings.Contains(text, "please start a new session"):
		return true
	case strings.Contains(text, "session not found"):
		return true
	case strings.Contains(text, "resource not found"):
		return true
	default:
		return false
	}
}

func requestErrorDiagnosticText(reqErr *acpsdk.RequestError) string {
	if reqErr == nil {
		return ""
	}

	parts := []string{reqErr.Message}
	if reqErr.Data != nil {
		parts = append(parts, fmt.Sprint(reqErr.Data))
		if payload, err := json.Marshal(reqErr.Data); err == nil {
			parts = append(parts, string(payload))
		}
	}
	return strings.Join(parts, " ")
}

func requestErrorRaw(err error) json.RawMessage {
	reqErr, ok := errors.AsType[*acpsdk.RequestError](err)
	if !ok || reqErr == nil {
		return nil
	}
	payload, marshalErr := json.Marshal(struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data,omitempty"`
	}{
		Code:    reqErr.Code,
		Message: reqErr.Message,
		Data:    reqErr.Data,
	})
	if marshalErr != nil {
		return nil
	}
	return payload
}

func firstNonEmptyFailureText(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func failureDiagnosticSummary(summary string, detail string) string {
	summary = strings.TrimSpace(summary)
	detail = strings.TrimSpace(detail)
	switch {
	case summary == "":
		return detail
	case detail == "":
		return summary
	case strings.Contains(summary, detail):
		return summary
	default:
		return summary + ": " + detail
	}
}

func failureSummary(failure *store.SessionFailure) string {
	if failure == nil {
		return ""
	}
	return failure.Normalize().Summary
}
