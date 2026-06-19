package parallelrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/skills"
)

const (
	gitRebaseSkillPath       = "git-rebase/SKILL.md"
	conflictResolverMakeGoal = "verify"
	maxConflictHunkBytes     = 24 * 1024
)

// ConflictResolver resolves an integration-branch squash conflict.
type ConflictResolver interface {
	Resolve(ctx context.Context, in ConflictInput) (ConflictResult, error)
}

// ConflictInput is the complete context for one bounded conflict-resolution
// cycle.
type ConflictInput struct {
	IntegrationWorktree string
	Conflicts           ConflictSet
	Task                TaskSpec
	CommitMessage       string
	ParentRunID         string
	Attempt             int
	MaxAttempts         int
	IDE                 string
	Model               string
	ReasoningEffort     string
}

// ConflictResult is derived after each resolver attempt from git status,
// conflict-marker checks, and the build gate.
type ConflictResult struct {
	Resolved bool
	Builds   bool
	Attempts int
}

type conflictPreparedPromptExecutor func(
	context.Context,
	*model.RuntimeConfig,
	string,
	*reusableagents.ExecutionContext,
	execpkg.SessionMCPBuilder,
) (execpkg.PreparedPromptResult, error)

type conflictCommandRunner interface {
	Git(ctx context.Context, dir string, args ...string) (string, error)
	Make(ctx context.Context, dir string, args ...string) (string, error)
}

// AgenticConflictResolution launches the configured agent through the exec
// prompt path and validates the resulting worktree state after each attempt.
type AgenticConflictResolution struct {
	executePreparedPrompt conflictPreparedPromptExecutor
	commands              conflictCommandRunner
	skillFS               fs.FS
}

// ConflictResolutionOption configures AgenticConflictResolution.
type ConflictResolutionOption func(*AgenticConflictResolution)

// WithConflictPreparedPromptExecutor overrides the ACP exec path for tests.
func WithConflictPreparedPromptExecutor(fn conflictPreparedPromptExecutor) ConflictResolutionOption {
	return func(resolver *AgenticConflictResolution) {
		if fn != nil {
			resolver.executePreparedPrompt = fn
		}
	}
}

// WithConflictCommandRunner overrides git and make commands for tests.
func WithConflictCommandRunner(runner conflictCommandRunner) ConflictResolutionOption {
	return func(resolver *AgenticConflictResolution) {
		if runner != nil {
			resolver.commands = runner
		}
	}
}

// WithConflictSkillFS overrides the embedded skill source for tests.
func WithConflictSkillFS(skillFS fs.FS) ConflictResolutionOption {
	return func(resolver *AgenticConflictResolution) {
		if skillFS != nil {
			resolver.skillFS = skillFS
		}
	}
}

// NewAgenticConflictResolution constructs the default conflict resolver.
func NewAgenticConflictResolution(opts ...ConflictResolutionOption) *AgenticConflictResolution {
	resolver := &AgenticConflictResolution{
		executePreparedPrompt: execpkg.ExecutePreparedPrompt,
		commands:              osConflictCommandRunner{},
		skillFS:               skills.FS,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(resolver)
		}
	}
	return resolver
}

var _ ConflictResolver = (*AgenticConflictResolution)(nil)

// Resolve runs the selected agent for a bounded number of attempts and
// validates the integration worktree after every attempt.
func (r *AgenticConflictResolution) Resolve(ctx context.Context, in ConflictInput) (ConflictResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if r == nil || r.executePreparedPrompt == nil {
		return ConflictResult{}, errors.New("conflict resolver: missing prompt executor")
	}
	if r.commands == nil {
		return ConflictResult{}, errors.New("conflict resolver: missing command runner")
	}
	root := strings.TrimSpace(in.IntegrationWorktree)
	if root == "" {
		return ConflictResult{}, errors.New("conflict resolver: integration worktree is required")
	}
	maxAttempts := boundedConflictAttempts(in.MaxAttempts)
	startAttempt := in.Attempt
	if startAttempt < 1 {
		startAttempt = 1
	}
	var last ConflictResult
	var lastErr error
	for attempt := startAttempt; attempt <= maxAttempts; attempt++ {
		attemptInput := in
		attemptInput.IntegrationWorktree = root
		attemptInput.Attempt = attempt
		attemptInput.MaxAttempts = maxAttempts
		systemPrompt, err := r.buildConflictSystemPrompt(attemptInput)
		if err != nil {
			return last, err
		}
		runtimeCfg := buildConflictRuntimeConfig(attemptInput, systemPrompt)
		_, runErr := r.executePreparedPrompt(ctx, &runtimeCfg, buildConflictPrompt(), nil, nil)
		result, inspectErr := r.evaluateConflictResult(ctx, attemptInput)
		result.Attempts = attempt
		last = result
		if inspectErr != nil {
			return last, inspectErr
		}
		if runErr != nil {
			lastErr = runErr
		}
		if result.Resolved && result.Builds {
			return result, nil
		}
		if err := ctx.Err(); err != nil {
			return last, err
		}
	}
	if lastErr != nil {
		return last, fmt.Errorf("conflict resolver exhausted after %d attempt(s): %w", last.Attempts, lastErr)
	}
	return last, nil
}

