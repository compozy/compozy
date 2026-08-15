package config

import (
	"fmt"
	"time"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

const (
	// DefaultSessionAttachmentMaxFileBytes is the per-file admission ceiling.
	DefaultSessionAttachmentMaxFileBytes int64 = 10 << 20
	// DefaultSessionAttachmentMaxFilesPerPrompt is the per-prompt attachment count ceiling.
	DefaultSessionAttachmentMaxFilesPerPrompt = 10
	// DefaultSessionAttachmentRetentionMaxCount is the retained attachment count ceiling.
	DefaultSessionAttachmentRetentionMaxCount = 200
	// DefaultSessionAttachmentRetentionMaxBytes is the retained attachment byte ceiling.
	DefaultSessionAttachmentRetentionMaxBytes int64 = 1 << 30
	// DefaultSessionAttachmentRetentionMaxAge is the retained attachment age ceiling.
	DefaultSessionAttachmentRetentionMaxAge = 720 * time.Hour
)

// DefaultSessionAttachmentsConfig returns v1 admission and retention defaults.
func DefaultSessionAttachmentsConfig() SessionAttachmentsConfig {
	return SessionAttachmentsConfig{
		MaxFileBytes:      DefaultSessionAttachmentMaxFileBytes,
		MaxFilesPerPrompt: DefaultSessionAttachmentMaxFilesPerPrompt,
		AllowedMIME:       attachmentspkg.DefaultAllowedMIMEs(),
		Retention: SessionAttachmentsRetentionConfig{
			MaxCount: DefaultSessionAttachmentRetentionMaxCount,
			MaxBytes: DefaultSessionAttachmentRetentionMaxBytes,
			MaxAge:   DefaultSessionAttachmentRetentionMaxAge,
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
	if _, err := attachmentspkg.NormalizeAllowedMIME(c.AllowedMIME); err != nil {
		return fmt.Errorf("session.attachments.allowed_mime: %w", err)
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
