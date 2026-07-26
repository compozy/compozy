package diagnostics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	contract "github.com/compozy/agh/internal/diagnosticcontract"
	redactpkg "github.com/compozy/agh/internal/redact"
)

// ItemOption customizes a DiagnosticItem built by NewItem.
type ItemOption func(*itemOptions)

type itemOptions struct {
	suggestedCommand string
	docURL           string
	evidence         map[string]any
}

const malformedDiagnosticID = "diagnostics.malformed"

// WithSuggestedCommand records the exact recovery command for an operator or agent.
func WithSuggestedCommand(command string) ItemOption {
	return func(opts *itemOptions) {
		opts.suggestedCommand = strings.TrimSpace(command)
	}
}

// WithDocURL records the documentation URL for a diagnostic.
func WithDocURL(url string) ItemOption {
	return func(opts *itemOptions) {
		opts.docURL = strings.TrimSpace(url)
	}
}

// WithEvidence attaches structured diagnostic evidence after recursive redaction.
func WithEvidence(evidence map[string]any) ItemOption {
	return func(opts *itemOptions) {
		opts.evidence = evidence
	}
}

// NewItem builds the canonical redacted DiagnosticItem.
func NewItem(
	id string,
	code string,
	category string,
	title string,
	message string,
	severity string,
	freshness string,
	options ...ItemOption,
) contract.DiagnosticItem {
	opts := itemOptions{}
	for _, option := range options {
		if option != nil {
			option(&opts)
		}
	}

	item := contract.DiagnosticItem{
		ID:               strings.TrimSpace(id),
		Code:             strings.TrimSpace(code),
		Severity:         strings.TrimSpace(severity),
		Category:         strings.TrimSpace(category),
		Title:            Redact(strings.TrimSpace(title)),
		Message:          Redact(strings.TrimSpace(message)),
		SuggestedCommand: Redact(strings.TrimSpace(opts.suggestedCommand)),
		DocURL:           Redact(strings.TrimSpace(opts.docURL)),
		DataFreshness:    strings.TrimSpace(freshness),
		Evidence:         RedactEvidence(opts.evidence),
	}
	if err := contract.ValidateDiagnosticItem(item); err != nil {
		item = downgradeInvalidItem(item, err)
	}
	return item
}

// EmptyItem returns the zero DiagnosticItem from the diagnostics package so
// production callers do not construct DiagnosticItem literals directly.
func EmptyItem() contract.DiagnosticItem {
	return contract.DiagnosticItem{}
}

func downgradeInvalidItem(
	item contract.DiagnosticItem,
	validationErr error,
) contract.DiagnosticItem {
	originalCode := item.Code
	originalCategory := item.Category
	originalSeverity := item.Severity
	originalFreshness := item.DataFreshness
	code := item.Code
	var category string
	if canonicalCategory, ok := contract.DiagnosticCodeCategory(code); ok {
		category = canonicalCategory
	} else {
		code = contract.CodeUnknownComponent
		category = contract.CategoryDaemon
	}
	if strings.TrimSpace(item.ID) == "" {
		item.ID = malformedDiagnosticID
	}
	if strings.TrimSpace(item.Title) == "" {
		item.Title = "Malformed diagnostic"
	}
	if strings.TrimSpace(item.Message) == "" {
		item.Message = "Diagnostic item failed validation and was downgraded."
	}

	item.Code = code
	item.Category = category
	item.Severity = contract.SeverityCritical
	item.DataFreshness = contract.FreshnessStale
	item.Evidence = RedactEvidence(mergeEvidence(item.Evidence, map[string]any{
		"diagnostic_validation_error": Redact(validationErr.Error()),
		"original_code":               originalCode,
		"original_category":           originalCategory,
		"original_severity":           originalSeverity,
		"original_freshness":          originalFreshness,
	}))

	slog.Default().Warn(
		"diagnostics: invalid DiagnosticItem downgraded",
		"error",
		Redact(validationErr.Error()),
		"code",
		originalCode,
		"category",
		originalCategory,
		"severity",
		originalSeverity,
		"freshness",
		originalFreshness,
	)
	if err := contract.ValidateDiagnosticItem(item); err != nil {
		slog.Default().Warn("diagnostics: downgraded DiagnosticItem still invalid", "error", err)
	}
	return item
}

func mergeEvidence(left map[string]any, right map[string]any) map[string]any {
	merged := make(map[string]any, len(left)+len(right))
	maps.Copy(merged, left)
	maps.Copy(merged, right)
	return merged
}

// RedactItem reapplies the diagnostics redaction boundary to an existing item.
func RedactItem(item contract.DiagnosticItem) contract.DiagnosticItem {
	return contract.DiagnosticItem{
		ID:               strings.TrimSpace(item.ID),
		Code:             strings.TrimSpace(item.Code),
		Severity:         strings.TrimSpace(item.Severity),
		Category:         strings.TrimSpace(item.Category),
		Title:            Redact(strings.TrimSpace(item.Title)),
		Message:          Redact(strings.TrimSpace(item.Message)),
		SuggestedCommand: Redact(strings.TrimSpace(item.SuggestedCommand)),
		DocURL:           Redact(strings.TrimSpace(item.DocURL)),
		DataFreshness:    strings.TrimSpace(item.DataFreshness),
		Evidence:         RedactEvidence(item.Evidence),
	}
}