func (r *AgenticConflictResolution) buildConflictSystemPrompt(in ConflictInput) (string, error) {
	if r.skillFS == nil {
		return "", errors.New("conflict resolver: missing skill filesystem")
	}
	skill, err := fs.ReadFile(r.skillFS, gitRebaseSkillPath)
	if err != nil {
		return "", fmt.Errorf("read embedded git-rebase skill: %w", err)
	}
	contextSection, err := buildConflictContextSection(in)
	if err != nil {
		return "", err
	}
	sections := []string{
		"You are the Compozy agentic merge-conflict resolver.",
		"Required embedded skill: git-rebase.",
		"<git-rebase-skill>\n" + strings.TrimSpace(string(skill)) + "\n</git-rebase-skill>",
		contextSection,
		strings.Join([]string{
			"Hard constraints:",
			"- Resolve only conflicts you understand.",
			"- Do not commit; Compozy creates the squash commit after validation.",
			"- Stage resolved files with git add so git status has no unmerged entries.",
			"- Never leave conflict markers in any file.",
			"- Run make verify and leave the conflict unresolved if it does not pass.",
		}, "\n"),
	}
	return strings.Join(nonEmptyConflictSections(sections), "\n\n"), nil
}

func buildConflictPrompt() string {
	return strings.Join([]string{
		"Resolve the merge conflicts described in your system prompt.",
		"Edit the integration worktree only.",
		"Stage resolved files, run make verify, and do not commit.",
		"If the resolution is unsafe, leave the conflict unresolved and explain why.",
	}, "\n")
}

func buildConflictRuntimeConfig(in ConflictInput, systemPrompt string) model.RuntimeConfig {
	cfg := model.RuntimeConfig{
		WorkspaceRoot:      strings.TrimSpace(in.IntegrationWorktree),
		Mode:               model.ExecutionModeExec,
		OutputFormat:       model.OutputFormatText,
		TUI:                false,
		Persist:            true,
		DaemonOwned:        true,
		ParentRunID:        strings.TrimSpace(in.ParentRunID),
		SystemPrompt:       systemPrompt,
		Recursive:          false,
		PromptText:         "",
		ResolvedPromptText: "",
		PromptFile:         "",
		ReadPromptStdin:    false,
	}
	if value := strings.TrimSpace(in.IDE); value != "" {
		cfg.IDE = value
	}
	if value := strings.TrimSpace(in.Model); value != "" {
		cfg.Model = value
	}
	if value := strings.TrimSpace(in.ReasoningEffort); value != "" {
		cfg.ReasoningEffort = value
	}
	cfg.ApplyDefaults()
	return cfg
}

func (r *AgenticConflictResolution) evaluateConflictResult(
	ctx context.Context,
	in ConflictInput,
) (ConflictResult, error) {
	status, err := r.commands.Git(ctx, in.IntegrationWorktree, "status", "--porcelain")
	if err != nil {
		return ConflictResult{}, fmt.Errorf("inspect conflict status: %w", err)
	}
	if len(unmergedFilesFromPorcelain(status)) > 0 {
		return ConflictResult{Resolved: false, Builds: false}, nil
	}
	hasMarkers, err := conflictMarkersPresent(in.IntegrationWorktree, in.Conflicts.Files)
	if err != nil {
		return ConflictResult{}, err
	}
	if hasMarkers {
		return ConflictResult{Resolved: false, Builds: false}, nil
	}
	if _, err := r.commands.Make(ctx, in.IntegrationWorktree, conflictResolverMakeGoal); err != nil {
		return ConflictResult{Resolved: true, Builds: false}, nil
	}
	return ConflictResult{Resolved: true, Builds: true}, nil
}

