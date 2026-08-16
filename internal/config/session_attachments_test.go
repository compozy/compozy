package config

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	attachmentspkg "github.com/compozy/compozy/internal/attachments"
)

func TestSessionAttachmentsConfigDefaultsAndValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should return v1 defaults that validate", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultSessionAttachmentsConfig()
		if cfg.MaxFileBytes != DefaultSessionAttachmentMaxFileBytes ||
			cfg.MaxFilesPerPrompt != DefaultSessionAttachmentMaxFilesPerPrompt {
			t.Fatalf("DefaultSessionAttachmentsConfig() = %#v", cfg)
		}
		if !slices.Equal(cfg.AllowedMIME, attachmentspkg.DefaultAllowedMIMEs()) {
			t.Fatalf("DefaultSessionAttachmentsConfig().AllowedMIME = %#v", cfg.AllowedMIME)
		}
		if cfg.Retention.MaxCount != DefaultSessionAttachmentRetentionMaxCount ||
			cfg.Retention.MaxBytes != DefaultSessionAttachmentRetentionMaxBytes ||
			cfg.Retention.MaxAge != DefaultSessionAttachmentRetentionMaxAge {
			t.Fatalf("DefaultSessionAttachmentsConfig().Retention = %#v", cfg.Retention)
		}
		if err := cfg.Validate(); err != nil {
			t.Fatalf("DefaultSessionAttachmentsConfig().Validate() error = %v", err)
		}
	})

	cases := []struct {
		name    string
		mutate  func(*SessionAttachmentsConfig)
		wantErr string
	}{
		{
			name:    "Should reject a zero max file size",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.MaxFileBytes = 0 },
			wantErr: "session.attachments.max_file_bytes",
		},
		{
			name:    "Should reject a zero per-prompt count",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.MaxFilesPerPrompt = 0 },
			wantErr: "session.attachments.max_files_per_prompt",
		},
		{
			name:    "Should reject an empty allowlist",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.AllowedMIME = nil },
			wantErr: "session.attachments.allowed_mime",
		},
		{
			name:    "Should reject a type outside the v1 allowlist",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.AllowedMIME = []string{"image/gif"} },
			wantErr: "session.attachments.allowed_mime",
		},
		{
			name:    "Should reject a blank MIME",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.AllowedMIME = []string{" "} },
			wantErr: "session.attachments.allowed_mime",
		},
		{
			name:    "Should reject a duplicate MIME",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.AllowedMIME = []string{"image/png", "image/png"} },
			wantErr: "session.attachments.allowed_mime",
		},
		{
			name:    "Should reject a zero retention count",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.Retention.MaxCount = 0 },
			wantErr: "session.attachments.retention.max_count",
		},
		{
			name:    "Should reject zero retention bytes",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.Retention.MaxBytes = 0 },
			wantErr: "session.attachments.retention.max_bytes",
		},
		{
			name:    "Should reject zero retention age",
			mutate:  func(cfg *SessionAttachmentsConfig) { cfg.Retention.MaxAge = 0 },
			wantErr: "session.attachments.retention.max_age",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := DefaultSessionAttachmentsConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %s context", err, tc.wantErr)
			}
		})
	}
}

func TestSessionAttachmentsOverlayMerge(t *testing.T) {
	t.Parallel()

	t.Run("Should overlay attachment keys onto defaults", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		cfg := DefaultWithHome(homePaths)
		overlayPath := filepath.Join(t.TempDir(), "overlay.toml")
		writeFile(t, overlayPath, `
[session.attachments]
max_file_bytes = 1048576
max_files_per_prompt = 3
allowed_mime = ["image/png", "text/plain"]

[session.attachments.retention]
max_count = 12
max_bytes = 4096
max_age = "2h"
`)
		if err := ApplyConfigOverlayFile(overlayPath, &cfg); err != nil {
			t.Fatalf("ApplyConfigOverlayFile() error = %v", err)
		}
		got := cfg.Session.Attachments
		if got.MaxFileBytes != 1<<20 || got.MaxFilesPerPrompt != 3 {
			t.Fatalf("Attachments = %#v, want overlaid ceilings", got)
		}
		if !slices.Equal(got.AllowedMIME, []string{"image/png", "text/plain"}) {
			t.Fatalf("AllowedMIME = %#v", got.AllowedMIME)
		}
		if got.Retention.MaxCount != 12 || got.Retention.MaxBytes != 4096 || got.Retention.MaxAge != 2*time.Hour {
			t.Fatalf("Retention = %#v", got.Retention)
		}
	})
}
