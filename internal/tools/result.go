package tools

import (
	"bytes"
	"encoding/json"

	redactpkg "github.com/compozy/agh/internal/redact"
)

// ToolContent is one typed content block returned by a tool.
type ToolContent struct {
	Type     string                     `json:"type"`
	Text     string                     `json:"text,omitempty"`
	Data     json.RawMessage            `json:"data,omitempty"`
	MIMEType string                     `json:"mime_type,omitempty"`
	Metadata map[string]json.RawMessage `json:"metadata,omitempty"`
}

// ArtifactRef points to a durable tool output artifact.
type ArtifactRef struct {
	URI      string `json:"uri"`
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	Bytes    int64  `json:"bytes,omitempty"`
	SHA256   string `json:"sha256,omitempty"`
}

// Redaction records a redaction applied before a result crosses surfaces.
type Redaction struct {
	Path   string     `json:"path"`
	Reason ReasonCode `json:"reason"`
	Bytes  int64      `json:"bytes,omitempty"`
}

// ToolResult is the canonical result envelope for all backends.
type ToolResult struct {
	Content    []ToolContent              `json:"content,omitempty"`
	Structured json.RawMessage            `json:"structured,omitempty"`
	Preview    string                     `json:"preview,omitempty"`
	Artifacts  []ArtifactRef              `json:"artifacts,omitempty"`
	Metadata   map[string]json.RawMessage `json:"metadata,omitempty"`
	Redactions []Redaction                `json:"redactions,omitempty"`
	Truncated  bool                       `json:"truncated"`
	Bytes      int64                      `json:"bytes"`
	DurationMS int64                      `json:"duration_ms"`
}

// Validate checks the public result envelope and metadata safety.
func (r ToolResult) Validate(maxBytes int64) error {
	if err := validateRawJSON("structured", r.Structured); err != nil {
		return err
	}
	if r.Bytes < 0 {
		return NewValidationError("bytes", ReasonResultBudgetExceeded, "bytes must be greater than or equal to zero")
	}
	if r.DurationMS < 0 {
		return NewValidationError(
			"duration_ms",
			ReasonBackendUnhealthy,
			"duration must be greater than or equal to zero",
		)
	}
	if maxBytes >= 0 && r.Bytes > maxBytes && !r.Truncated {
		return NewValidationError("truncated", ReasonResultBudgetExceeded, "oversized results must be marked truncated")
	}
	if err := validateMetadataKeys("metadata", r.Metadata); err != nil {
		return err
	}
	for i, content := range r.Content {
		if content.Type == "" {
			return NewValidationError(
				indexedField("content", i)+".type",
				ReasonSchemaInvalid,
				"content type is required",
			)
		}
		if err := validateRawJSON(indexedField("content", i)+".data", content.Data); err != nil {
			return err
		}
		if err := validateMetadataKeys(indexedField("content", i)+".metadata", content.Metadata); err != nil {
			return err
		}
	}
	for i, artifact := range r.Artifacts {
		if artifact.Bytes < 0 {
			return NewValidationError(
				indexedField("artifacts", i)+".bytes",
				ReasonResultBudgetExceeded,
				"artifact bytes must be greater than or equal to zero",
			)
		}
	}
	for i, redaction := range r.Redactions {
		if redaction.Path == "" {
			return NewValidationError(
				indexedField("redactions", i)+".path",
				ReasonSecretMetadata,
				"redaction path is required",
			)
		}
		if err := redaction.Reason.Validate(indexedField("redactions", i) + ".reason"); err != nil {
			return err
		}
	}
	return nil
}

func validateMetadataKeys(field string, metadata map[string]json.RawMessage) error {
	for key, value := range metadata {
		path := field + "." + key
		if sensitiveMetadataKey(key) {
			return NewValidationError(path, ReasonSecretMetadata, "metadata key may expose backend secrets")
		}
		if err := validateRawJSON(path, value); err != nil {
			return err
		}
	}
	return nil
}

func validateRawJSON(field string, raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	if !json.Valid(raw) {
		return NewValidationError(field, ReasonSchemaInvalid, "value must be valid JSON")
	}
	return nil
}

func sensitiveMetadataKey(key string) bool {
	return redactpkg.IsSensitiveKey(key)
}
