package worktree

import (
	"strings"
	"testing"
)

// Canonical suite: credential-free browser fallback URL shapes.
func TestExitBrowserURL(t *testing.T) {
	t.Parallel()

	t.Run("Should strip embedded remote credentials before any consumer sees the URL", func(t *testing.T) {
		t.Parallel()
		got := sanitizeGitRemote("https://secret-token@github.com/acme/repo.git?token=other#fragment")
		if strings.Contains(got, "secret-token") || strings.Contains(got, "token=") ||
			strings.Contains(got, "fragment") {
			t.Fatalf("sanitizeGitRemote() = %q, want credential-free URL", got)
		}
	})

	t.Run("Should preserve branch slashes as escaped compare segments", func(t *testing.T) {
		t.Parallel()
		got := browserCompareURL(
			[]string{"git@github.com:acme/repo.git"}, "release/next", "feature/auth", nil,
		)
		want := "https://github.com/acme/repo/compare/release%2Fnext...feature%2Fauth?expand=1"
		if got != want {
			t.Fatalf("browserCompareURL() = %q, want %q", got, want)
		}
	})

	t.Run("Should sanitize forge URLs before persistence or events", func(t *testing.T) {
		t.Parallel()
		got, err := sanitizeForgeWebURL(
			"https://secret@github.com/acme/repo/pull/42?access_token=other#fragment",
		)
		if err != nil || got != "https://github.com/acme/repo/pull/42" {
			t.Fatalf("sanitizeForgeWebURL() = %q, %v", got, err)
		}
		if _, err := sanitizeForgeWebURL("javascript:alert(1)"); err == nil {
			t.Fatal("sanitizeForgeWebURL(javascript) error = nil")
		}
	})

	t.Run("Should use the neutral remote root for an unknown host", func(t *testing.T) {
		t.Parallel()
		got := browserCompareURL([]string{"ssh://git@example.test/acme/repo.git"}, "main", "feature", nil)
		if got != "https://example.test/acme/repo" {
			t.Fatalf("browserCompareURL(unknown) = %q", got)
		}
	})
}
