package devcycle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/frontmatter"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestReviewArtifactStoreWrite(t *testing.T) {
	t.Run("Should write deterministic source agnostic review artifacts", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		output, err := fixedReviewArtifactStore().Write(
			t.Context(),
			reviewArtifactScope(workspace),
			writeReviewArtifactsInput{
				TaskName: "delivery",
				Issues: []ReviewIssue{
					{
						Title: "Avoid the race", Body: "Guard the shared map.", Severity: "high",
						File: "internal/store/cache.go", Line: 42, Author: "reviewer",
					},
					{
						Title: "Add regression coverage", Body: "The failure path is untested.",
						Severity: "medium",
					},
				},
			},
		)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if output.TaskName != "delivery" || output.Round != 1 || output.Count != 2 {
			t.Fatalf("Write() output = %#v", output)
		}
		assertReviewGolden(t, workspace, output.Files[0].Path, "testdata/review_artifacts/specific.md")
		assertReviewGolden(t, workspace, output.Files[1].Path, "testdata/review_artifacts/general.md")
		if got, want := output.Batches[1].File, unknownReviewFile; got != want {
			t.Fatalf("Batches[1].File = %q, want %q", got, want)
		}
	})

	t.Run("Should reject a missing task name before creating directories", func(t *testing.T) {
		t.Parallel()

		for _, taskName := range []string{"", "   "} {
			workspace := newReviewArtifactWorkspace(t)
			_, err := fixedReviewArtifactStore().Write(
				t.Context(),
				reviewArtifactScope(workspace),
				writeReviewArtifactsInput{
					TaskName: taskName,
					Issues:   []ReviewIssue{{Title: "Finding", Body: "Evidence", Severity: "high"}},
				},
			)
			if !errors.Is(err, errReviewArtifactInput) || !strings.Contains(err.Error(), "task_name") {
				t.Fatalf("Write(task_name=%q) error = %v, want structured task_name error", taskName, err)
			}
			entries, readErr := os.ReadDir(filepath.Join(workspace, reviewTasksDir))
			if readErr != nil {
				t.Fatalf("ReadDir(review tasks) error = %v", readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("review task entries = %v, want none", entries)
			}
		}
	})

	t.Run("Should reject incomplete review issues instead of inventing defaults", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name    string
			issue   ReviewIssue
			missing string
		}{
			{
				name: reviewIssueFieldTitle,
				issue: ReviewIssue{
					Body:     "Evidence",
					Severity: "high",
				},
				missing: reviewIssueFieldTitle,
			},
			{
				name: reviewIssueFieldBody,
				issue: ReviewIssue{
					Title:    "Finding",
					Severity: "high",
				},
				missing: reviewIssueFieldBody,
			},
			{
				name:    reviewIssueFieldSeverity,
				issue:   ReviewIssue{Title: "Finding", Body: "Evidence"},
				missing: reviewIssueFieldSeverity,
			},
		}
		for _, tc := range cases {
			t.Run("Should reject missing "+tc.name, func(t *testing.T) {
				t.Parallel()

				workspace := newReviewArtifactWorkspace(t)
				_, err := fixedReviewArtifactStore().Write(
					t.Context(),
					reviewArtifactScope(workspace),
					writeReviewArtifactsInput{TaskName: "delivery", Issues: []ReviewIssue{tc.issue}},
				)
				if !errors.Is(err, errReviewArtifactInput) || !strings.Contains(err.Error(), tc.missing) {
					t.Fatalf("Write() error = %v, want structured %s error", err, tc.missing)
				}
				deliveryDir := filepath.Join(workspace, reviewTasksDir, "delivery")
				if _, statErr := os.Stat(deliveryDir); !errors.Is(statErr, os.ErrNotExist) {
					t.Fatalf("Stat(delivery) error = %v, want not exist", statErr)
				}
			})
		}
	})

	t.Run("Should create no round for an empty clean review", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		output, err := fixedReviewArtifactStore().Write(
			t.Context(),
			reviewArtifactScope(workspace),
			writeReviewArtifactsInput{TaskName: "delivery", Issues: []ReviewIssue{}},
		)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if output.TaskName != "delivery" || output.Round != 0 || output.Count != 0 || len(output.Files) != 0 {
			t.Fatalf("Write() output = %#v, want empty clean review", output)
		}
		if _, err := os.Stat(filepath.Join(workspace, reviewTasksDir, "delivery")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("empty write task directory error = %v, want not exist", err)
		}
	})

	t.Run("Should discover gaps and append the next round", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		taskDir := filepath.Join(workspace, reviewTasksDir, "delivery")
		for _, round := range []string{"reviews-001", "reviews-003"} {
			if err := os.MkdirAll(filepath.Join(taskDir, round), 0o755); err != nil {
				t.Fatalf("MkdirAll(%s) error = %v", round, err)
			}
		}
		output, err := fixedReviewArtifactStore().Write(
			t.Context(),
			reviewArtifactScope(workspace),
			writeReviewArtifactsInput{
				TaskName: "delivery",
				Issues:   []ReviewIssue{{Title: "Finding", Body: "Evidence", Severity: "high"}},
			},
		)
		if err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if got, want := output.Round, 4; got != want {
			t.Fatalf("Write() round = %d, want %d", got, want)
		}
	})

	t.Run("Should allocate exclusive rounds to concurrent writers", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		store := fixedReviewArtifactStore()
		const writers = 2
		outputs := make(chan writeReviewArtifactsOutput, writers)
		errorsCh := make(chan error, writers)
		var group sync.WaitGroup
		for range writers {
			group.Go(func() {
				output, err := store.Write(
					t.Context(),
					reviewArtifactScope(workspace),
					writeReviewArtifactsInput{
						TaskName: "delivery",
						Issues: []ReviewIssue{{
							Title: "Finding", Body: "Evidence", Severity: "high",
						}},
					},
				)
				if err != nil {
					errorsCh <- err
					return
				}
				outputs <- output
			})
		}
		group.Wait()
		close(errorsCh)
		close(outputs)
		for err := range errorsCh {
			t.Fatalf("concurrent Write() error = %v", err)
		}
		rounds := make([]int, 0, writers)
		for output := range outputs {
			rounds = append(rounds, output.Round)
		}
		slices.Sort(rounds)
		if !slices.Equal(rounds, []int{1, 2}) {
			t.Fatalf("concurrent rounds = %v, want [1 2]", rounds)
		}
		for _, round := range rounds {
			entries, err := os.ReadDir(filepath.Join(workspace, reviewTasksDir, "delivery", reviewRoundDirName(round)))
			if err != nil {
				t.Fatalf("ReadDir(round %d) error = %v", round, err)
			}
			if len(entries) != 1 || entries[0].Name() != reviewIssueFileName(1) {
				t.Fatalf("round %d entries = %v, want one isolated issue", round, entries)
			}
		}
	})

	t.Run("Should append a new round without mutating earlier bytes", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		store := fixedReviewArtifactStore()
		first, err := store.Write(t.Context(), reviewArtifactScope(workspace), writeReviewArtifactsInput{
			TaskName: "delivery",
			Issues:   []ReviewIssue{{Title: "First", Body: "First evidence", Severity: "high"}},
		})
		if err != nil {
			t.Fatalf("first Write() error = %v", err)
		}
		firstPath := filepath.Join(workspace, first.Files[0].Path)
		firstBytes, err := os.ReadFile(firstPath)
		if err != nil {
			t.Fatalf("ReadFile(first) error = %v", err)
		}
		second, err := store.Write(t.Context(), reviewArtifactScope(workspace), writeReviewArtifactsInput{
			TaskName: "delivery",
			Issues:   []ReviewIssue{{Title: "First", Body: "First evidence", Severity: "high"}},
		})
		if err != nil {
			t.Fatalf("second Write() error = %v", err)
		}
		if second.Round != 2 {
			t.Fatalf("second round = %d, want 2", second.Round)
		}
		firstAfter, err := os.ReadFile(firstPath)
		if err != nil {
			t.Fatalf("ReadFile(first after) error = %v", err)
		}
		if !bytes.Equal(firstAfter, firstBytes) {
			t.Fatal("second write mutated the first round")
		}
	})
}

