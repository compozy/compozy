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

func TestAgenticConflictResolutionScenarios(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should include the embedded skill and conflict context in the system prompt",
		runConflictResolverSystemPromptIncludesSkillAndConflictContext,
	)
	t.Run(
		"Should derive the conflict result from git status, markers, and optional validation",
		runAgenticConflictResolutionDerivesResultFromStatusMarkersAndValidation,
	)
	t.Run(
		"Should reject validation commands that add paths to the worktree",
		runAgenticConflictResolutionRejectsMutatingValidationCommand,
	)
	t.Run(
		"Should reject validation commands that mutate an existing diff",
		runAgenticConflictResolutionRejectsValidationCommandThatMutatesExistingDiff,
	)
	t.Run(
		"Should reject real validation commands that mutate the git worktree",
		runAgenticConflictResolutionRejectsRealMutatingValidationCommand,
	)
	t.Run("Should bound resolver attempts at three", runAgenticConflictResolutionBoundsAttemptsAtThree)
	t.Run(
		"Should clamp an oversized starting attempt to the bounded maximum",
		runAgenticConflictResolutionClampsStartingAttemptToBoundedMax,
	)
	t.Run("Should reject a nil context at the resolver boundary", runAgenticConflictResolutionRejectsNilContext)
	t.Run(
		"Should build the runtime config with the selected agent settings",
		runAgenticConflictResolutionBuildsRuntimeConfigWithAgentSelection,
	)
	t.Run(
		"Should tolerate conflicted symlink-to-directory paths",
		runAgenticConflictResolutionToleratesSymlinkToDirectoryConflict,
	)
	t.Run(
		"Should return regular file read errors while scanning conflict markers",
		runConflictMarkersPresentReturnsRegularFileReadErrors,
	)
}

func runConflictResolverSystemPromptIncludesSkillAndConflictContext(t *testing.T) {
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
	for _, forbidden := range []string{"make verify", "Run make verify"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("system prompt contains hardcoded validation %q\nprompt:\n%s", forbidden, prompt)
		}
	}
}

func runAgenticConflictResolutionDerivesResultFromStatusMarkersAndValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		statuses          []string
		file              string
		validationCommand []string
		validationErr     error
		want              ConflictResult
		wantRuns          int
	}{
		{
			name:     "Should resolve without running project validation by default",
			statuses: []string{" M story.txt\n"},
			file:     "resolved\n",
			want:     ConflictResult{Resolved: true, Validated: true, Attempts: 1},
		},
		{
			name:     "Should stay unresolved when git status still reports unmerged files",
			statuses: []string{"UU story.txt\n"},
			file:     "resolved\n",
			want:     ConflictResult{Resolved: false, Validated: false, Attempts: 1},
		},
		{
			name:     "Should stay unresolved when conflict markers remain in the file",
			statuses: []string{" M story.txt\n"},
			file:     "<<<<<<< HEAD\nfirst\n=======\nsecond\n>>>>>>> task\n",
			want:     ConflictResult{Resolved: false, Validated: false, Attempts: 1},
		},
		{
			name:              "Should run optional validation when configured",
			statuses:          []string{" M story.txt\n", " M story.txt\n"},
			file:              "resolved\n",
			validationCommand: []string{"go", "test", "./..."},
			want:              ConflictResult{Resolved: true, Validated: true, Attempts: 1},
			wantRuns:          1,
		},
		{
			name:              "Should report failed validation when the optional command fails",
			statuses:          []string{" M story.txt\n", " M story.txt\n"},
			file:              "resolved\n",
			validationCommand: []string{"go", "test", "./..."},
			validationErr:     errors.New("validation failed"),
			want: ConflictResult{
				Resolved:        true,
				Validated:       false,
				Attempts:        1,
				ValidationError: "validation failed",
			},
			wantRuns: 1,
		},
	}

	for idx := range tests {
		idx := idx
		t.Run(tests[idx].name, func(t *testing.T) {
			t.Parallel()
			tt := &tests[idx]
			root := t.TempDir()
			writeResolverTestFile(t, root, "story.txt", tt.file)
			runner := &fakeConflictCommandRunner{
				statuses: tt.statuses,
				runErrs:  []error{tt.validationErr},
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
				ValidationCommand:   tt.validationCommand,
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Resolve() = %#v, want %#v", got, tt.want)
			}
			if runner.runCalls != tt.wantRuns {
				t.Fatalf("validation calls = %d, want %d", runner.runCalls, tt.wantRuns)
			}
		})
	}
}

