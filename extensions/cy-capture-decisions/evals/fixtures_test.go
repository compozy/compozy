package evals

// Suite: model-backed eval fixture patches.
// Invariant: every documented diff patch applies to an empty scratch repository in filename order.
// Boundary IN: checked-in eval patches and the real Git patch parser.
// Boundary OUT: model execution and decision-log assertions, owned by the opt-in harness.

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/compozy/compozy/extensions/cy-capture-decisions/decisionlog"
)

func TestFixturePatchesApply(t *testing.T) {
	t.Parallel()

	fixtures, err := filepath.Glob(filepath.Join("fixtures", "*"))
	if err != nil {
		t.Fatalf("list fixture directories: %v", err)
	}
	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			t.Parallel()

			patches, globErr := filepath.Glob(filepath.Join(fixture, "*.patch"))
			if globErr != nil {
				t.Fatalf("list patches: %v", globErr)
			}
			if len(patches) == 0 {
				return
			}
			sort.Strings(patches)

			repo := t.TempDir()
			runGit(t, repo, "init", "-q", "-b", "main")
			for _, patch := range patches {
				absolute, absErr := filepath.Abs(patch)
				if absErr != nil {
					t.Fatalf("resolve patch %s: %v", patch, absErr)
				}
				runGit(t, repo, "apply", absolute)
			}
		})
	}
}

func TestFitnessHubExampleIsACompleteValidSupersessionChain(t *testing.T) {
	t.Parallel()

	root := filepath.Join("examples", "fitnesshub-web", ".compozy")
	if err := decisionlog.Validate(os.DirFS(root)); err != nil {
		t.Fatalf("validate real-world example: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
}
