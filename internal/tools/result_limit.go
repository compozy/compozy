package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/compozy/agh/internal/diagnostics"
)

const (
	resultLimitTextKey = "text"
)

const redactedJSONValue = "[REDACTED]"

// DefaultResultProcessor applies secret redaction and descriptor/default byte caps.
type DefaultResultProcessor struct {
	defaultMaxBytes int64
	artifacts       ToolArtifactStore
	sensitiveFields []string
}

var _ ResultProcessor = (*DefaultResultProcessor)(nil)

// NewResultProcessor builds the default registry result processor.
func NewResultProcessor(
	defaultMaxBytes int64,
	artifacts ToolArtifactStore,
	sensitiveFields ...string,
) *DefaultResultProcessor {
	return &DefaultResultProcessor{
		defaultMaxBytes: defaultMaxBytes,
		artifacts:       artifacts,
		sensitiveFields: normalizeSensitiveFields(sensitiveFields),
	}
}

// Sanitize redacts sensitive fields before a result reaches post-call hooks.
func (l *DefaultResultProcessor) Sanitize(ctx context.Context, d Descriptor, result ToolResult) (ToolResult, error) {
	if err := contextErr(ctx, d.ID); err != nil {
		return ToolResult{}, err
	}
	limited := cloneToolResult(result)
	redactions, err := redactToolResult(&limited, l.sensitiveFields)
	if err != nil {
		return ToolResult{}, resultLimiterRejection(d.ID, err)
	}
	limited.Redactions = appendRedactions(limited.Redactions, redactions...)
	if err := refreshResultEnvelopeBytes(&limited); err != nil {
		return ToolResult{}, resultLimiterRejection(d.ID, err)
	}
	if err := limited.Validate(-1); err != nil {
		return ToolResult{}, resultLimiterRejection(d.ID, err)
	}
	return limited, nil
}

func resultLimiterRejection(id ToolID, err error) *ToolError {
	reason, ok := ReasonOf(err)
	if !ok {
		reason = ReasonResultBudgetExceeded
	}
	return NewToolError(
		ErrorCodeResultTooLarge,
		id,
		fmt.Sprintf("tool %q result rejected", id),
		fmt.Errorf("%w: %w", ErrToolResultTooLarge, err),
		reason,
	)
}

func (l *DefaultResultProcessor) maxBytes(d Descriptor) int64 {
	switch {
	case d.MaxResultBytes > 0:
		return d.MaxResultBytes
	case l.defaultMaxBytes > 0:
		return l.defaultMaxBytes
	default:
		return -1
	}
}

func cloneToolResult(src ToolResult) ToolResult {
	cloned := src
	cloned.Structured = cloneRawMessage(src.Structured)
	cloned.Content = make([]ToolContent, len(src.Content))
	for i := range src.Content {
		cloned.Content[i] = src.Content[i]
		cloned.Content[i].Data = cloneRawMessage(src.Content[i].Data)
		cloned.Content[i].Metadata = cloneRawMap(src.Content[i].Metadata)
	}
	cloned.Artifacts = append([]ArtifactRef(nil), src.Artifacts...)
	cloned.Metadata = cloneRawMap(src.Metadata)
	cloned.Redactions = append([]Redaction(nil), src.Redactions...)
	return cloned
}

func cloneRawMap(src map[string]json.RawMessage) map[string]json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	cloned := make(map[string]json.RawMessage, len(src))
	for key, value := range src {
		cloned[key] = cloneRawMessage(value)
	}
	return cloned
}

