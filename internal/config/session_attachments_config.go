package config

import (
	"fmt"
	"slices"
	"strings"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

const (
	// DefaultSessionAttachmentMaxFileBytes is the per-file admission ceiling.
	DefaultSessionAttachmentMaxFileBytes int64 = 10 << 20
	// DefaultSessionAttachmentMaxFilesPerPrompt is the per-prompt attachment count ceiling.
	DefaultSessionAttachmentMaxFilesPerPrompt = 10
)

// DefaultSessionAttachmentsConfig returns v1 admission and retention defaults.
func DefaultSessionAttachmentsConfig() SessionAttachmentsConfig {
	return SessionAttachmentsConfig{
		MaxFileBytes:      DefaultSessionAttachmentMaxFileBytes,
		MaxFilesPerPrompt: DefaultSessionAttachmentMaxFilesPerPrompt,
		AllowedMIME:       slices.Clone(attachmentspkg.DefaultAllowedMIME),
		Retention: SessionAttachmentsRetentionConfig{
			MaxCount: DefaultToolsArtifactMaxCount,
			MaxBytes: DefaultToolsArtifactMaxBytes,
			MaxAge:   DefaultToolsArtifactMaxAge,
		},
	}
}

// Validate ensures attachment admission and retention values form real positive bounds.
func (c SessionAttachmentsConfig) Validate() error {
	if c.MaxFileBytes <= 0 {
		return fmt.Errorf("session.attachments.max_file_bytes must be greater than zero: %d", c.MaxFileBytes)
	}
	if c.MaxFilesPerPrompt <= 0 {
		return fmt.Errorf(
			"session.attachments.max_files_per_prompt must be greater than zero: %d",
			c.MaxFilesPerPrompt,
		)
	}
	if len(c.AllowedMIME) == 0 {
		return fmt.Errorf("session.attachments.allowed_mime must not be empty")
	}
	seen := make(map[string]struct{}, len(c.AllowedMIME))
	for i, raw := range c.AllowedMIME {
		mime := strings.ToLower(strings.TrimSpace(raw))
		if mime == "" {
			return fmt.Errorf("session.attachments.allowed_mime[%d] must not be blank", i)
		}
		if !slices.Contains(attachmentspkg.DefaultAllowedMIME, mime) {
			return fmt.Errorf(
				"session.attachments.allowed_mime[%d] must be one of %s: %q",
				i,
				strings.Join(attachmentspkg.DefaultAllowedMIME, ", "),
				raw,
			)
		}
		if _, exists := seen[mime]; exists {
			return fmt.Errorf("session.attachments.allowed_mime[%d] duplicates %q", i, mime)
		}
		seen[mime] = struct{}{}
	}
	return c.Retention.Validate()
}

// Validate ensures attachment retention values form real positive bounds.
func (c SessionAttachmentsRetentionConfig) Validate() error {
	if c.MaxCount <= 0 {
		return fmt.Errorf("session.attachments.retention.max_count must be greater than zero: %d", c.MaxCount)
	}
	if c.MaxBytes <= 0 {
		return fmt.Errorf("session.attachments.retention.max_bytes must be greater than zero: %d", c.MaxBytes)
	}
	if c.MaxAge <= 0 {
		return fmt.Errorf("session.attachments.retention.max_age must be greater than zero: %s", c.MaxAge)
	}
	return nil
}