func TestReviewArtifactStoreContainment(t *testing.T) {
	t.Run("Should reject traversal and escaping symlinks", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		store := fixedReviewArtifactStore()
		_, err := store.Write(t.Context(), reviewArtifactScope(workspace), writeReviewArtifactsInput{
			TaskName: "../escape",
			Issues:   []ReviewIssue{{Title: "Finding", Body: "Evidence", Severity: "high"}},
		})
		if err == nil {
			t.Fatal("Write(traversal) error = nil")
		}

		outside := t.TempDir()
		taskLink := filepath.Join(workspace, reviewTasksDir, "linked")
		if err := os.Symlink(outside, taskLink); err != nil {
			t.Skipf("create task symlink: %v", err)
		}
		_, err = store.Write(t.Context(), reviewArtifactScope(workspace), writeReviewArtifactsInput{
			TaskName: "linked",
			Issues:   []ReviewIssue{{Title: "Finding", Body: "Evidence", Severity: "high"}},
		})
		if err == nil {
			t.Fatal("Write(task symlink) error = nil")
		}
		entries, readErr := os.ReadDir(outside)
		if readErr != nil {
			t.Fatalf("ReadDir(outside) error = %v", readErr)
		}
		if len(entries) != 0 {
			t.Fatalf("outside entries = %v, want none", entries)
		}
	})

	t.Run("Should reject a symlink selected as trusted workspace root", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		link := filepath.Join(t.TempDir(), "workspace-link")
		if err := os.Symlink(workspace, link); err != nil {
			t.Skipf("create workspace symlink: %v", err)
		}
		_, err := fixedReviewArtifactStore().Write(
			t.Context(),
			reviewArtifactScope(link),
			writeReviewArtifactsInput{
				TaskName: "delivery",
				Issues:   []ReviewIssue{{Title: "Finding", Body: "Evidence", Severity: "high"}},
			},
		)
		if err == nil {
			t.Fatal("Write(workspace symlink) error = nil")
		}
	})

	t.Run("Should isolate identical task names across trusted workspaces", func(t *testing.T) {
		t.Parallel()

		workspaces := []string{newReviewArtifactWorkspace(t), newReviewArtifactWorkspace(t)}
		store := fixedReviewArtifactStore()
		for index, workspace := range workspaces {
			_, err := store.Write(t.Context(), reviewArtifactScope(workspace), writeReviewArtifactsInput{
				TaskName: "delivery",
				Issues: []ReviewIssue{{
					Title: "Workspace finding",
					Body:  "Evidence " + string(rune('A'+index)), Severity: "high",
				}},
			})
			if err != nil {
				t.Fatalf("Write(workspace %d) error = %v", index, err)
			}
		}
		path := filepath.Join(reviewTasksDir, "delivery", "reviews-001", "issue_001.md")
		first, err := os.ReadFile(filepath.Join(workspaces[0], path))
		if err != nil {
			t.Fatalf("ReadFile(first workspace) error = %v", err)
		}
		second, err := os.ReadFile(filepath.Join(workspaces[1], path))
		if err != nil {
			t.Fatalf("ReadFile(second workspace) error = %v", err)
		}
		if bytes.Equal(first, second) {
			t.Fatal("isolated workspaces wrote indistinguishable cross-workspace content")
		}
	})
}