func redactToolResult(result *ToolResult, fields []string) ([]Redaction, error) {
	var redactions []Redaction
	var err error
	result.Preview, redactions = redactDisplayText(result.Preview, "$.preview", redactions)
	var structured json.RawMessage
	structured, redactions, err = redactRawJSON(result.Structured, "$.structured", fields, redactions)
	if err != nil {
		return nil, err
	}
	result.Structured = structured
	for i := range result.Content {
		result.Content[i].Text, redactions = redactDisplayText(
			result.Content[i].Text,
			fmt.Sprintf("$.content[%d].text", i),
			redactions,
		)
		path := fmt.Sprintf("$.content[%d].data", i)
		result.Content[i].Data, redactions, err = redactRawJSON(result.Content[i].Data, path, fields, redactions)
		if err != nil {
			return nil, err
		}
		redactions, err = redactRawMap(
			result.Content[i].Metadata,
			fmt.Sprintf("$.content[%d].metadata", i),
			fields,
			redactions,
		)
		if err != nil {
			return nil, err
		}
	}
	redactions, err = redactRawMap(result.Metadata, "$.metadata", fields, redactions)
	if err != nil {
		return nil, err
	}
	return redactions, nil
}

func redactDisplayText(text string, path string, redactions []Redaction) (string, []Redaction) {
	redacted := diagnostics.Redact(text)
	if redacted == text {
		return text, redactions
	}
	redactions = append(redactions, Redaction{
		Path:   path,
		Reason: ReasonSecretMetadata,
		Bytes:  int64(len(text)),
	})
	return redacted, redactions
}

func redactRawMap(
	values map[string]json.RawMessage,
	basePath string,
	fields []string,
	redactions []Redaction,
) ([]Redaction, error) {
	for key, value := range values {
		path := basePath + "." + key
		if sensitiveFieldName(key, fields) {
			delete(values, key)
			redactions = append(redactions, Redaction{
				Path:   path,
				Reason: ReasonSecretMetadata,
				Bytes:  int64(len(value)),
			})
			continue
		}
		redacted, next, err := redactRawJSON(value, path, fields, redactions)
		if err != nil {
			return nil, err
		}
		values[key] = redacted
		redactions = next
	}
	return redactions, nil
}

func redactRawJSON(
	raw json.RawMessage,
	path string,
	fields []string,
	redactions []Redaction,
) (json.RawMessage, []Redaction, error) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return cloneRawMessage(raw), redactions, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, nil, NewValidationError(path, ReasonSchemaInvalid, err.Error())
	}
	changed, redactedValue, next := redactJSONValue(value, path, fields, redactions)
	if !changed {
		return cloneRawMessage(raw), next, nil
	}
	data, err := json.Marshal(redactedValue)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal redacted result: %w", err)
	}
	return data, next, nil
}

func redactJSONValue(value any, path string, fields []string, redactions []Redaction) (bool, any, []Redaction) {
	switch typed := value.(type) {
	case map[string]any:
		changed := false
		for _, key := range sortedAnyKeys(typed) {
			childPath := path + "." + key
			if sensitiveFieldName(key, fields) {
				redactions = append(redactions, Redaction{
					Path:   childPath,
					Reason: ReasonSecretMetadata,
					Bytes:  int64(len(fmt.Sprint(typed[key]))),
				})
				typed[key] = redactedJSONValue
				changed = true
				continue
			}
			childChanged, childValue, next := redactJSONValue(typed[key], childPath, fields, redactions)
			redactions = next
			if childChanged {
				typed[key] = childValue
				changed = true
			}
		}
		return changed, typed, redactions
	case []any:
		changed := false
		for i := range typed {
			childChanged, childValue, next := redactJSONValue(
				typed[i],
				path+"["+strconv.Itoa(i)+"]",
				fields,
				redactions,
			)
			redactions = next
			if childChanged {
				typed[i] = childValue
				changed = true
			}
		}
		return changed, typed, redactions
	default:
		return false, value, redactions
	}
}

func sortedAnyKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func resultEnvelopeBytes(result ToolResult) (int64, error) {
	copyForSize := result
	copyForSize.Bytes = 0
	data, err := json.Marshal(copyForSize)
	if err != nil {
		return 0, NewValidationError("$", ReasonSchemaInvalid, err.Error())
	}
	return int64(len(data)), nil
}

func refreshResultEnvelopeBytes(result *ToolResult) error {
	bytes, err := resultEnvelopeBytes(*result)
	if err != nil {
		return err
	}
	result.Bytes = bytes
	return nil
}

