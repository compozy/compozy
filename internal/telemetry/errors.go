package telemetry

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"strings"
	"syscall"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Error classes reported by Classify, the only error description telemetry exports.
const (
	ErrorCanceled         = "canceled"
	ErrorDeadlineExceeded = "deadline_exceeded"
	ErrorProvider         = "provider_error"
	ErrorPermissionDenied = "permission_denied"
	ErrorNotFound         = "not_found"
	ErrorInvalidInput     = "invalid_input"
	ErrorIO               = "io_error"
	ErrorInternal         = "internal"
)

// errorTextClasses classifies errors that crossed a text boundary (provider or subprocess output)
// by fixed phrases; the text itself is never exported. Earlier entries win.
var errorTextClasses = []struct {
	class   string
	phrases []string
}{
	{ErrorPermissionDenied, []string{"permission denied", "operation not permitted", "access is denied"}},
	{ErrorNotFound, []string{"no such file or directory", "not found"}},
	{ErrorDeadlineExceeded, []string{"deadline exceeded", "timed out", "i/o timeout"}},
	{ErrorCanceled, []string{"context canceled"}},
	{ErrorInvalidInput, []string{"invalid argument"}},
	{
		ErrorIO,
		[]string{"broken pipe", "connection reset", "connection refused", "input/output error", "unexpected eof"},
	},
}

type providerError struct{ err error }

func (e providerError) Error() string { return e.err.Error() }

func (e providerError) Unwrap() error { return e.err }

// ProviderError marks err as a provider failure for Classify without changing its text or chain.
func ProviderError(err error) error {
	if err == nil {
		return nil
	}
	return providerError{err: err}
}

// Classify maps err to one fixed class; it never returns error text. A nil error has no class.
func Classify(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, context.Canceled):
		return ErrorCanceled
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, os.ErrDeadlineExceeded):
		return ErrorDeadlineExceeded
	}
	if _, ok := errors.AsType[providerError](err); ok {
		return ErrorProvider
	}
	switch {
	case errors.Is(err, fs.ErrPermission):
		return ErrorPermissionDenied
	case errors.Is(err, fs.ErrNotExist):
		return ErrorNotFound
	case errors.Is(err, fs.ErrInvalid):
		return ErrorInvalidInput
	case isIOError(err):
		return ErrorIO
	}
	message := strings.ToLower(err.Error())
	for _, entry := range errorTextClasses {
		for _, phrase := range entry.phrases {
			if strings.Contains(message, phrase) {
				return entry.class
			}
		}
	}
	return ErrorInternal
}

func isIOError(err error) bool {
	for _, target := range []error{io.EOF, io.ErrUnexpectedEOF, io.ErrClosedPipe, net.ErrClosed} {
		if errors.Is(err, target) {
			return true
		}
	}
	if _, ok := errors.AsType[*fs.PathError](err); ok {
		return true
	}
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}
	_, ok := errors.AsType[syscall.Errno](err)
	return ok
}

// MarkError records err on span as status Error with an empty description and error.type from
// Classify. Instrumentation uses it instead of RecordError or a status description. A span that is
// not recording (export disabled or unsampled) skips classification.
func MarkError(span trace.Span, err error) {
	if span == nil || err == nil || !span.IsRecording() {
		return
	}
	span.SetStatus(codes.Error, "")
	span.SetAttributes(AttrErrorType.String(Classify(err)))
}

// EndSpan ends span with compozy.outcome from err: canceled work gets OutcomeCancelled without an error status,
// any other error gets OutcomeError through MarkError, and no error gets OutcomeOK.
func EndSpan(span trace.Span, err error) {
	if span == nil {
		return
	}
	outcome := OutcomeOK
	switch {
	case errors.Is(err, context.Canceled):
		outcome = OutcomeCancelled
	case err != nil:
		outcome = OutcomeError
		MarkError(span, err)
	}
	span.SetAttributes(AttrOutcome.String(outcome))
	span.End()
}
