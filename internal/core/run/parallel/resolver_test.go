package parallelrun

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
)

func TestConflictResolverSystemPromptIncludesSkillAndConflictContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", strings.Join([]string{
		"<<<<<<< HEAD",
		"first",
		"=======",
		"second",
		">>>>>>> task",
		"",
	}, "\n"))
	resolver := NewAgenticConflictResolution()
	prompt, err := resolver.buildConflictSystemPrompt(ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2, Title: "Resolve story"},
		CommitMessage:       "task 02: Resolve story",
		Attempt:             1,
		MaxAttempts:         3,
	})
	if err != nil {
		t.Fatalf("buildConflictSystemPrompt() error = %v", err)
	}
	for _, want := range []string{
		"Required embedded skill: git-rebase",
		"name: git-rebase",
		"conflicted_files: story.txt",
		"<<<<<<< HEAD",
		"task 02: Resolve story",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("system prompt missing %q\nprompt:\n%s", want, prompt)
		}
	}
}

func TestAgenticConflictResolutionDerivesResultFromStatusMarkersAndBuildGate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     string
		file       string
		buildErr   error
		want       ConflictResult
		wantBuilds int
	}{
		{
			name:       "clean status and passing build resolves",
			status:     " M story.txt\n",
			file:       "resolved\n",
			want:       ConflictResult{Resolved: true, Builds: true, Attempts: 1},
			wantBuilds: 1,
		},
		{
			name:       "unmerged status is unresolved without build",
			status:     "UU story.txt\n",
			file:       "resolved\n",
			want:       ConflictResult{Resolved: false, Builds: false, Attempts: 1},
			wantBuilds: 0,
		},
		{
			name:       "marker in file is unresolved without build",
			status:     " M story.txt\n",
			file:       "<<<<<<< HEAD\nfirst\n=======\nsecond\n>>>>>>> task\n",
			want:       ConflictResult{Resolved: false, Builds: false, Attempts: 1},
			wantBuilds: 0,
		},
		{
			name:       "build failure is clean but not buildable",
			status:     " M story.txt\n",
			file:       "resolved\n",
			buildErr:   errors.New("verify failed"),
			want:       ConflictResult{Resolved: true, Builds: false, Attempts: 1},
			wantBuilds: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeResolverTestFile(t, root, "story.txt", tt.file)
			runner := &fakeConflictCommandRunner{
				statuses: []string{tt.status},
				makeErrs: []error{tt.buildErr},
			}
			resolver := NewAgenticConflictResolution(
				WithConflictCommandRunner(runner),
				WithConflictPreparedPromptExecutor(successfulConflictPromptExecutor),
				WithConflictSkillFS(minimalGitRebaseSkillFS()),
			)
			got, err := resolver.Resolve(context.Background(), ConflictInput{
				IntegrationWorktree: root,
				Conflicts:           ConflictSet{Files: []string{"story.txt"}},
				Task:                TaskSpec{ID: "task_02", Number: 2, Title: "Story"},
				MaxAttempts:         1,
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Resolve() = %#v, want %#v", got, tt.want)
			}
			if runner.makeCalls != tt.wantBuilds {
				t.Fatalf("build calls = %d, want %d", runner.makeCalls, tt.wantBuilds)
			}
		})
	}
}

func TestAgenticConflictResolutionBoundsAttemptsAtThree(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	runner := &fakeConflictCommandRunner{statuses: []string{"UU story.txt\n"}}
	var calls int
	resolver := NewAgenticConflictResolution(
		WithConflictCommandRunner(runner),
		WithConflictPreparedPromptExecutor(func(
			context.Context,
			*model.RuntimeConfig,
			string,
			*reusableagents.ExecutionContext,
			execpkg.SessionMCPBuilder,
		) (execpkg.PreparedPromptResult, error) {
			calls++
			return execpkg.PreparedPromptResult{RunID: "resolver-run"}, nil
		}),
		WithConflictSkillFS(minimalGitRebaseSkillFS()),
	)
	got, err := resolver.Resolve(context.Background(), ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2},
		MaxAttempts:         4,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if calls != 3 {
		t.Fatalf("prompt executor calls = %d, want 3", calls)
	}
	if got.Attempts != 3 {
		t.Fatalf("attempts = %d, want 3", got.Attempts)
	}
}