func buildConflictContextSection(in ConflictInput) (string, error) {
	var b strings.Builder
	b.WriteString("Conflict context:\n")
	writeConflictContextLine(&b, "integration_worktree", in.IntegrationWorktree)
	writeConflictContextLine(&b, "task_id", string(in.Task.ID))
	if in.Task.Number > 0 {
		writeConflictContextLine(&b, "task_number", strconv.Itoa(in.Task.Number))
	}
	writeConflictContextLine(&b, "task_title", in.Task.Title)
	writeConflictContextLine(&b, "commit_message", in.CommitMessage)
	if in.Attempt > 0 && in.MaxAttempts > 0 {
		writeConflictContextLine(
			&b,
			"attempt",
			fmt.Sprintf("%d/%d", in.Attempt, in.MaxAttempts),
		)
	}
	files := normalizedConflictFiles(in.Conflicts.Files)
	if len(files) > 0 {
		writeConflictContextLine(&b, "conflicted_files", strings.Join(files, ", "))
	}
	for _, file := range files {
		hunks, err := conflictHunksForFile(in.IntegrationWorktree, file)
		if err != nil {
			return "", err
		}
		b.WriteString("\nFile: ")
		b.WriteString(file)
		b.WriteString("\n")
		if len(hunks) == 0 {
			b.WriteString("(no conflict-marker hunk found; inspect git status and file history manually)\n")
			continue
		}
		for idx, hunk := range hunks {
			fmt.Fprintf(&b, "Hunk %d:\n", idx+1)
			b.WriteString(hunk)
			if !strings.HasSuffix(hunk, "\n") {
				b.WriteString("\n")
			}
		}
	}
	return strings.TrimSpace(b.String()), nil
}

func conflictHunksForFile(root string, file string) ([]string, error) {
	path, ok, err := safeConflictFilePath(root, file)
	if err != nil || !ok {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read conflicted file %s: %w", file, err)
	}
	return extractConflictHunks(string(content)), nil
}

func extractConflictHunks(content string) []string {
	lines := strings.SplitAfter(content, "\n")
	hunks := make([]string, 0)
	var current strings.Builder
	inConflict := false
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "<<<<<<<") {
			inConflict = true
			current.Reset()
		}
		if !inConflict {
			continue
		}
		if current.Len()+len(line) <= maxConflictHunkBytes {
			current.WriteString(line)
		}
		if strings.HasPrefix(trimmed, ">>>>>>>") {
			hunks = append(hunks, current.String())
			inConflict = false
			current.Reset()
		}
	}
	return hunks
}

func conflictMarkersPresent(root string, files []string) (bool, error) {
	for _, file := range normalizedConflictFiles(files) {
		path, ok, err := safeConflictFilePath(root, file)
		if err != nil {
			return false, err
		}
		if !ok {
			continue
		}
		content, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("read resolved conflict file %s: %w", file, err)
		}
		if containsConflictMarker(content) {
			return true, nil
		}
	}
	return false, nil
}

func containsConflictMarker(content []byte) bool {
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		trimmed := bytes.TrimLeft(line, " \t")
		if bytes.HasPrefix(trimmed, []byte("<<<<<<<")) ||
			bytes.HasPrefix(trimmed, []byte("=======")) ||
			bytes.HasPrefix(trimmed, []byte(">>>>>>>")) {
			return true
		}
	}
	return false
}

func safeConflictFilePath(root string, file string) (string, bool, error) {
	cleanRoot := filepath.Clean(strings.TrimSpace(root))
	if cleanRoot == "." || cleanRoot == "" {
		return "", false, errors.New("conflict resolver: integration worktree is required")
	}
	trimmedFile := strings.TrimSpace(file)
	if trimmedFile == "" {
		return "", false, nil
	}
	if filepath.IsAbs(trimmedFile) {
		return "", false, fmt.Errorf("conflict file %q must be relative", file)
	}
	fullPath := filepath.Clean(filepath.Join(cleanRoot, trimmedFile))
	rel, err := filepath.Rel(cleanRoot, fullPath)
	if err != nil {
		return "", false, fmt.Errorf("resolve conflict file %q: %w", file, err)
	}
	if rel == "." || rel == "" {
		return "", false, nil
	}
	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", false, fmt.Errorf("conflict file %q escapes integration worktree", file)
	}
	return fullPath, true, nil
}

