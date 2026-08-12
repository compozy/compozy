package worktree

import (
	"encoding/hex"
	"strings"
	"testing"
)

// Canonical suite: worktree naming and recorded-namespace behavior.
func TestNaming(t *testing.T) {
	t.Parallel()

	t.Run("Should derive one slug for the branch and directory", func(t *testing.T) {
		t.Parallel()
		got := DeriveNames("Docs Refresh!")
		if got.Branch != "docs-refresh" || got.Directory != "docs-refresh" {
			t.Fatalf("DeriveNames() = %#v, want docs-refresh for both fields", got)
		}
	})

	t.Run("Should use a deterministic fallback for invalid input", func(t *testing.T) {
		t.Parallel()
		first := DeriveNames("///___")
		second := DeriveNames("///___")
		if first != second || first.Branch != fallbackSlug {
			t.Fatalf("DeriveNames() = %#v and %#v, want deterministic %q", first, second, fallbackSlug)
		}
	})

	t.Run("Should suffix a generated name when its base is taken", func(t *testing.T) {
		t.Parallel()
		base := GenerateName(nil)
		got := GenerateName(func(candidate string) bool { return candidate == base })
		if got != base+"-2" {
			t.Fatalf("GenerateName() = %q, want %q", got, base+"-2")
		}
	})

	t.Run("Should derive distinct per-run branch suffixes", func(t *testing.T) {
		t.Parallel()
		first := RunBranchName("run/", "fix-flaky-e2e", "run-1")
		second := RunBranchName("run/", "fix-flaky-e2e", "run-2")
		if !strings.HasPrefix(first, "run/fix-flaky-e2e-") || first == second {
			t.Fatalf("RunBranchName() = %q and %q, want distinct names in run/", first, second)
		}
	})

	t.Run("Should classify reclamation from the recorded namespace", func(t *testing.T) {
		t.Parallel()
		if !ReclamationEligible("run/task-abc", "run/", true) {
			t.Fatal("ReclamationEligible() = false, want true for recorded namespace")
		}
		if ReclamationEligible("wt/task-abc", "run/", true) {
			t.Fatal("ReclamationEligible() = true, want no retroactive eligibility")
		}
	})

	t.Run("Should mint opaque worktree IDs with the domain prefix", func(t *testing.T) {
		t.Parallel()
		first, err := generateID("wt")
		if err != nil {
			t.Fatalf("generateID() error = %v", err)
		}
		second, err := generateID("wt")
		if err != nil {
			t.Fatalf("generateID(second) error = %v", err)
		}
		if !strings.HasPrefix(first, "wt_") || first == second {
			t.Fatalf("generateID() = %q and %q, want distinct wt_ IDs", first, second)
		}
		if _, err := hex.DecodeString(strings.TrimPrefix(first, "wt_")); err != nil {
			t.Fatalf("generateID() entropy = %q, want hex: %v", first, err)
		}
	})
}
