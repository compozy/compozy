package worktree

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Canonical suite: PR templates are selected unambiguously from regular blobs in the base tree.
func TestPRTemplateDetection(t *testing.T) {
	t.Parallel()

	t.Run("Should read the highest-priority regular template from the base tree", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
			switch strings.Join(call.args, " ") {
			case "ls-tree -r -z main":
				return gitResponse{stdout: []byte(
					"100644 blob abc\t.github/pull_request_template.md\x00" +
						"120000 blob def\tpull_request_template.md\x00",
				)}
			case "show main:.github/pull_request_template.md":
				return gitResponse{stdout: []byte("## Summary\n\nDescribe the change.\n")}
			default:
				return gitResponse{err: errors.New("unexpected template call")}
			}
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		if got := service.detectPRTemplate(
			context.Background(),
			"/repo",
			"main",
			nil,
		); got != "## Summary\n\nDescribe the change." {
			t.Fatalf("detectPRTemplate() = %q", got)
		}
	})

	t.Run("Should abstain when one priority level is ambiguous", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{{stdout: []byte(
			"100644 blob abc\t.github/pull_request_template.md\x00" +
				"100644 blob def\t.github/pull_request_template.txt\x00",
		)}}}
		service := NewService(newMemoryWorktreeStore(), runner)
		if got := service.detectPRTemplate(context.Background(), "/repo", "main", nil); got != "" {
			t.Fatalf("detectPRTemplate(ambiguous) = %q, want abstention", got)
		}
		if len(runner.invocations()) != 1 {
			t.Fatalf("ambiguous template Git calls = %#v, want no blob read", runner.invocations())
		}
	})

	t.Run("Should cap a large template at a valid UTF-8 boundary", func(t *testing.T) {
		t.Parallel()
		content := strings.Repeat("a", prTemplateMaxBytes-1) + "é" + strings.Repeat("b", 50)
		runner := &recordingGitRunner{responses: []gitResponse{
			{stdout: []byte("100644 blob abc\tpull_request_template.md\x00")},
			{stdout: []byte(content)},
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		got := service.detectPRTemplate(context.Background(), "/repo", "main", nil)
		if len(got) > prTemplateMaxBytes || !strings.HasSuffix(got, "a") {
			t.Fatalf("bounded template length/suffix = %d/%q", len(got), got[len(got)-1:])
		}
	})

	t.Run("Should use provider template paths in declared priority order", func(t *testing.T) {
		t.Parallel()
		runner := &recordingGitRunner{responses: []gitResponse{
			{stdout: []byte(
				"100644 blob abc\t.github/pull_request_template.md\x00" +
					"100644 blob def\t.forge/review.md\x00",
			)},
			{stdout: []byte("Provider-owned template\n")},
		}}
		service := NewService(newMemoryWorktreeStore(), runner)
		got := service.detectPRTemplate(
			context.Background(), "/repo", "main", []string{".forge/review.md"},
		)
		if got != "Provider-owned template" {
			t.Fatalf("detectPRTemplate(provider paths) = %q", got)
		}
		calls := runner.invocations()
		if len(calls) != 2 || strings.Join(calls[1].args, " ") != "show main:.forge/review.md" {
			t.Fatalf("template calls = %#v, want provider path", calls)
		}
	})
}