func TestReviewArtifactStoreFinalize(t *testing.T) {
	t.Run("Should resolve triaged statuses while preserving other bytes", func(t *testing.T) {
		t.Parallel()

		workspace, store, written := writeFinalizeFixture(t)
		validBefore := readReviewArtifact(t, workspace, written.Files[0].Path)
		invalidBefore := readReviewArtifact(t, workspace, written.Files[1].Path)

		output, err := store.Finalize(t.Context(), reviewArtifactScope(workspace), finalizeReviewRoundInput{
			TaskName: "delivery", Round: 1,
		})
		if err != nil {
			t.Fatalf("Finalize() error = %v", err)
		}
		if output.Resolved != 2 || output.Invalid != 1 || output.Pending != 1 {
			t.Fatalf("Finalize() = %#v", output)
		}
		assertOnlyReviewStatusChanged(t, validBefore, readReviewArtifact(t, workspace, written.Files[0].Path))
		assertOnlyReviewStatusChanged(t, invalidBefore, readReviewArtifact(t, workspace, written.Files[1].Path))
	})

	t.Run("Should leave pending files untouched and counted", func(t *testing.T) {
		t.Parallel()

		workspace, store, written := writeFinalizeFixture(t)
		pendingBefore := readReviewArtifact(t, workspace, written.Files[2].Path)
		output, err := store.Finalize(t.Context(), reviewArtifactScope(workspace), finalizeReviewRoundInput{
			TaskName: "delivery", Round: 1,
		})
		if err != nil {
			t.Fatalf("Finalize() error = %v", err)
		}
		if output.Pending != 1 {
			t.Fatalf("Finalize().Pending = %d, want 1", output.Pending)
		}
		if pendingAfter := readReviewArtifact(t, workspace, written.Files[2].Path); pendingAfter != pendingBefore {
			t.Fatal("Finalize() changed pending issue bytes")
		}
	})

	t.Run("Should be idempotent when finalizing an already finalized round", func(t *testing.T) {
		t.Parallel()

		workspace, store, _ := writeFinalizeFixture(t)
		input := finalizeReviewRoundInput{TaskName: "delivery", Round: 1}
		first, err := store.Finalize(t.Context(), reviewArtifactScope(workspace), input)
		if err != nil {
			t.Fatalf("first Finalize() error = %v", err)
		}
		second, err := store.Finalize(t.Context(), reviewArtifactScope(workspace), input)
		if err != nil {
			t.Fatalf("second Finalize() error = %v", err)
		}
		if second != first {
			t.Fatalf("second Finalize() = %#v, want %#v", second, first)
		}
	})

	t.Run("Should return deterministic task and round context when a round is absent", func(t *testing.T) {
		t.Parallel()

		workspace := newReviewArtifactWorkspace(t)
		if err := os.MkdirAll(filepath.Join(workspace, reviewTasksDir, "delivery"), 0o755); err != nil {
			t.Fatalf("MkdirAll(task) error = %v", err)
		}
		_, err := fixedReviewArtifactStore().Finalize(
			t.Context(),
			reviewArtifactScope(workspace),
			finalizeReviewRoundInput{TaskName: "delivery", Round: 7},
		)
		if !errors.Is(err, errReviewRoundNotFound) || !strings.Contains(err.Error(), `task "delivery" round 007`) {
			t.Fatalf("Finalize() error = %v, want deterministic not-found", err)
		}
	})
}

