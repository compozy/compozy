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
---

Review body.
`
	ctx, err := ParseReviewContext(reviewContent)
	if err != nil {
		t.Fatalf("parse review context: %v", err)
	}
	if ctx.Status != "resolved" || ctx.File != "internal/app/service.go" || ctx.Line != 42 {
		t.Fatalf("unexpected review context: %#v", ctx)
	}
	resolved, err := IsReviewResolved(reviewContent)
	if err != nil {
		t.Fatalf("is review resolved: %v", err)
	}
	if !resolved {
		t.Fatal("expected resolved review to be terminal")
	}

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
		"Legacy review body.",
		"",
	}, "\n")
	if !LooksLikeLegacyReviewFile(legacyContent) {
		t.Fatal("expected legacy review detection")
	}
	if _, err := ParseReviewContext(legacyContent); !errors.Is(err, ErrLegacyReviewMetadata) {
		t.Fatalf("expected legacy review sentinel, got %v", err)
	}

	legacyCtx, err := ParseLegacyReviewContext(legacyContent)
	if err != nil {
		t.Fatalf("parse legacy review context: %v", err)
	}
	if legacyCtx.Status != "pending" || legacyCtx.File != "internal/app/service.go" || legacyCtx.Line != 7 {
		t.Fatalf("unexpected legacy review context: %#v", legacyCtx)
	}

	legacyBody, err := ExtractLegacyReviewBody(legacyContent)
	if err != nil {
		t.Fatalf("extract legacy review body: %v", err)
	}
	if strings.Contains(legacyBody, "<review_context>") || strings.Contains(legacyBody, "## Status:") {
		t.Fatalf("expected legacy review body extraction to remove metadata, got:\n%s", legacyBody)
	}
	if !strings.Contains(legacyBody, "Legacy review body.") {
		t.Fatalf("expected legacy review body content to remain, got:\n%s", legacyBody)
	}
}

func TestExtractIssueNumber(t *testing.T) {
	t.Parallel()

	if got := ExtractIssueNumber("issue_042.md"); got != 42 {
		t.Fatalf("unexpected issue number: %d", got)
	}
}

func TestWrapParseErrorProvidesMigrationGuidance(t *testing.T) {
	t.Parallel()

	err := WrapParseError("/tmp/issue_001.md", ErrLegacyReviewMetadata)
	if !strings.Contains(err.Error(), "run `compozy migrate`") {
		t.Fatalf("expected migrate guidance, got %v", err)
	}
}
