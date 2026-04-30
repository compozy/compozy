package reviews

import (
	"errors"
	"strings"
	"testing"
)

func TestReviewParsingHelpers(t *testing.T) {
	t.Parallel()

	reviewContent := `---
status: resolved
file: internal/app/service.go
line: 42
severity: high
author: review-bot
provider_ref: thread:1
review_hash: abc123def456
source_review_id: 4089982130
source_review_submitted_at: 2026-04-10T13:33:25Z
---

Review body.
`
	legacyContent := strings.Join([]string{
		"# Issue 001",
		"",
		"## Status: pending",
		"",
		"<review_context>",
		"  <file>internal/app/service.go</file>",
		"  <line>7</line>",
		"  <severity>medium</severity>",
		"  <author>review-bot</author>",
		"  <provider_ref>thread:1</provider_ref>",
		"</review_context>",
		"",
		"Legacy review body with stray markup <file>ignored.md</file>.",
		"",
	}, "\n")

	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Should parse frontmatter review context",
			run: func(t *testing.T) {
				t.Helper()

				ctx, err := ParseReviewContext(reviewContent)
				if err != nil {
					t.Fatalf("parse review context: %v", err)
				}
				if ctx.Status != "resolved" || ctx.File != "internal/app/service.go" || ctx.Line != 42 {
					t.Fatalf("unexpected review context: %#v", ctx)
				}
				if ctx.ReviewHash != "abc123def456" {
					t.Fatalf("unexpected review hash: %q", ctx.ReviewHash)
				}
				if ctx.SourceReviewID != "4089982130" {
					t.Fatalf("unexpected source review id: %q", ctx.SourceReviewID)
				}
				if ctx.SourceReviewSubmittedAt != "2026-04-10T13:33:25Z" {
					t.Fatalf("unexpected source review submitted_at: %q", ctx.SourceReviewSubmittedAt)
				}
			},
		},
		{
			name: "Should report resolved frontmatter reviews as terminal",
			run: func(t *testing.T) {
				t.Helper()

				resolved, err := IsReviewResolved(reviewContent)
				if err != nil {
					t.Fatalf("is review resolved: %v", err)
				}
				if !resolved {
					t.Fatal("expected resolved review to be terminal")
				}
			},
		},
		{
			name: "Should detect legacy review files and return the sentinel parse error",
			run: func(t *testing.T) {
				t.Helper()

				if !LooksLikeLegacyReviewFile(legacyContent) {
					t.Fatal("expected legacy review detection")
				}
				if _, err := ParseReviewContext(legacyContent); !errors.Is(err, ErrLegacyReviewMetadata) {
					t.Fatalf("expected legacy review sentinel, got %v", err)
				}
			},
		},
		{
			name: "Should parse legacy review context only from the review_context block",
			run: func(t *testing.T) {
				t.Helper()

				legacyCtx, err := ParseLegacyReviewContext(legacyContent)
				if err != nil {
					t.Fatalf("parse legacy review context: %v", err)
				}
				if legacyCtx.Status != "pending" || legacyCtx.File != "internal/app/service.go" || legacyCtx.Line != 7 {
					t.Fatalf("unexpected legacy review context: %#v", legacyCtx)
				}
			},
		},
		{
			name: "Should remove legacy metadata while preserving the review body",
			run: func(t *testing.T) {
				t.Helper()

				legacyBody, err := ExtractLegacyReviewBody(legacyContent)
				if err != nil {
					t.Fatalf("extract legacy review body: %v", err)
				}
				if strings.Contains(legacyBody, "<review_context>") || strings.Contains(legacyBody, "## Status:") {
					t.Fatalf("expected legacy review body extraction to remove metadata, got:\n%s", legacyBody)
				}
				if !strings.Contains(legacyBody, "Legacy review body with stray markup <file>ignored.md</file>.") {
					t.Fatalf("expected legacy review body content to remain, got:\n%s", legacyBody)
				}
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestExtractIssueNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		want     int
	}{
		{
			name:     "Should extract the numeric issue id from a canonical review file name",
			filename: "issue_042.md",
			want:     42,
		},
		{
			name:     "Should return zero for non-review file names",
			filename: "notes.md",
			want:     0,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ExtractIssueNumber(tc.filename); got != tc.want {
				t.Fatalf("unexpected issue number: %d", got)
			}
		})
	}
}

func TestWrapParseErrorProvidesMigrationGuidance(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		err     error
		wantErr string
	}{
		{
			name:    "Should include migrate guidance for legacy review metadata",
			err:     ErrLegacyReviewMetadata,
			wantErr: "run `compozy migrate`",
		},
		{
			name:    "Should wrap non-legacy parse failures with the artifact path",
			err:     errors.New("boom"),
			wantErr: "parse review artifact /tmp/issue_001.md: boom",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := WrapParseError("/tmp/issue_001.md", tc.err)
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected wrapped error to include %q, got %v", tc.wantErr, err)
			}
			if !errors.Is(err, tc.err) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", err, tc.err)
			}
		})
	}

	if err := WrapParseError("/tmp/issue_001.md", nil); err != nil {
		t.Fatalf("WrapParseError(nil) = %v, want nil", err)
	}
}