func writeFinalizeFixture(
	t *testing.T,
) (string, *reviewArtifactStore, writeReviewArtifactsOutput) {
	t.Helper()
	workspace := newReviewArtifactWorkspace(t)
	store := fixedReviewArtifactStore()
	written, err := store.Write(t.Context(), reviewArtifactScope(workspace), writeReviewArtifactsInput{
		TaskName: "delivery",
		Issues: []ReviewIssue{
			{Title: "Valid", Body: "Valid evidence", Severity: "high"},
			{Title: "Invalid", Body: "Invalid evidence", Severity: "medium"},
			{Title: "Pending", Body: "Pending evidence", Severity: "low"},
		},
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	rewriteReviewStatus(t, workspace, written.Files[0].Path, reviewStatusValid, "VALID")
	rewriteReviewStatus(t, workspace, written.Files[1].Path, reviewStatusInvalid, "INVALID")
	return workspace, store, written
}

func newReviewArtifactWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, reviewTasksDir), 0o755); err != nil {
		t.Fatalf("MkdirAll(review tasks) error = %v", err)
	}
	return workspace
}

func reviewArtifactScope(root string) *toolspkg.ExtensionToolWorkspaceScope {
	return &toolspkg.ExtensionToolWorkspaceScope{ID: "workspace-test", Root: root}
}

func fixedReviewArtifactStore() *reviewArtifactStore {
	return &reviewArtifactStore{now: func() time.Time {
		return time.Date(2026, time.July, 26, 12, 0, 0, 0, time.UTC)
	}}
}

func assertReviewGolden(t *testing.T, workspace string, relative string, golden string) {
	t.Helper()
	got := readReviewArtifact(t, workspace, relative)
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", golden, err)
	}
	if got != string(want) {
		t.Fatalf("artifact %s bytes mismatch\ngot:\n%s\nwant:\n%s", relative, got, want)
	}
}

func readReviewArtifact(t *testing.T, workspace string, relative string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(workspace, relative))
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", relative, err)
	}
	return string(content)
}

func rewriteReviewStatus(t *testing.T, workspace string, relative string, status string, decision string) {
	t.Helper()
	content := readReviewArtifact(t, workspace, relative)
	rewritten, err := frontmatter.RewriteStringField(content, "status", status)
	if err != nil {
		t.Fatalf("RewriteStringField(%s) error = %v", relative, err)
	}
	rewritten = strings.Replace(rewritten, "- Decision: `UNREVIEWED`", "- Decision: `"+decision+"`", 1)
	if err := os.WriteFile(filepath.Join(workspace, relative), []byte(rewritten), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", relative, err)
	}
}

func assertOnlyReviewStatusChanged(t *testing.T, before string, after string) {
	t.Helper()
	want := before
	replaced := false
	for _, status := range []string{reviewStatusValid, reviewStatusInvalid} {
		current := "status: " + status + "\n"
		if !strings.Contains(want, current) {
			continue
		}
		want = strings.Replace(want, current, "status: "+reviewStatusResolved+"\n", 1)
		replaced = true
		break
	}
	if !replaced {
		t.Fatalf("fixture has no triaged status:\n%s", before)
	}
	if after != want {
		t.Fatalf("finalized bytes changed beyond status\ngot:\n%s\nwant:\n%s", after, want)
	}
}