func TestAgenticConflictResolutionBuildsRuntimeConfigWithAgentSelection(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	runner := &fakeConflictCommandRunner{statuses: []string{" M story.txt\n"}}
	var captured model.RuntimeConfig
	resolver := NewAgenticConflictResolution(
		WithConflictCommandRunner(runner),
		WithConflictPreparedPromptExecutor(func(
			_ context.Context,
			cfg *model.RuntimeConfig,
			_ string,
			_ *reusableagents.ExecutionContext,
			_ execpkg.SessionMCPBuilder,
		) (execpkg.PreparedPromptResult, error) {
			captured = *cfg
			return execpkg.PreparedPromptResult{RunID: "resolver-run"}, nil
		}),
		WithConflictSkillFS(minimalGitRebaseSkillFS()),
	)
	if _, err := resolver.Resolve(context.Background(), ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2},
		ParentRunID:         "parallel-run",
		MaxAttempts:         1,
		IDE:                 "claude",
		Model:               "sonnet",
		ReasoningEffort:     "high",
	}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if captured.WorkspaceRoot != root {
		t.Fatalf("WorkspaceRoot = %q, want %q", captured.WorkspaceRoot, root)
	}
	if captured.Mode != model.ExecutionModeExec {
		t.Fatalf("Mode = %q, want exec", captured.Mode)
	}
	if captured.IDE != "claude" || captured.Model != "sonnet" || captured.ReasoningEffort != "high" {
		t.Fatalf("agent selection = %s/%s/%s", captured.IDE, captured.Model, captured.ReasoningEffort)
	}
	if captured.ParentRunID != "parallel-run" {
		t.Fatalf("ParentRunID = %q, want parallel-run", captured.ParentRunID)
	}
	if !strings.Contains(captured.SystemPrompt, "name: git-rebase") {
		t.Fatalf("SystemPrompt missing embedded skill: %q", captured.SystemPrompt)
	}
}

type fakeConflictCommandRunner struct {
	statuses []string
	gitErrs  []error
	makeErrs []error

	gitCalls  int
	makeCalls int
}

func (r *fakeConflictCommandRunner) Git(_ context.Context, _ string, args ...string) (string, error) {
	if strings.Join(args, " ") != "status --porcelain" {
		return "", errors.New("unexpected git call")
	}
	idx := r.gitCalls
	r.gitCalls++
	status := valueAt(r.statuses, idx)
	return status, errorAtIndex(r.gitErrs, idx)
}

func (r *fakeConflictCommandRunner) Make(_ context.Context, _ string, args ...string) (string, error) {
	if strings.Join(args, " ") != "verify" {
		return "", errors.New("unexpected make call")
	}
	idx := r.makeCalls
	r.makeCalls++
	return "verify ok", errorAtIndex(r.makeErrs, idx)
}

func successfulConflictPromptExecutor(
	context.Context,
	*model.RuntimeConfig,
	string,
	*reusableagents.ExecutionContext,
	execpkg.SessionMCPBuilder,
) (execpkg.PreparedPromptResult, error) {
	return execpkg.PreparedPromptResult{RunID: "resolver-run"}, nil
}

func minimalGitRebaseSkillFS() fs.FS {
	return fstest.MapFS{
		gitRebaseSkillPath: &fstest.MapFile{
			Data: []byte("---\nname: git-rebase\ndescription: Test skill\n---\n# Test\n"),
		},
	}
}

func writeResolverTestFile(t *testing.T, root string, name string, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func valueAt(values []string, idx int) string {
	if idx < len(values) {
		return values[idx]
	}
	if len(values) == 0 {
		return ""
	}
	return values[len(values)-1]
}

func errorAtIndex(errs []error, idx int) error {
	if idx < len(errs) {
		return errs[idx]
	}
	return nil
}