func normalizedConflictFiles(files []string) []string {
	seen := make(map[string]struct{}, len(files))
	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for file := range seen {
		out = append(out, file)
	}
	sort.Strings(out)
	return out
}

func unmergedFilesFromPorcelain(status string) []string {
	seen := make(map[string]struct{})
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		if !isUnmergedPorcelainCode(code) {
			continue
		}
		path := conflictStatusPath(line[3:])
		if path == "" {
			continue
		}
		seen[path] = struct{}{}
	}
	files := make([]string, 0, len(seen))
	for file := range seen {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func isUnmergedPorcelainCode(code string) bool {
	switch code {
	case "DD", "AU", "UD", "UA", "DU", "AA", "UU":
		return true
	default:
		return false
	}
}

func conflictStatusPath(raw string) string {
	path := strings.TrimSpace(raw)
	if _, after, ok := strings.Cut(path, " -> "); ok {
		path = after
	}
	if unquoted, err := strconv.Unquote(path); err == nil {
		path = unquoted
	}
	return path
}

func boundedConflictAttempts(maxAttempts int) int {
	if maxAttempts < workspace.DefaultRecoveryMaxAttempts {
		return workspace.DefaultRecoveryMaxAttempts
	}
	if maxAttempts > workspace.MaxRecoveryAttempts {
		return workspace.MaxRecoveryAttempts
	}
	return maxAttempts
}

// resolverMaxAttempts returns the bounded conflict-resolution attempt ceiling for
// a plan, mirroring the value conflictResolverInput threads into the resolver.
func resolverMaxAttempts(plan ParallelPlan) int {
	cfg := plan.Config.ApplyDefaults()
	resolver := workspace.DefaultAgentRecoveryConfig().ApplyDefaults()
	if cfg.ConflictResolver != nil {
		resolver = cfg.ConflictResolver.ApplyDefaults()
	}
	return boundedConflictAttempts(intPtrValue(resolver.MaxAttempts))
}

func conflictResolverInput(
	plan ParallelPlan,
	task TaskSpec,
	conflicts ConflictSet,
	message string,
) ConflictInput {
	cfg := plan.Config.ApplyDefaults()
	resolver := workspace.DefaultAgentRecoveryConfig().ApplyDefaults()
	if cfg.ConflictResolver != nil {
		resolver = cfg.ConflictResolver.ApplyDefaults()
	}
	return ConflictInput{
		IntegrationWorktree: strings.TrimSpace(plan.IntegrationPath),
		Conflicts:           conflicts,
		Task:                task,
		CommitMessage:       strings.TrimSpace(message),
		ParentRunID:         strings.TrimSpace(plan.RunID),
		Attempt:             1,
		MaxAttempts:         intPtrValue(resolver.MaxAttempts),
		IDE:                 stringPtrValue(resolver.IDE),
		Model:               stringPtrValue(resolver.Model),
		ReasoningEffort:     stringPtrValue(resolver.ReasoningEffort),
	}
}

func intPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func writeConflictContextLine(b *strings.Builder, key string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	b.WriteString("- ")
	b.WriteString(key)
	b.WriteString(": ")
	b.WriteString(trimmed)
	b.WriteString("\n")
}

func nonEmptyConflictSections(sections []string) []string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		if trimmed := strings.TrimSpace(section); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

type osConflictCommandRunner struct{}

func (osConflictCommandRunner) Git(ctx context.Context, dir string, args ...string) (string, error) {
	return runConflictCommand(ctx, dir, "git", append([]string{"-C", strings.TrimSpace(dir)}, args...)...)
}

func (osConflictCommandRunner) Make(ctx context.Context, dir string, args ...string) (string, error) {
	return runConflictCommand(ctx, dir, "make", args...)
}

func runConflictCommand(ctx context.Context, dir string, name string, args ...string) (string, error) {
	cmd := osexec.CommandContext(ctx, name, args...)
	cmd.Dir = strings.TrimSpace(dir)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		output := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
		if output == "" {
			return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
		}
		return output, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, output)
	}
	return stdout.String(), nil
}