func truncateToolResult(
	result ToolResult,
	maxBytes int64,
	artifacts []ArtifactRef,
) (ToolResult, error) {
	originalBytes := result.Bytes
	if maxBytes < 0 {
		return result, nil
	}
	preview := result.Preview
	if strings.TrimSpace(preview) == "" {
		preview = fallbackPreview(result)
	}
	if maxBytes == 0 {
		preview = ""
	} else if int64(len(preview)) > maxBytes {
		preview = truncateUTF8ByBytes(preview, int(maxBytes))
	}
	result.Content = nil
	if preview != "" {
		result.Content = []ToolContent{{Type: resultLimitTextKey, Text: preview}}
	}
	result.Structured = nil
	result.Preview = preview
	result.Artifacts = append([]ArtifactRef(nil), artifacts...)
	result.Truncated = true
	if err := refreshResultEnvelopeBytes(&result); err != nil {
		return ToolResult{}, err
	}
	result.Redactions = appendRedactions(result.Redactions, Redaction{
		Path:   "$",
		Reason: ReasonResultBudgetExceeded,
		Bytes:  max(0, originalBytes-maxBytes),
	})
	if result.Metadata == nil {
		result.Metadata = make(map[string]json.RawMessage, 1)
	}
	result.Metadata["truncated_from_bytes"] = json.RawMessage(strconv.FormatInt(originalBytes, 10))
	if err := refreshResultEnvelopeBytes(&result); err != nil {
		return ToolResult{}, err
	}
	for result.Bytes > maxBytes && result.Preview != "" {
		result.Preview = truncateUTF8ByBytes(result.Preview, len(result.Preview)-1)
		if len(result.Content) == 1 {
			result.Content[0].Text = result.Preview
		}
		if err := refreshResultEnvelopeBytes(&result); err != nil {
			return ToolResult{}, err
		}
	}
	if result.Bytes > maxBytes && maxBytes >= 0 {
		result.Content = nil
		result.Preview = ""
		result.Metadata = map[string]json.RawMessage{
			"truncated_from_bytes": json.RawMessage(strconv.FormatInt(originalBytes, 10)),
		}
		if err := refreshResultEnvelopeBytes(&result); err != nil {
			return ToolResult{}, err
		}
	}
	return result, nil
}

func fallbackPreview(result ToolResult) string {
	var builder strings.Builder
	for _, content := range result.Content {
		if content.Text != "" {
			if builder.Len() > 0 {
				builder.WriteByte('\n')
			}
			builder.WriteString(content.Text)
		}
	}
	if builder.Len() > 0 {
		return builder.String()
	}
	if len(result.Structured) > 0 {
		return string(result.Structured)
	}
	return ""
}

func truncateUTF8ByBytes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	for limit > 0 && !utf8.ValidString(value[:limit]) {
		limit--
	}
	return value[:limit]
}

func appendRedactions(existing []Redaction, incoming ...Redaction) []Redaction {
	for _, item := range incoming {
		if item.Path == "" {
			continue
		}
		duplicate := slices.ContainsFunc(existing, func(candidate Redaction) bool {
			return candidate.Path == item.Path && candidate.Reason == item.Reason
		})
		if !duplicate {
			existing = append(existing, item)
		}
	}
	return existing
}

func normalizeSensitiveFields(fields []string) []string {
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		item := normalizeSensitiveField(field)
		if item != "" && !slices.Contains(normalized, item) {
			normalized = append(normalized, item)
		}
	}
	return normalized
}

func normalizeSensitiveField(field string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(field), "-", "_"))
}

func sensitiveFieldName(key string, configured []string) bool {
	normalized := normalizeSensitiveField(key)
	if slices.Contains(configured, normalized) {
		return true
	}
	if publicDiagnosticFieldName(normalized) {
		return false
	}
	return sensitiveMetadataKey(normalized)
}

func publicDiagnosticFieldName(normalized string) bool {
	return normalized == "token_present" ||
		normalized == "max_input_tokens" ||
		normalized == "max_output_tokens"
}

func digestRaw(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	sum := sha256.Sum256(trimmed)
	return hex.EncodeToString(sum[:])
}