// RedactEvidence recursively redacts evidence values without mutating the input map.
func RedactEvidence(evidence map[string]any) map[string]any {
	if len(evidence) == 0 {
		return nil
	}
	redacted := make(map[string]any, len(evidence))
	for key, value := range evidence {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		redacted[key] = redactValueForKey(key, value)
	}
	if len(redacted) == 0 {
		return nil
	}
	return redacted
}

func redactValueForKey(key string, value any) any {
	if redactpkg.IsSensitiveKey(key) {
		return redactedValue
	}
	return RedactValue(value)
}

// RedactValue recursively redacts diagnostic evidence values.
func RedactValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return Redact(typed)
	case json.RawMessage:
		redacted, err := RedactJSON([]byte(typed))
		if err != nil {
			return Redact(string(typed))
		}
		var decoded any
		if unmarshalErr := json.Unmarshal(redacted, &decoded); unmarshalErr != nil {
			return string(redacted)
		}
		return decoded
	case []byte:
		return Redact(string(typed))
	case error:
		return Redact(typed.Error())
	case json.Number:
		return typed
	case fmt.Stringer:
		return Redact(typed.String())
	case map[string]any:
		return RedactEvidence(typed)
	case map[string]string:
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			redacted[strings.TrimSpace(key)] = redactValueForKey(key, nested)
		}
		return redacted
	case map[string][]string:
		redacted := make(map[string]any, len(typed))
		for key, nested := range typed {
			redacted[strings.TrimSpace(key)] = redactValueForKey(key, nested)
		}
		return redacted
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, RedactValue(nested))
		}
		return redacted
	case []string:
		redacted := make([]any, 0, len(typed))
		for _, nested := range typed {
			redacted = append(redacted, Redact(nested))
		}
		return redacted
	default:
		return value
	}
}

// RedactJSON recursively redacts secret-shaped values inside a JSON document.
func RedactJSON(raw []byte) ([]byte, error) {
	var decoded any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("diagnostics: decode JSON for redaction: %w", err)
	}
	redacted, err := json.Marshal(RedactValue(decoded))
	if err != nil {
		return nil, fmt.Errorf("diagnostics: encode redacted JSON: %w", err)
	}
	return redacted, nil
}

// StructuredError carries a DiagnosticItem while preserving the underlying cause.
type StructuredError struct {
	Item  contract.DiagnosticItem
	Cause error
}

var _ = [1]error{(*StructuredError)(nil)}

// NewStructuredError creates a diagnostic-bearing error.
func NewStructuredError(item contract.DiagnosticItem, cause error) *StructuredError {
	return &StructuredError{Item: RedactItem(item), Cause: newRedactedCause(cause)}
}

func (e *StructuredError) Error() string {
	if e == nil {
		return ""
	}
	title := strings.TrimSpace(e.Item.Title)
	message := strings.TrimSpace(e.Item.Message)
	switch {
	case title == "" && message == "":
		message = strings.TrimSpace(e.Item.Code)
	case title == "":
		title = message
		message = ""
	}
	rendered := title
	if message != "" && message != title {
		rendered += ": " + message
	}
	if command := strings.TrimSpace(e.Item.SuggestedCommand); command != "" {
		rendered += " (next: " + command + ")"
	}
	return rendered
}

func (e *StructuredError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type redactedCause struct {
	cause error
}

func newRedactedCause(cause error) error {
	if cause == nil {
		return nil
	}
	return &redactedCause{cause: cause}
}

func (e *redactedCause) Error() string {
	if e == nil || e.cause == nil {
		return ""
	}
	return Redact(e.cause.Error())
}

func (e *redactedCause) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// DiagnosticItem returns the carried diagnostic for common error renderers.
func (e *StructuredError) DiagnosticItem() contract.DiagnosticItem {
	if e == nil {
		return contract.DiagnosticItem{}
	}
	return RedactItem(e.Item)
}

type diagnosticItemCarrier interface {
	DiagnosticItem() contract.DiagnosticItem
}

// ItemFromError extracts a redacted DiagnosticItem from any diagnostic-bearing error.
func ItemFromError(err error) (contract.DiagnosticItem, bool) {
	if err == nil {
		return contract.DiagnosticItem{}, false
	}
	var carrier diagnosticItemCarrier
	if !errors.As(err, &carrier) {
		return contract.DiagnosticItem{}, false
	}
	item := RedactItem(carrier.DiagnosticItem())
	if strings.TrimSpace(item.Code) == "" {
		return contract.DiagnosticItem{}, false
	}
	return item, true
}
