package modelcatalog

import (
	"errors"
	"fmt"
	"strings"
)

// InvalidViewError reports an unsupported public catalog view.
type InvalidViewError struct {
	View string
}

// InvalidReasoningEffortError reports a non-canonical effort at a catalog source boundary.
type InvalidReasoningEffortError struct {
	ProviderID string
	ModelID    string
	Field      string
	Effort     string
}

func (e *InvalidReasoningEffortError) Error() string {
	if e == nil {
		return "model catalog: invalid reasoning effort"
	}
	return fmt.Sprintf(
		"model catalog provider %q model %q %s value %q is invalid",
		strings.TrimSpace(e.ProviderID),
		strings.TrimSpace(e.ModelID),
		e.Field,
		strings.TrimSpace(e.Effort),
	)
}

func (e *InvalidViewError) Error() string {
	if e == nil {
		return "model catalog: invalid view"
	}
	return fmt.Sprintf(
		"model catalog view %q is invalid; expected curated or all",
		strings.TrimSpace(e.View),
	)
}

var (
	// ErrAllSourcesFailed reports that refresh could not produce usable rows.
	ErrAllSourcesFailed = errors.New("model catalog: all usable sources failed")
	// ErrSourceDisabled reports that a source is intentionally disabled.
	ErrSourceDisabled = errors.New("model catalog: source disabled")
	// ErrSourceNotRegistered reports that a requested source id is not registered.
	ErrSourceNotRegistered = errors.New("model catalog: source not registered")
)

// StaleFallbackError reports a refresh failure that returned stale fallback rows.
type StaleFallbackError struct {
	SourceID string
	Err      error
}

func (e *StaleFallbackError) Error() string {
	if e == nil {
		return "model catalog: stale fallback"
	}
	return fmt.Sprintf("model catalog: source %q returned stale fallback", e.SourceID)
}

func (e *StaleFallbackError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func sourceErrorText(err error) string {
	if err == nil {
		return ""
	}
	var fallback *StaleFallbackError
	if errors.As(err, &fallback) && fallback.Err != nil {
		return RedactString(fallback.Err.Error())
	}
	return RedactString(err.Error())
}