func runAgenticConflictResolutionRejectsMutatingValidationCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	runner := &fakeConflictCommandRunner{
		statuses: []string{" M story.txt\n", " M story.txt\n M generated.txt\n"},
	}
	resolver := NewAgenticConflictResolution(
		WithConflictCommandRunner(runner),
		WithConflictPreparedPromptExecutor(successfulConflictPromptExecutor),
		WithConflictSkillFS(minimalGitRebaseSkillFS()),
	)
	_, err := resolver.Resolve(context.Background(), ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2, Title: "Story"},
		MaxAttempts:         1,
		ValidationCommand:   []string{"go", "generate", "./..."},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want mutating validation rejection")
	}
	if !strings.Contains(err.Error(), "modified the integration worktree") {
		t.Fatalf("Resolve() error = %v, want mutation rejection", err)
	}
}

func runAgenticConflictResolutionRejectsValidationCommandThatMutatesExistingDiff(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	runner := &fakeConflictCommandRunner{
		statuses: []string{" M story.txt\n", " M story.txt\n"},
		gitOutputs: map[string][]string{
			"diff --no-ext-diff --binary": {
				"diff --git a/story.txt b/story.txt\n--- a/story.txt\n+++ b/story.txt\n@@\n-resolved\n+resolved\n",
				"diff --git a/story.txt b/story.txt\n--- a/story.txt\n+++ b/story.txt\n@@\n-resolved\n+rewritten\n",
			},
		},
	}
	resolver := NewAgenticConflictResolution(
		WithConflictCommandRunner(runner),
		WithConflictPreparedPromptExecutor(successfulConflictPromptExecutor),
		WithConflictSkillFS(minimalGitRebaseSkillFS()),
	)
	_, err := resolver.Resolve(context.Background(), ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2, Title: "Story"},
		MaxAttempts:         1,
		ValidationCommand:   []string{"go", "generate", "./..."},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want mutating validation rejection")
	}
	if !strings.Contains(err.Error(), "modified the integration worktree") {
		t.Fatalf("Resolve() error = %v, want mutation rejection", err)
	}
}

func runAgenticConflictResolutionRejectsRealMutatingValidationCommand(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	runResolverTestGit(t, root, "init", "-b", "main")
	runResolverTestGit(t, root, "config", "user.email", "resolver@example.test")
	runResolverTestGit(t, root, "config", "user.name", "Resolver Test")
	writeResolverTestFile(t, root, "story.txt", "base\n")
	runResolverTestGit(t, root, "add", "story.txt")
	runResolverTestGit(t, root, "commit", "-m", "base")
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	runResolverTestGit(t, root, "add", "story.txt")
	script := filepath.Join(t.TempDir(), "mutate-validation.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho generated > generated.txt\n"), 0o700); err != nil {
		t.Fatalf("write validation script: %v", err)
	}
	resolver := NewAgenticConflictResolution(
		WithConflictPreparedPromptExecutor(successfulConflictPromptExecutor),
		WithConflictSkillFS(minimalGitRebaseSkillFS()),
	)
	_, err := resolver.Resolve(context.Background(), ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2, Title: "Story"},
		MaxAttempts:         1,
		ValidationCommand:   []string{script},
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want mutating validation rejection")
	}
	if !strings.Contains(err.Error(), "modified the integration worktree") {
		t.Fatalf("Resolve() error = %v, want mutation rejection", err)
	}
}

func runConflictMarkersPresentReturnsRegularFileReadErrors(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can read mode 000 files")
	}
	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "<<<<<<< HEAD\n")
	path := filepath.Join(root, "story.txt")
	if err := os.Chmod(path, 0); err != nil {
		t.Fatalf("chmod unreadable file: %v", err)
	}
	defer func() {
		if err := os.Chmod(path, 0o600); err != nil {
			t.Fatalf("restore unreadable file mode: %v", err)
		}
	}()

	present, err := conflictMarkersPresent(root, []string{"story.txt"})
	if err == nil {
		t.Fatalf("conflictMarkersPresent() error = nil, present=%t, want read error", present)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("conflictMarkersPresent() error = %v, want permission error", err)
	}
}

