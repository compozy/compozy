// Package evals provides the opt-in model-backed behavioral harness for the
// cy-capture-decisions skill. It is not part of make verify because it consumes
// a real model; make verify still compiles and unit-tests the harness itself.
package evals

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	defaultRepetitions = 3
	defaultTimeout     = 20 * time.Minute
)

// Config controls one complete behavioral eval run.
type Config struct {
	RepoRoot        string
	CompozyBinary   string
	ExtensionDir    string
	ResultsDir      string
	IDE             string
	Model           string
	ReasoningEffort string
	Repetitions     int
	CaseIDs         []string
	Timeout         time.Duration
	OnResult        func(Result)
}

// Result records one case trial and its artifact directory.
type Result struct {
	CaseID      string        `json:"case_id"`
	Trial       int           `json:"trial"`
	Passed      bool          `json:"passed"`
	Duration    time.Duration `json:"duration"`
	ArtifactDir string        `json:"artifact_dir"`
	Error       string        `json:"error,omitempty"`
}

// Summary is the reproducible structural result manifest emitted by Run.
type Summary struct {
	SourceSHA       string    `json:"source_sha"`
	IDE             string    `json:"ide"`
	Model           string    `json:"model"`
	ReasoningEffort string    `json:"reasoning_effort"`
	Repetitions     int       `json:"repetitions"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
	Results         []Result  `json:"results"`
}

type evalCase struct {
	ID   string
	Name string
	Run  func(context.Context, *trial) error
}

type harness struct {
	config     Config
	runtimeDir string
	homeDir    string
	env        []string
	cases      []evalCase
}

type trial struct {
	harness      *harness
	caseID       string
	number       int
	artifactDir  string
	workspaceSeq int
	modelRunSeq  int
	workspaces   []*workspace
}

type workspace struct {
	Name string
	Root string
}

type workspaceOptions struct {
	Fixture      string
	SeedLog      bool
	ApplyPatches []string
	NoMainBranch bool
}

// DefaultConfig returns the standard Codex-backed configuration. Callers must
// still provide Model explicitly so a paid model run is never accidental.
func DefaultConfig(repoRoot string) Config {
	return Config{
		RepoRoot:        repoRoot,
		CompozyBinary:   filepath.Join(repoRoot, "bin", "compozy"),
		ExtensionDir:    filepath.Join(repoRoot, "extensions", "cy-capture-decisions"),
		ResultsDir:      filepath.Join(repoRoot, "extensions", "cy-capture-decisions", "evals", "results"),
		IDE:             "codex",
		ReasoningEffort: "medium",
		Repetitions:     defaultRepetitions,
		Timeout:         defaultTimeout,
	}
}

// Run executes selected cases serially, three times by default, and writes a
// machine-readable plus human-readable summary even when one or more cases fail.
func Run(ctx context.Context, config Config) (summary Summary, runErr error) {
	if err := validateConfig(config); err != nil {
		return Summary{}, err
	}
	resolved, err := resolveConfig(config)
	if err != nil {
		return Summary{}, err
	}
	runtimeDir, err := os.MkdirTemp(evalTempBase(), "cye-")
	if err != nil {
		return Summary{}, fmt.Errorf("create eval runtime directory: %w", err)
	}
	defer func() {
		runErr = errors.Join(runErr, os.RemoveAll(runtimeDir))
	}()

	h := &harness{
		config:     resolved,
		runtimeDir: runtimeDir,
		homeDir:    filepath.Join(runtimeDir, "compozy-home"),
		cases:      allCases(),
	}
	h.env = append(os.Environ(),
		"COMPOZY_HOME="+h.homeDir,
		"COMPOZY_DAEMON_HTTP_PORT=0",
		"NO_COLOR=1",
		"CI=1",
	)
	if err := h.prepareRuntime(ctx); err != nil {
		return Summary{}, err
	}
	defer func() {
		runErr = errors.Join(runErr, h.stopDaemon())
	}()

	selected, err := selectCases(h.cases, resolved.CaseIDs)
	if err != nil {
		return Summary{}, err
	}
	sha, err := h.gitOutput(ctx, resolved.RepoRoot, "rev-parse", "HEAD")
	if err != nil {
		return Summary{}, fmt.Errorf("resolve source SHA: %w", err)
	}
	startedAt := time.Now().UTC()
	summary = Summary{
		SourceSHA:       strings.TrimSpace(sha),
		IDE:             resolved.IDE,
		Model:           resolved.Model,
		ReasoningEffort: resolved.ReasoningEffort,
		Repetitions:     resolved.Repetitions,
		StartedAt:       startedAt,
	}

	for _, eval := range selected {
		for repetition := 1; repetition <= resolved.Repetitions; repetition++ {
			result := h.runTrial(ctx, eval, repetition)
			summary.Results = append(summary.Results, result)
			if resolved.OnResult != nil {
				resolved.OnResult(result)
			}
		}
	}
	summary.FinishedAt = time.Now().UTC()
	if err := writeSummary(resolved.ResultsDir, summary); err != nil {
		return summary, err
	}

	failures := 0
	for _, result := range summary.Results {
		if !result.Passed {
			failures++
		}
	}
	if failures > 0 {
		return summary, fmt.Errorf("model-backed eval: %d of %d trials failed", failures, len(summary.Results))
	}
	return summary, nil
}

func evalTempBase() string {
	// Unix-domain sockets have a small path limit (104 bytes on macOS). The
	// platform temp directory can live below a long /var/folders path, so keep
	// the isolated COMPOZY_HOME short enough for its daemon socket.
	if runtime.GOOS != "windows" {
		return "/tmp"
	}
	return ""
}

func validateConfig(config Config) error {
	if strings.TrimSpace(config.Model) == "" {
		return errors.New("model is required for the opt-in paid eval")
	}
	if config.Repetitions < 1 {
		return errors.New("repetitions must be at least one")
	}
	if config.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}

func resolveConfig(config Config) (Config, error) {
	fields := []*string{&config.RepoRoot, &config.CompozyBinary, &config.ExtensionDir, &config.ResultsDir}
	for _, field := range fields {
		absolute, err := filepath.Abs(*field)
		if err != nil {
			return Config{}, fmt.Errorf("resolve path %q: %w", *field, err)
		}
		*field = absolute
	}
	for _, path := range []string{config.RepoRoot, config.CompozyBinary, config.ExtensionDir} {
		if _, err := os.Stat(path); err != nil {
			return Config{}, fmt.Errorf("inspect required path %q: %w", path, err)
		}
	}
	if err := os.MkdirAll(config.ResultsDir, 0o755); err != nil {
		return Config{}, fmt.Errorf("create results directory: %w", err)
	}
	return config, nil
}

func (h *harness) prepareRuntime(ctx context.Context) error {
	if err := os.MkdirAll(h.homeDir, 0o755); err != nil {
		return fmt.Errorf("create isolated Compozy home: %w", err)
	}
	commands := [][]string{
		{"ext", "install", "--yes", h.config.ExtensionDir},
		{"ext", "enable", "cy-capture-decisions"},
		{"daemon", "start", "--format", "json"},
	}
	for _, args := range commands {
		// Running lifecycle commands outside the repository prevents a bundled
		// workspace extension with the same name from shadowing the isolated
		// user extension that this harness just installed.
		if _, err := h.commandOutput(ctx, h.runtimeDir, h.config.CompozyBinary, args...); err != nil {
			return fmt.Errorf("prepare isolated runtime with %v: %w", args, err)
		}
	}
	return nil
}

func (h *harness) stopDaemon() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := h.commandOutput(
		ctx,
		h.config.RepoRoot,
		h.config.CompozyBinary,
		"daemon",
		"stop",
		"--force",
	); err != nil {
		return fmt.Errorf("stop isolated daemon: %w", err)
	}
	return nil
}

func (h *harness) runTrial(ctx context.Context, eval evalCase, repetition int) Result {
	relative := filepath.Join(eval.ID, fmt.Sprintf("run-%d", repetition))
	artifactDir := filepath.Join(h.config.ResultsDir, relative)
	result := Result{CaseID: eval.ID, Trial: repetition, ArtifactDir: relative}
	if err := resetArtifactDir(h.config.ResultsDir, artifactDir); err != nil {
		result.Error = fmt.Sprintf("reset artifact directory: %v", err)
		return result
	}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		result.Error = fmt.Sprintf("create artifact directory: %v", err)
		return result
	}
	current := &trial{
		harness:     h,
		caseID:      eval.ID,
		number:      repetition,
		artifactDir: artifactDir,
	}
	started := time.Now()
	err := eval.Run(ctx, current)
	result.Duration = time.Since(started)
	if archiveErr := current.archiveWorkspaces(ctx); archiveErr != nil {
		err = errors.Join(err, archiveErr)
	}
	if err != nil {
		failurePath := filepath.Join(artifactDir, "failure.txt")
		if writeErr := os.WriteFile(failurePath, []byte(err.Error()+"\n"), 0o600); writeErr != nil {
			err = errors.Join(err, fmt.Errorf("write failure artifact: %w", writeErr))
		}
		result.Error = err.Error()
		return result
	}
	result.Passed = true
	return result
}

func (t *trial) newWorkspace(ctx context.Context, name string, opts workspaceOptions) (*workspace, error) {
	t.workspaceSeq++
	root := filepath.Join(t.harness.runtimeDir, fmt.Sprintf("%s-%d-%d-%s", t.caseID, t.number, t.workspaceSeq, name))
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	w := &workspace{Name: name, Root: root}
	t.workspaces = append(t.workspaces, w)
	if err := t.initializeRepository(ctx, w); err != nil {
		return nil, err
	}
	if err := t.stageFixture(ctx, w, opts); err != nil {
		return nil, err
	}
	if _, err := t.harness.gitOutput(ctx, root, "add", "."); err != nil {
		return nil, err
	}
	if _, err := t.harness.gitOutput(ctx, root, "commit", "-q", "--allow-empty", "-m", "fixture"); err != nil {
		return nil, err
	}
	if opts.NoMainBranch {
		if _, err := t.harness.gitOutput(ctx, root, "branch", "-D", "main"); err != nil {
			return nil, fmt.Errorf("remove main branch for degraded-mode fixture: %w", err)
		}
	}
	if err := t.installSkill(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (t *trial) initializeRepository(ctx context.Context, w *workspace) error {
	root := w.Root
	if _, err := t.harness.gitOutput(ctx, root, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	for _, pair := range [][2]string{{"user.name", "Compozy Eval"}, {"user.email", "eval@example.invalid"}} {
		if _, err := t.harness.gitOutput(ctx, root, "config", pair[0], pair[1]); err != nil {
			return err
		}
	}
	if _, err := t.harness.gitOutput(ctx, root, "commit", "-q", "--allow-empty", "-m", "base"); err != nil {
		return err
	}
	if _, err := t.harness.gitOutput(ctx, root, "switch", "-q", "-c", "eval-work"); err != nil {
		return err
	}
	return nil
}

func (t *trial) stageFixture(ctx context.Context, w *workspace, opts workspaceOptions) error {
	root := w.Root
	if opts.Fixture != "" {
		fixtureRoot := filepath.Join(t.harness.config.ExtensionDir, "evals", "fixtures", opts.Fixture)
		workflowSource := filepath.Join(fixtureRoot, "workflow")
		workflowTarget := filepath.Join(root, ".compozy", "tasks", opts.Fixture)
		if err := copyTree(workflowSource, workflowTarget); err != nil {
			return fmt.Errorf("copy %s workflow: %w", opts.Fixture, err)
		}
		if opts.SeedLog {
			if err := copyTree(filepath.Join(fixtureRoot, "seed-log"), filepath.Join(root, ".compozy")); err != nil {
				return fmt.Errorf("copy seed log: %w", err)
			}
		}
		patches := opts.ApplyPatches
		if patches == nil {
			if _, err := os.Stat(filepath.Join(fixtureRoot, "diff.patch")); err == nil {
				patches = []string{"diff.patch"}
			} else if !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("inspect fixture patch: %w", err)
			}
		}
		for _, patch := range patches {
			if _, err := t.harness.gitOutput(ctx, root, "apply", filepath.Join(fixtureRoot, patch)); err != nil {
				return fmt.Errorf("apply %s/%s: %w", opts.Fixture, patch, err)
			}
		}
	}
	return nil
}

func (t *trial) installSkill(ctx context.Context, w *workspace) error {
	args := []string{"setup", "--agent", "codex", "--skill", "cy-capture-decisions", "--copy", "--yes"}
	if _, err := t.harness.commandOutput(ctx, w.Root, t.harness.config.CompozyBinary, args...); err != nil {
		return fmt.Errorf("install shipped skill in %s: %w", w.Name, err)
	}
	source := filepath.Join(t.harness.config.ExtensionDir, "skills", "cy-capture-decisions")
	installed := filepath.Join(w.Root, ".agents", "skills", "cy-capture-decisions")
	sourceDigest, err := treeDigest(source)
	if err != nil {
		return fmt.Errorf("digest shipped skill: %w", err)
	}
	installedDigest, err := treeDigest(installed)
	if err != nil {
		return fmt.Errorf("digest installed skill: %w", err)
	}
	if sourceDigest != installedDigest {
		return fmt.Errorf("installed skill tree differs from shipped source: %s", installed)
	}
	return nil
}

func (t *trial) runModel(ctx context.Context, w *workspace, prompt string) (string, error) {
	t.modelRunSeq++
	base := fmt.Sprintf("model-run-%02d", t.modelRunSeq)
	stdoutPath := filepath.Join(t.artifactDir, base+".raw.jsonl")
	stderrPath := filepath.Join(t.artifactDir, base+".stderr.log")
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return "", fmt.Errorf("create model stdout artifact: %w", err)
	}
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		if closeErr := stdoutFile.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close model stdout artifact: %w", closeErr))
		}
		return "", fmt.Errorf("create model stderr artifact: %w", err)
	}

	modelCtx, cancel := context.WithTimeout(ctx, t.harness.config.Timeout)
	defer cancel()
	args := []string{
		"exec",
		"--ide", t.harness.config.IDE,
		"--model", t.harness.config.Model,
		"--reasoning-effort", t.harness.config.ReasoningEffort,
		"--timeout", t.harness.config.Timeout.String(),
		"--max-retries", "0",
		"--format", "raw-json",
		prompt,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	commandArgs := append([]string{t.harness.config.CompozyBinary}, args...)
	cmd := exec.CommandContext(modelCtx, "env", commandArgs...)
	cmd.Dir = w.Root
	cmd.Env = t.harness.env
	cmd.Stdout = io.MultiWriter(stdoutFile, &stdout)
	cmd.Stderr = io.MultiWriter(stderrFile, &stderr)
	runErr := cmd.Run()
	closeErr := errors.Join(stdoutFile.Close(), stderrFile.Close())
	if runErr != nil {
		return stdout.String(), errors.Join(
			fmt.Errorf("run model: %w; stderr: %s", runErr, strings.TrimSpace(stderr.String())),
			closeErr,
		)
	}
	if closeErr != nil {
		return stdout.String(), fmt.Errorf("close model artifacts: %w", closeErr)
	}
	return stdout.String(), nil
}

func (t *trial) capture(ctx context.Context, w *workspace, slug string) (string, error) {
	prompt := fmt.Sprintf("Use the cy-capture-decisions skill to capture the finished %s workflow.", slug)
	return t.runModel(ctx, w, prompt)
}

func (t *trial) archiveWorkspaces(ctx context.Context) error {
	var archiveErr error
	for _, w := range t.workspaces {
		target := filepath.Join(t.artifactDir, "workspaces", w.Name)
		for _, relative := range []string{
			filepath.Join(".compozy", "DECISIONS.md"),
			filepath.Join(".compozy", "decisions"),
			"AGENTS.md",
		} {
			source := filepath.Join(w.Root, relative)
			info, err := os.Stat(source)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				archiveErr = errors.Join(archiveErr, fmt.Errorf("inspect artifact %s: %w", source, err))
				continue
			}
			if info.IsDir() {
				if err := copyTree(source, filepath.Join(target, relative)); err != nil {
					archiveErr = errors.Join(archiveErr, err)
				}
				continue
			}
			if err := copyFile(source, filepath.Join(target, relative), info.Mode().Perm()); err != nil {
				archiveErr = errors.Join(archiveErr, err)
			}
		}
		diff, err := t.harness.gitOutput(ctx, w.Root, "diff", "--", ".compozy")
		if err != nil {
			archiveErr = errors.Join(archiveErr, fmt.Errorf("capture workspace diff: %w", err))
			continue
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			archiveErr = errors.Join(archiveErr, err)
			continue
		}
		if err := os.WriteFile(filepath.Join(target, "decision-log.diff"), []byte(diff), 0o600); err != nil {
			archiveErr = errors.Join(archiveErr, err)
		}
	}
	return archiveErr
}

func (h *harness) commandOutput(ctx context.Context, dir, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = dir
	cmd.Env = h.env
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s %v: %w\n%s", command, args, err, output)
	}
	return string(output), nil
}

func (h *harness) gitOutput(ctx context.Context, dir string, args ...string) (string, error) {
	return h.commandOutput(ctx, dir, "git", args...)
}

func selectCases(cases []evalCase, ids []string) ([]evalCase, error) {
	if len(ids) == 0 {
		return cases, nil
	}
	wanted := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		wanted[strings.ToUpper(strings.TrimSpace(id))] = struct{}{}
	}
	selected := make([]evalCase, 0, len(wanted))
	for _, eval := range cases {
		if _, ok := wanted[eval.ID]; ok {
			selected = append(selected, eval)
			delete(wanted, eval.ID)
		}
	}
	if len(wanted) > 0 {
		unknown := make([]string, 0, len(wanted))
		for id := range wanted {
			unknown = append(unknown, id)
		}
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown eval cases: %s", strings.Join(unknown, ", "))
	}
	return selected, nil
}

func writeSummary(resultsDir string, summary Summary) error {
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("encode eval summary: %w", err)
	}
	if err := os.WriteFile(filepath.Join(resultsDir, "summary.json"), append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write summary.json: %w", err)
	}
	var markdown strings.Builder
	markdown.WriteString("# cy-capture-decisions model-backed eval\n\n")
	format := "- Source SHA: `%s`\n- Runtime: `%s`\n- Model: `%s`\n" +
		"- Reasoning: `%s`\n- Repetitions: `%d`\n\n"
	fmt.Fprintf(
		&markdown,
		format,
		summary.SourceSHA,
		summary.IDE,
		summary.Model,
		summary.ReasoningEffort,
		summary.Repetitions,
	)
	markdown.WriteString("| Case | Trial | Result | Duration |\n| --- | ---: | --- | ---: |\n")
	for _, result := range summary.Results {
		status := "PASS"
		if !result.Passed {
			status = "FAIL: " + strings.ReplaceAll(result.Error, "\n", " ")
		}
		duration := result.Duration.Round(time.Millisecond)
		fmt.Fprintf(&markdown, "| %s | %d | %s | %s |\n", result.CaseID, result.Trial, status, duration)
	}
	if err := os.WriteFile(filepath.Join(resultsDir, "summary.md"), []byte(markdown.String()), 0o600); err != nil {
		return fmt.Errorf("write summary.md: %w", err)
	}
	return nil
}

func copyTree(source, target string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}
		return copyFile(path, destination, info.Mode().Perm())
	})
}

func copyFile(source, target string, mode fs.FileMode) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(target))
	if err != nil {
		return err
	}
	file, err := root.OpenFile(filepath.Base(target), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return errors.Join(err, root.Close())
	}
	if _, err := file.Write(content); err != nil {
		return errors.Join(err, file.Close(), root.Close())
	}
	return errors.Join(file.Close(), root.Close())
}

func resetArtifactDir(resultsDir, artifactDir string) error {
	relative, err := filepath.Rel(resultsDir, artifactDir)
	if err != nil {
		return err
	}
	if relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("artifact path %q escapes results directory", artifactDir)
	}
	return os.RemoveAll(artifactDir)
}

func treeDigest(root string) (string, error) {
	hash := sha256.New()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, path := range paths {
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if _, err := hash.Write([]byte(relative)); err != nil {
			return "", fmt.Errorf("hash relative path: %w", err)
		}
		if _, err := hash.Write(content); err != nil {
			return "", fmt.Errorf("hash file content: %w", err)
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
