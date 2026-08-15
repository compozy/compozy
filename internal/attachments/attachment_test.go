package attachments

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseAttachmentURI(t *testing.T) {
	t.Parallel()

	validID := "att_" + strings.Repeat("ab", 32)
	cases := []struct {
		name    string
		uri     string
		wantID  string
		wantErr bool
	}{
		{
			name:   "Should accept a canonical attachment URI",
			uri:    AttachmentURIPrefix + validID,
			wantID: validID,
		},
		{
			name:    "Should reject surrounding whitespace",
			uri:     " " + AttachmentURIPrefix + validID,
			wantErr: true,
		},
		{
			name:    "Should reject a foreign URI prefix",
			uri:     "compozy://tool-artifacts/" + validID,
			wantErr: true,
		},
		{
			name:    "Should reject a non-hex identity",
			uri:     AttachmentURIPrefix + "att_" + strings.Repeat("zz", 32),
			wantErr: true,
		},
		{
			name:    "Should reject a path traversal identity",
			uri:     AttachmentURIPrefix + "../secret",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseAttachmentURI(tc.uri)
			if tc.wantErr {
				if err == nil {
					t.Fatal("ParseAttachmentURI() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAttachmentURI() error = %v", err)
			}
			if got != tc.wantID {
				t.Fatalf("ParseAttachmentURI() = %q, want %q", got, tc.wantID)
			}
		})
	}
}

func TestAttachmentRetentionValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mutate  func(*AttachmentRetention)
		wantErr string
	}{
		{
			name:    "Should reject a zero max count",
			mutate:  func(r *AttachmentRetention) { r.MaxCount = 0 },
			wantErr: "max count",
		},
		{
			name:    "Should reject a zero max bytes",
			mutate:  func(r *AttachmentRetention) { r.MaxBytes = 0 },
			wantErr: "max bytes",
		},
		{
			name:    "Should reject a zero max age",
			mutate:  func(r *AttachmentRetention) { r.MaxAge = 0 },
			wantErr: "max age",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			retention := AttachmentRetention{MaxCount: 2, MaxBytes: 1024, MaxAge: time.Hour}
			tc.mutate(&retention)
			err := retention.Validate()
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %q", err, tc.wantErr)
			}
		})
	}

	t.Run("Should accept finite positive bounds", func(t *testing.T) {
		t.Parallel()
		retention := AttachmentRetention{MaxCount: 2, MaxBytes: 1024, MaxAge: time.Hour}
		if err := retention.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})
}

func TestValidateSize(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		bytes        int64
		maxFileBytes int64
		wantErr      error
	}{
		{name: "Should accept a file at the configured ceiling", bytes: 8, maxFileBytes: 8},
		{name: "Should reject a file over the configured ceiling", bytes: 9, maxFileBytes: 8, wantErr: ErrTooLarge},
		{name: "Should reject a non-positive ceiling", bytes: 1, maxFileBytes: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateSize(tc.bytes, tc.maxFileBytes)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("ValidateSize() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if tc.maxFileBytes <= 0 {
				if err == nil {
					t.Fatal("ValidateSize() error = nil, want invalid ceiling")
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateSize() error = %v", err)
			}
		})
	}
}