func runAgenticConflictResolutionBoundsAttemptsAtThree(t *testing.T) {
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

func runAgenticConflictResolutionClampsStartingAttemptToBoundedMax(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	runner := &fakeConflictCommandRunner{statuses: []string{" M story.txt\n"}}
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
		Attempt:             9,
		MaxAttempts:         4,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("prompt executor calls = %d, want 1", calls)
	}
	if got.Attempts != 3 {
		t.Fatalf("attempts = %d, want 3", got.Attempts)
	}
}

func runAgenticConflictResolutionRejectsNilContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeResolverTestFile(t, root, "story.txt", "resolved\n")
	resolver := NewAgenticConflictResolution(
		WithConflictCommandRunner(&fakeConflictCommandRunner{statuses: []string{" M story.txt\n"}}),
		WithConflictPreparedPromptExecutor(successfulConflictPromptExecutor),
		WithConflictSkillFS(minimalGitRebaseSkillFS()),
	)

	_, err := resolver.Resolve(nilContextForTest(), ConflictInput{
		IntegrationWorktree: root,
		Conflicts:           ConflictSet{Files: []string{"story.txt"}},
		Task:                TaskSpec{ID: "task_02", Number: 2},
		MaxAttempts:         1,
	})
	if err == nil {
		t.Fatal("Resolve() error = nil, want nil-context rejection")
	}
	if !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("Resolve() error = %v, want nil-context rejection", err)
	}
}

func runAgenticConflictResolutionBuildsRuntimeConfigWithAgentSelection(t *testing.T) {
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

func runAgenticConflictResolutionToleratesSymlinkToDirectoryConflict(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	target := filepath.Join(root, ".agents", "skills", "review")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir symlink target: %v", err)
	}
	link := filepath.Join(root, ".claude", "skills", "review")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir symlink dir: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	var calls int
	resolver := NewAgenticConflictResolution(
		WithConflictCommandRunner(&fakeConflictCommandRunner{statuses: []string{" M .claude/skills/review\n"}}),
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
		Conflicts:           ConflictSet{Files: []string{".claude/skills/review"}},
		Task:                TaskSpec{ID: "task_02", Number: 2},
		MaxAttempts:         1,
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("prompt executor calls = %d, want 1", calls)
	}
	if got != (ConflictResult{Resolved: true, Validated: true, Attempts: 1}) {
		t.Fatalf("Resolve() = %#v, want resolved validation pass", got)
	}
}

type fakeConflictCommandRunner struct {
	statuses   []string
	gitOutputs map[string][]string
	gitErrs    []error
	runOutputs []string
	runErrs    []error

	gitCalls       int
	gitCallsByArgs map[string]int
	runCalls       int
}

func (r *fakeConflictCommandRunner) Git(_ context.Context, _ string, args ...string) (string, error) {
	key := strings.Join(args, " ")
	if key == "status --porcelain" {
		idx := r.gitCalls
		r.gitCalls++
		status := valueAt(r.statuses, idx)
		return status, errorAtIndex(r.gitErrs, idx)
	}
	if key != "diff --no-ext-diff --binary" && key != "diff --cached --no-ext-diff --binary" {
		return "", errors.New("unexpected git call")
	}
	if r.gitCallsByArgs == nil {
		r.gitCallsByArgs = make(map[string]int)
	}
	idx := r.gitCallsByArgs[key]
	r.gitCallsByArgs[key] = idx + 1
	return valueAt(r.gitOutputs[key], idx), nil
}

func (r *fakeConflictCommandRunner) Run(_ context.Context, _ string, command []string) (string, error) {
	if len(command) == 0 {
		return "", errors.New("unexpected empty validation command")
	}
	idx := r.runCalls
	r.runCalls++
	return valueAt(r.runOutputs, idx), errorAtIndex(r.runErrs, idx)
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

func runResolverTestGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runConflictCommand(context.Background(), dir, "git", args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
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

func nilContextForTest() context.Context {
	return nil
}
