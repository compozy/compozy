package frontmatter

import (
	"strings"
	"testing"
)

func TestRewriteStringFieldPreservesBodyAndUnknownMetadata(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"---",
		"status: pending",
		"domain: backend",
		"custom_field: keep-me",
		"---",
		"",
		"# Title",
		"",
		"Body line",
		"",
	}, "\n")

	rewritten, err := RewriteStringField(content, "status", "completed")
	if err != nil {
		t.Fatalf("rewrite string field: %v", err)
	}

	if !strings.Contains(rewritten, "status: completed") {
		t.Fatalf("expected rewritten content to include updated status, got:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "custom_field: keep-me") {
		t.Fatalf("expected rewritten content to preserve unknown metadata, got:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "# Title\n\nBody line\n") {
		t.Fatalf("expected rewritten content to preserve body, got:\n%s", rewritten)
	}
}

func TestRewriteStringFieldAddsMissingField(t *testing.T) {
	t.Parallel()

	content := strings.Join([]string{
		"---",
		"domain: backend",
		"---",
		"",
		"# Title",
		"",
	}, "\n")

	rewritten, err := RewriteStringField(content, "status", "resolved")
	if err != nil {
		t.Fatalf("rewrite string field: %v", err)
	}

	if !strings.Contains(rewritten, "status: resolved") {
		t.Fatalf("expected rewritten content to add missing status field, got:\n%s", rewritten)
	}
	if !strings.Contains(rewritten, "domain: backend") {
		t.Fatalf("expected rewritten content to preserve existing metadata, got:\n%s", rewritten)
	}
}
