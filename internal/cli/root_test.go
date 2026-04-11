package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

func TestRootCommandShowsHelpAndWorkflowSubcommands(t *testing.T) {
	t.Parallel()

	cmd := NewRootCommand()
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected root command to omit mode flag")
	}

	output, err := executeRootCommand()
	if err != nil {
		t.Fatalf("execute root command: %v", err)
	}

	required := []string{
		"compozy setup",
		"compozy agents",
		"compozy upgrade",
		"compozy migrate",
		"compozy validate-tasks",
		"compozy sync",
		"compozy archive",
		"compozy fetch-reviews",
		"compozy fix-reviews",
		"compozy start",
		"setup",
		"agents",
		"upgrade",
		"migrate",
		"validate-tasks",
		"sync",
		"archive",
		"fetch-reviews",
		"fix-reviews",
		"compozy exec",
		"exec",
		"start",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected root help to include %q\noutput:\n%s", snippet, output)
		}
	}

	if strings.Contains(output, "mcp-serve") {
		t.Fatalf("expected root help to omit hidden mcp-serve command\noutput:\n%s", output)
	}
}

func TestUpgradeHelpShowsNoUnexpectedFlags(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "upgrade")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected upgrade to omit mode flag")
	}

	output, err := executeRootCommand("upgrade", "--help")
	if err != nil {
		t.Fatalf("execute upgrade help: %v", err)
	}

	required := []string{
		"Upgrade compozy using the appropriate installation flow for this machine.",
		"Package-manager installs print the correct command",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected upgrade help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--provider", "--pr", "--tasks-dir", "--batch-size", "--include-completed"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected upgrade help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestValidateTasksHelpShowsValidationFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "validate-tasks")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected validate-tasks to omit mode flag")
	}

	output, err := executeRootCommand("validate-tasks", "--help")
	if err != nil {
		t.Fatalf("execute validate-tasks help: %v", err)
	}

	required := []string{"--name", "--tasks-dir", "--format"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected validate-tasks help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--pr", "--provider", "--reviews-dir", "--batch-size", "--concurrent", "--include-completed"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected validate-tasks help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestMigrateHelpShowsMigrationFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "migrate")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected migrate to omit mode flag")
	}

	output, err := executeRootCommand("migrate", "--help")
	if err != nil {
		t.Fatalf("execute migrate help: %v", err)
	}

	required := []string{"--root-dir", "--name", "--tasks-dir", "--reviews-dir", "--dry-run"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected migrate help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--provider", "--pr", "--batch-size", "--include-resolved", "--include-completed"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected migrate help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestSyncHelpShowsSyncFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "sync")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected sync to omit mode flag")
	}

	output, err := executeRootCommand("sync", "--help")
	if err != nil {
		t.Fatalf("execute sync help: %v", err)
	}

	required := []string{"--root-dir", "--name", "--tasks-dir"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected sync help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--reviews-dir", "--provider", "--pr", "--batch-size", "--include-completed"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected sync help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestArchiveHelpShowsArchiveFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "archive")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected archive to omit mode flag")
	}

	output, err := executeRootCommand("archive", "--help")
	if err != nil {
		t.Fatalf("execute archive help: %v", err)
	}

	required := []string{"--root-dir", "--name", "--tasks-dir"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected archive help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--reviews-dir", "--provider", "--pr", "--batch-size", "--include-completed"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected archive help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestFetchReviewsHelpShowsFetchFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "fetch-reviews")
	output, err := executeRootCommand("fetch-reviews", "--help")
	if err != nil {
		t.Fatalf("execute fetch-reviews help: %v", err)
	}

	required := []string{"--provider", "--pr", "--name", "--round", "--nitpicks"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected fetch-reviews help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--reviews-dir", "--tasks-dir", "--batch-size", "--grouped", "--include-resolved", "--form "}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected fetch-reviews help to omit %q\noutput:\n%s", snippet, output)
		}
	}

	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected fetch-reviews to omit mode flag")
	}
}

func TestFixReviewsHelpShowsReviewFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "fix-reviews")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected fix-reviews to omit mode flag")
	}

	output, err := executeRootCommand("fix-reviews", "--help")
	if err != nil {
		t.Fatalf("execute fix-reviews help: %v", err)
	}

	required := []string{
		"--format",
		"--name",
		"--round",
		"--reviews-dir",
		"--batch-size",
		"--concurrent",
		"--include-resolved",
		"--tui",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected fix-reviews help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--provider", "--pr", "--tasks-dir", "--include-completed", "--form "}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected fix-reviews help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestStartHelpShowsTaskFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "start")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected start to omit mode flag")
	}

	output, err := executeRootCommand("start", "--help")
	if err != nil {
		t.Fatalf("execute start help: %v", err)
	}

	required := []string{
		"--format",
		"--name",
		"--tasks-dir",
		"--include-completed",
		"--skip-validation",
		"Skip task metadata preflight; use only when tasks were validated separately",
		"--force",
		"Continue after task metadata validation fails in non-interactive mode",
		"--tui",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected start help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{
		"--pr",
		"--provider",
		"--reviews-dir",
		"--batch-size",
		"--concurrent",
		"--grouped",
		"--include-resolved",
		"--form ",
	}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected start help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestExecHelpShowsExecFlagsOnly(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "exec [prompt]")
	if cmd.Flags().Lookup("mode") != nil {
		t.Fatalf("expected exec to omit mode flag")
	}

	output, err := executeRootCommand("exec", "--help")
	if err != nil {
		t.Fatalf("execute exec help: %v", err)
	}

	required := []string{
		"--agent",
		"--extensions",
		"--prompt-file",
		"--format",
		"--dry-run",
		"--ide",
		"--persist",
		"--run-id",
		"--tui",
		"--verbose",
		".compozy/runs/<run-id>/",
		"JSONL events",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected exec help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{
		"--name",
		"--tasks-dir",
		"--reviews-dir",
		"--batch-size",
		"--concurrent",
		"--include-resolved",
	}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected exec help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestExecHelpMatchesGolden(t *testing.T) {
	t.Parallel()

	output, err := executeRootCommand("exec", "--help")
	if err != nil {
		t.Fatalf("execute exec help: %v", err)
	}

	goldenPath := mustCLITestDataPath(t, "exec_help.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v", goldenPath, err)
	}

	if output != string(want) {
		t.Fatalf("exec help output mismatch\nwant:\n%s\n\ngot:\n%s", string(want), output)
	}
}

func TestHiddenMCPServeCommandIsRegisteredButHidden(t *testing.T) {
	t.Parallel()

	cmd := findCommand(t, NewRootCommand(), "mcp-serve")
	if !cmd.Hidden {
		t.Fatal("expected mcp-serve command to be hidden")
	}

	output, err := executeRootCommand("mcp-serve", "--help")
	if err != nil {
		t.Fatalf("execute hidden mcp-serve help: %v", err)
	}
	if !strings.Contains(output, "--server") {
		t.Fatalf("expected hidden mcp-serve help to remain invokable\noutput:\n%s", output)
	}
}

func TestREADMEExecDocumentationMatchesCurrentContract(t *testing.T) {
	t.Parallel()

	readmePath := mustCLIRepoRootPath(t, "README.md")
	body, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read %s: %v", readmePath, err)
	}

	content := string(body)
	required := []string{
		"## ⚡ Ad Hoc Exec",
		"compozy exec \"Summarize the current repository changes\"",
		"compozy exec --prompt-file prompt.md",
		"cat prompt.md | compozy exec --format json",
		"compozy exec --persist \"Review the latest changes\"",
		".compozy/runs/<run-id>/run.json",
		".compozy/runs/<run-id>/events.jsonl",
		".compozy/runs/<run-id>/turns/0001/prompt.md",
		".compozy/runs/<run-id>/turns/0001/result.json",
		"flags > [exec] > [defaults] > built-in defaults",
		"[exec]",
		"`copilot`",
		"`cursor-agent`",
	}
	for _, snippet := range required {
		if !strings.Contains(content, snippet) {
			t.Fatalf("expected README to include %q", snippet)
		}
	}

	forbidden := []string{
		".tmp/codex-prompts",
		"Agent: `claude`, `codex`, `cursor`, `droid`, `opencode`, `pi`",
		"| `--tail-lines`               | `30`",
	}
	for _, snippet := range forbidden {
		if strings.Contains(content, snippet) {
			t.Fatalf("expected README to omit stale snippet %q", snippet)
		}
	}
}

func TestActiveDocsAndHelpFixturesOmitLegacyArtifactRoot(t *testing.T) {
	t.Parallel()

	paths := []string{
		mustCLIRepoRootPath(t, "README.md"),
		mustCLITestDataPath(t, "exec_help.golden"),
		mustCLITestDataPath(t, "start_help.golden"),
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(body), ".tmp/codex-prompts") {
			t.Fatalf("expected %s to omit legacy artifact root", path)
		}
	}
}

func TestStartHelpMatchesGolden(t *testing.T) {
	t.Parallel()

	output, err := executeRootCommand("start", "--help")
	if err != nil {
		t.Fatalf("execute start help: %v", err)
	}

	goldenPath := mustCLITestDataPath(t, "start_help.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file %s: %v", goldenPath, err)
	}

	if output != string(want) {
		t.Fatalf("start help output mismatch\nwant:\n%s\n\ngot:\n%s", string(want), output)
	}
}

func TestBuildConfigNormalizesReviewAddDirs(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	state.autoCommit = true
	state.timeout = "10m"
	state.addDirs = []string{"../shared", "../docs", "../shared"}
	state.accessMode = core.AccessModeDefault

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if !cfg.AutoCommit {
		t.Fatalf("expected AutoCommit=true in config")
	}
	if !reflect.DeepEqual(cfg.AddDirs, []string{"../shared", "../docs"}) {
		t.Fatalf("expected normalized addDirs in config, got %#v", cfg.AddDirs)
	}
	if cfg.Mode != core.ModePRReview {
		t.Fatalf("expected review mode, got %q", cfg.Mode)
	}
	if cfg.AccessMode != core.AccessModeDefault {
		t.Fatalf("expected access mode in config, got %q", cfg.AccessMode)
	}
}

func TestBuildConfigUsesTaskFlagsForStartWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.name = "multi-repo"
	state.tasksDir = ".compozy/tasks/multi-repo"
	state.includeCompleted = true

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Name != "multi-repo" {
		t.Fatalf("expected Name field to carry task name, got %q", cfg.Name)
	}
	if cfg.TasksDir != ".compozy/tasks/multi-repo" {
		t.Fatalf("expected TasksDir to carry tasks dir, got %q", cfg.TasksDir)
	}
	if !cfg.IncludeCompleted {
		t.Fatalf("expected IncludeCompleted=true in config")
	}
	if cfg.Mode != core.ModePRDTasks {
		t.Fatalf("expected start mode, got %q", cfg.Mode)
	}
}

func TestBuildConfigRejectsNonPositiveTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		timeout string
	}{
		{name: "Should reject zero timeout", timeout: "0s"},
		{name: "Should reject negative timeout", timeout: "-1s"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindStart, core.ModePRDTasks)
			state.timeout = tt.timeout

			_, err := state.buildConfig()
			if err == nil {
				t.Fatal("expected buildConfig to reject non-positive timeout")
			}
			if !strings.Contains(err.Error(), "must be > 0") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildConfigUsesFetchFlagsForFetchWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	state.provider = "coderabbit"
	state.pr = "259"
	state.name = "my-feature"
	state.round = 2

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Provider != "coderabbit" || cfg.PR != "259" || cfg.Name != "my-feature" || cfg.Round != 2 {
		t.Fatalf("unexpected fetch config: %#v", cfg)
	}
}

func TestBuildConfigUsesExecFieldsForExecWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindExec, core.ModeExec)
	state.outputFormat = string(core.OutputFormatJSON)
	state.promptFile = "prompt.md"
	state.readPromptStdin = false

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Mode != core.ModeExec {
		t.Fatalf("expected exec mode, got %q", cfg.Mode)
	}
	if cfg.OutputFormat != core.OutputFormatJSON {
		t.Fatalf("expected json output format, got %q", cfg.OutputFormat)
	}
	if cfg.PromptFile != "prompt.md" {
		t.Fatalf("expected prompt file to carry through, got %q", cfg.PromptFile)
	}
	if cfg.ReadPromptStdin {
		t.Fatal("did not expect stdin prompt source")
	}
	if cfg.ResolvedPromptText != "" {
		t.Fatalf("did not expect resolved prompt text by default, got %q", cfg.ResolvedPromptText)
	}
}

func TestCaptureExplicitRuntimeFlagsUsesCobraChangedSemantics(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindExec, core.ModeExec)
	cmd := newTestCommand(state)

	unset := captureExplicitRuntimeFlags(cmd)
	if unset.Model || unset.IDE || unset.ReasoningEffort || unset.AccessMode {
		t.Fatalf("expected no runtime flags to be marked explicit when unset, got %#v", unset)
	}

	if err := cmd.Flags().Set("model", ""); err != nil {
		t.Fatalf("set model flag: %v", err)
	}
	if err := cmd.Flags().Set("access-mode", core.AccessModeFull); err != nil {
		t.Fatalf("set access-mode flag: %v", err)
	}

	explicit := captureExplicitRuntimeFlags(cmd)
	if !explicit.Model {
		t.Fatalf("expected explicit empty model flag to be preserved, got %#v", explicit)
	}
	if !explicit.AccessMode {
		t.Fatalf("expected access-mode flag set to its default value to still count as explicit, got %#v", explicit)
	}
	if explicit.IDE || explicit.ReasoningEffort {
		t.Fatalf("expected only changed flags to be explicit, got %#v", explicit)
	}
}

func TestResolveExecPromptSourceHandlesPromptVariants(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("Prompt from file\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	cases := []struct {
		name                string
		args                []string
		promptFile          string
		stdin               io.Reader
		wantPromptText      string
		wantPromptFile      string
		wantReadPromptStdin bool
		wantResolved        string
	}{
		{
			name:           "Should resolve prompt from positional argument",
			args:           []string{"Summarize the repo"},
			wantPromptText: "Summarize the repo",
			wantResolved:   "Summarize the repo",
		},
		{
			name:           "Should resolve prompt from --prompt-file",
			promptFile:     promptPath,
			wantPromptFile: promptPath,
			wantResolved:   "Prompt from file\n",
		},
		{
			name:                "Should resolve prompt from stdin",
			stdin:               strings.NewReader("Prompt from stdin\n"),
			wantReadPromptStdin: true,
			wantResolved:        "Prompt from stdin\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindExec, core.ModeExec)
			state.promptFile = tc.promptFile
			cmd := &cobra.Command{Use: "exec"}
			if tc.stdin != nil {
				cmd.SetIn(tc.stdin)
			}

			if err := state.resolveExecPromptSource(cmd, tc.args); err != nil {
				t.Fatalf("resolveExecPromptSource: %v", err)
			}
			if state.promptText != tc.wantPromptText {
				t.Fatalf("unexpected promptText: %q", state.promptText)
			}
			if state.promptFile != tc.wantPromptFile {
				t.Fatalf("unexpected promptFile: %q", state.promptFile)
			}
			if state.readPromptStdin != tc.wantReadPromptStdin {
				t.Fatalf("unexpected readPromptStdin: %t", state.readPromptStdin)
			}
			if state.resolvedPromptText != tc.wantResolved {
				t.Fatalf("unexpected resolvedPromptText: %q", state.resolvedPromptText)
			}
		})
	}
}

func TestResolveExecPromptSourceSkipsStdinWhenPromptIsResolvedExplicitly(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	promptPath := filepath.Join(tmpDir, "prompt.md")
	if err := os.WriteFile(promptPath, []byte("Prompt from file\n"), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}

	cases := []struct {
		name       string
		args       []string
		promptFile string
		want       string
	}{
		{
			name: "Should ignore stdin when positional prompt is present",
			args: []string{"Prompt from args"},
			want: "Prompt from args",
		},
		{
			name:       "Should ignore stdin when --prompt-file is present",
			promptFile: promptPath,
			want:       "Prompt from file\n",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			reader := strings.NewReader("   \n")
			state := newCommandState(commandKindExec, core.ModeExec)
			state.promptFile = tc.promptFile
			cmd := &cobra.Command{Use: "exec"}
			cmd.SetIn(reader)

			if err := state.resolveExecPromptSource(cmd, tc.args); err != nil {
				t.Fatalf("resolveExecPromptSource: %v", err)
			}
			if state.resolvedPromptText != tc.want {
				t.Fatalf("unexpected resolved prompt text: %q", state.resolvedPromptText)
			}
		})
	}
}

func TestResolveExecPromptSourceRejectsAmbiguousExplicitSources(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindExec, core.ModeExec)
	state.promptFile = "prompt.md"
	cmd := &cobra.Command{Use: "exec"}

	err := state.resolveExecPromptSource(cmd, []string{"Prompt from args"})
	if err == nil {
		t.Fatal("expected ambiguous prompt sources to fail")
	}
	if !strings.Contains(err.Error(), "accepts only one prompt source") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveExecPromptSourceRejectsExplicitPromptWhenStdinIsAlsoPresent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		args       []string
		promptFile string
	}{
		{
			name: "Should reject positional prompt plus stdin",
			args: []string{"Prompt from args"},
		},
		{
			name:       "Should reject prompt file plus stdin",
			promptFile: mustWritePromptFile(t, "Prompt from file\n"),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindExec, core.ModeExec)
			state.promptFile = tc.promptFile
			cmd := &cobra.Command{Use: "exec"}
			cmd.SetIn(strings.NewReader("Prompt from stdin\n"))

			err := state.resolveExecPromptSource(cmd, tc.args)
			if err == nil {
				t.Fatal("expected ambiguous prompt sources to fail")
			}
			if !strings.Contains(err.Error(), "accepts only one prompt source") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestResolveExecPromptSourceClearsStalePromptFileBetweenRuns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		stdin      io.Reader
		wantPrompt string
	}{
		{
			name:       "Should clear stale prompt file when positional prompt wins",
			args:       []string{"Prompt from args"},
			stdin:      strings.NewReader("   \n"),
			wantPrompt: "Prompt from args",
		},
		{
			name:       "Should clear stale prompt file when stdin prompt wins",
			stdin:      strings.NewReader("Prompt from stdin\n"),
			wantPrompt: "Prompt from stdin\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			state := newCommandState(commandKindExec, core.ModeExec)
			initialPromptPath := mustWritePromptFile(t, "Prompt from file\n")
			initialCmd := &cobra.Command{Use: "exec"}
			initialCmd.Flags().String("prompt-file", "", "prompt file")
			initialCmd.SetIn(strings.NewReader("   \n"))
			if err := initialCmd.Flags().Set("prompt-file", initialPromptPath); err != nil {
				t.Fatalf("set prompt-file flag: %v", err)
			}
			state.promptFile = initialPromptPath

			if err := state.resolveExecPromptSource(initialCmd, nil); err != nil {
				t.Fatalf("resolve initial prompt file source: %v", err)
			}
			if state.promptFile != initialPromptPath {
				t.Fatalf("expected initial prompt file to remain active, got %q", state.promptFile)
			}

			nextCmd := &cobra.Command{Use: "exec"}
			nextCmd.Flags().String("prompt-file", "", "prompt file")
			nextCmd.SetIn(tt.stdin)
			if err := state.resolveExecPromptSource(nextCmd, tt.args); err != nil {
				t.Fatalf("resolve subsequent prompt source: %v", err)
			}
			if state.promptFile != "" {
				t.Fatalf("expected stale prompt file to be cleared, got %q", state.promptFile)
			}
			if state.resolvedPromptText != tt.wantPrompt {
				t.Fatalf("unexpected resolved prompt text: %q", state.resolvedPromptText)
			}
		})
	}
}

func TestFetchReviewsReturnsWriterErrors(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	state.fetchReviewsFn = func(context.Context, core.Config) (*core.FetchResult, error) {
		return &core.FetchResult{
			Provider:   "coderabbit",
			PR:         "70",
			Round:      1,
			ReviewsDir: "/tmp/reviews",
			Total:      19,
		}, nil
	}

	cmd := newTestCommand(state)
	cmd.SetOut(&failOnWriteWriter{err: errors.New("broken stdout")})
	if err := cmd.Flags().Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run flag: %v", err)
	}

	err := state.fetchReviews(cmd, nil)
	if err == nil {
		t.Fatal("expected fetchReviews to return a write error")
	}
	if !strings.Contains(err.Error(), "write fetch summary") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFormInputsApplyForFetchWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFetchReviews, core.ModePRReview)
	cmd := newTestCommand(state)
	cmd.Flags().String("provider", "", "provider")
	cmd.Flags().String("pr", "", "pull request")
	cmd.Flags().String("name", "", "prd name")
	cmd.Flags().Int("round", 0, "round")

	fi := &formInputs{
		provider: "coderabbit",
		pr:       "259",
		name:     "my-feature",
		round:    "3",
	}

	fi.apply(cmd, state)

	if state.provider != "coderabbit" || state.pr != "259" || state.name != "my-feature" || state.round != 3 {
		t.Fatalf("unexpected fetch form state: %#v", state)
	}
}

func TestFormInputsApplyForReviewWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)
	cmd.Flags().String("name", "", "prd name")
	cmd.Flags().String("reviews-dir", "", "review dir")
	cmd.Flags().Int("round", 0, "round")
	cmd.Flags().Int("batch-size", 1, "batch size")
	cmd.Flags().Bool("include-resolved", false, "include resolved")

	fi := &formInputs{
		name:            "my-feature",
		reviewsDir:      ".compozy/tasks/my-feature/reviews-001",
		round:           "2",
		batchSize:       "3",
		addDirs:         " ../shared, ../docs ,, ../shared \n ../workspace ",
		includeResolved: true,
	}

	fi.apply(cmd, state)

	if state.name != "my-feature" {
		t.Fatalf("expected name to be applied, got %q", state.name)
	}
	if state.reviewsDir != ".compozy/tasks/my-feature/reviews-001" {
		t.Fatalf("expected reviews dir to map to reviewsDir, got %q", state.reviewsDir)
	}
	if state.round != 2 {
		t.Fatalf("expected round 2, got %d", state.round)
	}
	if state.batchSize != 3 {
		t.Fatalf("expected batch size 3, got %d", state.batchSize)
	}
	if !state.includeResolved {
		t.Fatalf("expected includeResolved=true")
	}
	wantDirs := []string{"../shared", "../docs", "../workspace"}
	if !reflect.DeepEqual(state.addDirs, wantDirs) {
		t.Fatalf("unexpected addDirs from form\nwant: %#v\ngot:  %#v", wantDirs, state.addDirs)
	}
}

func TestFormInputsApplyClearsPrefilledOptionalValues(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)
	state.round = 2
	state.reviewsDir = ".compozy/tasks/my-feature/reviews-001"
	state.model = "gpt-5.4"
	state.addDirs = []string{"../shared", "../docs,archive"}
	state.tailLines = 25
	state.accessMode = core.AccessModeFull
	state.timeout = "5m"
	cmd.Flags().Int("round", 0, "round")
	cmd.Flags().String("reviews-dir", "", "review dir")

	fi := &formInputs{}
	fi.apply(cmd, state)

	if state.round != 0 {
		t.Fatalf("expected round to clear, got %d", state.round)
	}
	if state.reviewsDir != "" {
		t.Fatalf("expected reviewsDir to clear, got %q", state.reviewsDir)
	}
	if state.model != "" {
		t.Fatalf("expected model to clear, got %q", state.model)
	}
	if state.timeout != "5m" {
		t.Fatalf("expected timeout to remain config-only, got %q", state.timeout)
	}
	if state.addDirs != nil {
		t.Fatalf("expected addDirs to clear, got %#v", state.addDirs)
	}
	if state.tailLines != 25 {
		t.Fatalf("expected tailLines to remain config-only, got %d", state.tailLines)
	}
	if state.accessMode != core.AccessModeFull {
		t.Fatalf("expected accessMode to remain config-only, got %q", state.accessMode)
	}
}

func TestFormInputsApplyParsesQuotedAddDirs(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)

	fi := &formInputs{
		addDirs: "\"../docs,archive\", ../shared",
	}
	fi.apply(cmd, state)

	want := []string{"../docs,archive", "../shared"}
	if !reflect.DeepEqual(state.addDirs, want) {
		t.Fatalf("unexpected addDirs from quoted form input\nwant: %#v\ngot:  %#v", want, state.addDirs)
	}
}

func TestFormInputsApplyForStartWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().String("name", "", "task name")
	cmd.Flags().String("tasks-dir", "", "tasks dir")
	cmd.Flags().Bool("include-completed", false, "include completed")

	fi := &formInputs{
		name:             "multi-repo",
		tasksDir:         ".compozy/tasks/multi-repo",
		includeCompleted: true,
	}

	fi.apply(cmd, state)

	if state.name != "multi-repo" {
		t.Fatalf("expected task name to map to name, got %q", state.name)
	}
	if state.tasksDir != ".compozy/tasks/multi-repo" {
		t.Fatalf("expected tasks dir to map to tasksDir, got %q", state.tasksDir)
	}
	if !state.includeCompleted {
		t.Fatalf("expected includeCompleted=true")
	}
}

func TestMaybeCollectInteractiveParamsUsesFormWhenNoFlags(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)

	called := false
	state.isInteractive = func() bool { return true }
	state.collectForm = func(_ *cobra.Command, got *commandState) error {
		called = true
		if got != state {
			t.Fatalf("collectForm received unexpected state pointer")
		}
		return nil
	}

	if err := state.maybeCollectInteractiveParams(cmd); err != nil {
		t.Fatalf("maybeCollectInteractiveParams: %v", err)
	}
	if !called {
		t.Fatal("expected form collection to run when no flags are provided")
	}
}

func TestMaybeCollectInteractiveParamsReturnsClearErrorWithoutTTY(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)

	called := false
	state.isInteractive = func() bool { return false }
	state.collectForm = func(_ *cobra.Command, _ *commandState) error {
		called = true
		return nil
	}

	err := state.maybeCollectInteractiveParams(cmd)
	if err == nil {
		t.Fatal("expected error without interactive terminal")
	}
	if called {
		t.Fatal("did not expect form collection without interactive terminal")
	}
	if !strings.Contains(err.Error(), "requires an interactive terminal when called without flags") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMaybeCollectInteractiveParamsSkipsFormWhenAnyFlagIsProvided(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)

	called := false
	state.isInteractive = func() bool { return false }
	state.collectForm = func(_ *cobra.Command, _ *commandState) error {
		called = true
		return nil
	}

	if err := cmd.Flags().Set("ide", "claude"); err != nil {
		t.Fatalf("set ide flag: %v", err)
	}

	if err := state.maybeCollectInteractiveParams(cmd); err != nil {
		t.Fatalf("maybeCollectInteractiveParams: %v", err)
	}
	if called {
		t.Fatal("did not expect form collection when flags are provided")
	}
}

func TestFetchReviewsWithPartialFlagsSkipsFormAndReturnsValidationError(t *testing.T) {
	t.Parallel()

	output, err := executeRootCommand("fetch-reviews", "--provider", "coderabbit")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), "interactive terminal when called without flags") {
		t.Fatalf("unexpected interactive-terminal error: %v", err)
	}
	if !strings.Contains(err.Error(), "fetch-reviews requires --name") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "Error: fetch-reviews requires --name") {
		t.Fatalf("unexpected command output: %q", output)
	}
}

func TestStartCommandWrapsWorkspaceDefaultErrors(t *testing.T) {
	root := t.TempDir()
	writeCLIWorkspaceConfig(t, root, `
[defaults]
timeout = "not-a-duration"
`)

	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	chdirCLITest(t, startDir)

	_, err := executeRootCommand("start", "--name", "demo", "--tasks-dir", ".compozy/tasks/demo")
	if err == nil {
		t.Fatal("expected workspace default error")
	}
	if !strings.Contains(err.Error(), "apply workspace defaults for start") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncCommandWrapsWorkspaceRootErrors(t *testing.T) {
	root := t.TempDir()
	writeCLIWorkspaceConfig(t, root, `
[defaults]
timeout = "not-a-duration"
`)

	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	chdirCLITest(t, startDir)

	_, err := executeRootCommand("sync", "--name", "demo", "--tasks-dir", ".compozy/tasks/demo")
	if err == nil {
		t.Fatal("expected workspace root error")
	}
	if !strings.Contains(err.Error(), "load workspace root for sync") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunPreparedBlocksStartWhenBundledSkillsAreMissing(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{
			{Name: "cy-execute-task", Description: "Execute a task"},
			{Name: "cy-final-verify", Description: "Verify completion"},
		}, nil
	}
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeUnknown,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateMissing},
				{Skill: setup.Skill{Name: "cy-final-verify"}, State: setup.VerifyStateMissing},
			},
		}, nil
	}

	called := false
	state.runWorkflow = func(context.Context, core.Config) error {
		called = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"

	err := state.runPrepared(context.Background(), cmd, core.Config{IDE: core.IDECodex, Mode: core.ModePRDTasks})
	if err == nil {
		t.Fatal("expected missing skills to block execution")
	}
	if called {
		t.Fatal("did not expect workflow runner when skills are missing")
	}
	if !strings.Contains(err.Error(), "compozy setup --agent codex") {
		t.Fatalf("expected setup instruction, got %v", err)
	}
	if !strings.Contains(err.Error(), "cy-execute-task, cy-final-verify") {
		t.Fatalf("expected missing skill list, got %v", err)
	}
}

func TestRunPreparedBlocksFixReviewsWhenBundledSkillsAreMissing(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{
			{Name: "cy-fix-reviews", Description: "Fix reviews"},
			{Name: "cy-final-verify", Description: "Verify completion"},
		}, nil
	}
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "claude-code", DisplayName: "Claude Code"},
			Scope: setup.InstallScopeProject,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-fix-reviews"}, State: setup.VerifyStateCurrent},
				{Skill: setup.Skill{Name: "cy-final-verify"}, State: setup.VerifyStateMissing},
			},
		}, nil
	}

	called := false
	state.runWorkflow = func(context.Context, core.Config) error {
		called = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "fix-reviews"

	err := state.runPrepared(context.Background(), cmd, core.Config{IDE: core.IDEClaude, Mode: core.ModePRReview})
	if err == nil {
		t.Fatal("expected missing skills to block execution")
	}
	if called {
		t.Fatal("did not expect workflow runner when skills are missing")
	}
	if !strings.Contains(err.Error(), "compozy setup --agent claude-code") {
		t.Fatalf("expected setup instruction, got %v", err)
	}
	if !strings.Contains(err.Error(), "project-local install is missing: cy-final-verify") {
		t.Fatalf("expected project scope error, got %v", err)
	}
}

func TestRunPreparedRefreshesDriftedSkillsBeforeRunningWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.isInteractive = func() bool { return true }
	state.skipValidation = true
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{
			{Name: "cy-execute-task", Description: "Execute a task"},
			{Name: "cy-final-verify", Description: "Verify completion"},
		}, nil
	}

	verifyCalls := 0
	state.verifyBundledSkills = func(cfg setup.VerifyConfig) (setup.VerifyResult, error) {
		verifyCalls++
		if !reflect.DeepEqual(cfg.SkillNames, []string{"cy-execute-task", "cy-final-verify"}) {
			t.Fatalf("unexpected verify skill names: %#v", cfg.SkillNames)
		}
		if cfg.AgentName != "codex" {
			t.Fatalf("expected codex verify agent, got %q", cfg.AgentName)
		}

		if verifyCalls == 1 {
			return setup.VerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
				Mode:  setup.InstallModeCopy,
				Skills: []setup.VerifiedSkill{
					{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateDrifted},
					{Skill: setup.Skill{Name: "cy-final-verify"}, State: setup.VerifyStateCurrent},
				},
			}, nil
		}
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeCopy,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateCurrent},
				{Skill: setup.Skill{Name: "cy-final-verify"}, State: setup.VerifyStateCurrent},
			},
		}, nil
	}

	confirmed := false
	state.confirmSkillRefresh = func(_ *cobra.Command, prompt skillRefreshPrompt) (bool, error) {
		confirmed = true
		if prompt.AgentName != "codex" {
			t.Fatalf("expected codex prompt, got %q", prompt.AgentName)
		}
		if !reflect.DeepEqual(prompt.DriftedSkills, []string{"cy-execute-task"}) {
			t.Fatalf("unexpected drifted skills in prompt: %#v", prompt.DriftedSkills)
		}
		return true, nil
	}

	installed := false
	state.installBundledSkills = func(cfg setup.InstallConfig) (*setup.Result, error) {
		installed = true
		if cfg.Global {
			t.Fatal("did not expect global refresh for project drift")
		}
		if cfg.Mode != setup.InstallModeCopy {
			t.Fatalf("expected copy mode refresh, got %q", cfg.Mode)
		}
		if !reflect.DeepEqual(cfg.SkillNames, []string{"cy-execute-task", "cy-final-verify"}) {
			t.Fatalf("unexpected install skill names: %#v", cfg.SkillNames)
		}
		if !reflect.DeepEqual(cfg.AgentNames, []string{"codex"}) {
			t.Fatalf("unexpected install agents: %#v", cfg.AgentNames)
		}
		return &setup.Result{}, nil
	}

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := state.runPrepared(context.Background(), cmd, core.Config{IDE: core.IDECodex, Mode: core.ModePRDTasks})
	if err != nil {
		t.Fatalf("runPrepared: %v", err)
	}
	if !confirmed {
		t.Fatal("expected refresh confirmation prompt")
	}
	if !installed {
		t.Fatal("expected inline setup install")
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner after successful refresh")
	}
	if verifyCalls != 2 {
		t.Fatalf("expected verify to run twice, got %d", verifyCalls)
	}
	if !strings.Contains(output.String(), "Updated required Compozy skills for Codex (project scope).") {
		t.Fatalf("expected refresh success output, got %q", output.String())
	}
}

func TestRunPreparedContinuesWhenInteractiveRefreshIsDeclined(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.isInteractive = func() bool { return true }
	state.skipValidation = true
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{
			{Name: "cy-execute-task", Description: "Execute a task"},
		}, nil
	}
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeCopy,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateDrifted},
			},
		}, nil
	}
	state.confirmSkillRefresh = func(*cobra.Command, skillRefreshPrompt) (bool, error) {
		return false, nil
	}

	installed := false
	state.installBundledSkills = func(setup.InstallConfig) (*setup.Result, error) {
		installed = true
		return &setup.Result{}, nil
	}

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := state.runPrepared(context.Background(), cmd, core.Config{IDE: core.IDECodex, Mode: core.ModePRDTasks})
	if err != nil {
		t.Fatalf("runPrepared: %v", err)
	}
	if installed {
		t.Fatal("did not expect install when refresh is declined")
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner after declining refresh")
	}
	if !strings.Contains(output.String(), "continuing with the installed skills") {
		t.Fatalf("expected decline warning, got %q", output.String())
	}
}

func TestRunPreparedWarnsAndContinuesOnNonInteractiveDrift(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.isInteractive = func() bool { return false }
	state.skipValidation = true
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{
			{Name: "cy-execute-task", Description: "Execute a task"},
		}, nil
	}
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeGlobal,
			Mode:  setup.InstallModeSymlink,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateDrifted},
			},
		}, nil
	}
	state.confirmSkillRefresh = func(*cobra.Command, skillRefreshPrompt) (bool, error) {
		t.Fatal("did not expect prompt in non-interactive mode")
		return false, nil
	}
	state.installBundledSkills = func(setup.InstallConfig) (*setup.Result, error) {
		t.Fatal("did not expect install in non-interactive mode")
		return nil, nil
	}

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := state.runPrepared(context.Background(), cmd, core.Config{IDE: core.IDECodex, Mode: core.ModePRDTasks})
	if err != nil {
		t.Fatalf("runPrepared: %v", err)
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner in non-interactive drift mode")
	}
	if !strings.Contains(output.String(), "installed global scope") {
		t.Fatalf("expected non-interactive warning output, got %q", output.String())
	}
}

func TestRunPreparedStartSkipValidationBypassesTaskValidation(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	allowBundledSkillsForStartTest(state)
	state.skipValidation = true
	state.workspaceRoot = t.TempDir()

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := state.runPrepared(context.Background(), cmd, core.Config{IDE: core.IDECodex, Mode: core.ModePRDTasks})
	if err != nil {
		t.Fatalf("runPrepared skip validation: %v", err)
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner when validation is skipped")
	}
	if !strings.Contains(output.String(), "preflight=skipped") {
		t.Fatalf("expected skipped preflight log, got %q", output.String())
	}
}

func TestRunPreparedStartNonInteractiveValidationFailureBlocksWorkflow(t *testing.T) {
	t.Parallel()

	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Task 1: Missing Title",
	))

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	allowBundledSkillsForStartTest(state)
	state.isInteractive = func() bool { return false }
	state.workspaceRoot = workspaceRoot

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := state.runPrepared(context.Background(), cmd, core.Config{
		IDE:      core.IDECodex,
		Mode:     core.ModePRDTasks,
		TasksDir: tasksDir,
	})
	if err == nil {
		t.Fatal("expected task validation failure")
	}
	if runnerCalled {
		t.Fatal("did not expect workflow runner when preflight aborts")
	}

	var exitErr interface{ ExitCode() int }
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1 error, got %v", err)
	}

	got := output.String()
	for _, want := range []string{"Fix prompt:", "title is required", "preflight=aborted"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q\noutput:\n%s", want, got)
		}
	}
}

func TestRunPreparedStartNonInteractiveForceContinuesPastValidationFailure(t *testing.T) {
	t.Parallel()

	workspaceRoot, tasksDir := makeValidateTasksWorkspace(t, "demo")
	writeRawTaskFileForCLI(t, tasksDir, "task_01.md", cliTaskMarkdown(
		[]string{"status: pending", "type: backend", "complexity: low"},
		"# Task 1: Missing Title",
	))

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	allowBundledSkillsForStartTest(state)
	state.isInteractive = func() bool { return false }
	state.workspaceRoot = workspaceRoot
	state.force = true

	runnerCalled := false
	state.runWorkflow = func(context.Context, core.Config) error {
		runnerCalled = true
		return nil
	}

	cmd := newTestCommand(state)
	cmd.Use = "start"
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	err := state.runPrepared(context.Background(), cmd, core.Config{
		IDE:      core.IDECodex,
		Mode:     core.ModePRDTasks,
		TasksDir: tasksDir,
	})
	if err != nil {
		t.Fatalf("runPrepared forced validation: %v", err)
	}
	if !runnerCalled {
		t.Fatal("expected workflow runner after forced preflight")
	}
	if got := output.String(); !strings.Contains(got, "preflight=forced") {
		t.Fatalf("expected forced preflight log, got %q", got)
	}
}

func executeRootCommand(args ...string) (string, error) {
	return executeCommandCombinedOutput(NewRootCommand(), nil, args...)
}

func executeCommandCombinedOutput(
	cmd *cobra.Command,
	in io.Reader,
	args ...string,
) (string, error) {
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	if in != nil {
		cmd.SetIn(in)
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func TestExecuteCommandCombinedOutputPreservesEmissionOrder(t *testing.T) {
	t.Parallel()

	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := cmd.OutOrStdout().Write([]byte("stdout-1\n")); err != nil {
				return err
			}
			if _, err := cmd.ErrOrStderr().Write([]byte("stderr-1\n")); err != nil {
				return err
			}
			if _, err := cmd.OutOrStdout().Write([]byte("stdout-2\n")); err != nil {
				return err
			}
			return nil
		},
	}

	output, err := executeCommandCombinedOutput(cmd, nil)
	if err != nil {
		t.Fatalf("execute combined output command: %v", err)
	}
	if output != "stdout-1\nstderr-1\nstdout-2\n" {
		t.Fatalf("unexpected combined output order: %q", output)
	}
}

func TestCommandStateDefaultsWithFallbacksPreservesExplicitFunctions(t *testing.T) {
	t.Parallel()

	customInteractive := func() bool { return false }
	customCollectForm := func(*cobra.Command, *commandState) error { return nil }

	filled := (commandStateDefaults{
		commandStateCallbacks: commandStateCallbacks{
			isInteractive: customInteractive,
			collectForm:   customCollectForm,
		},
	}).withFallbacks()

	if got := reflect.ValueOf(filled.isInteractive).Pointer(); got != reflect.ValueOf(customInteractive).Pointer() {
		t.Fatal("expected withFallbacks to preserve explicit isInteractive function")
	}
	if got := reflect.ValueOf(filled.collectForm).Pointer(); got != reflect.ValueOf(customCollectForm).Pointer() {
		t.Fatal("expected withFallbacks to preserve explicit collectForm function")
	}
	if filled.listBundledSkills == nil ||
		filled.verifyBundledSkills == nil ||
		filled.installBundledSkills == nil ||
		filled.confirmSkillRefresh == nil {
		t.Fatalf("expected withFallbacks to populate bundled skill defaults: %#v", filled)
	}
}

func newTestCommand(state *commandState) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	addCommonFlags(cmd, state, commonFlagOptions{includeConcurrent: state.kind == commandKindFixReviews})
	if state.kind == commandKindStart || state.kind == commandKindFixReviews {
		addWorkflowOutputFlags(cmd, state)
	}
	return cmd
}

func allowBundledSkillsForStartTest(state *commandState) {
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return nil, nil
	}
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{}, nil
	}
}

func findCommand(t *testing.T, root *cobra.Command, use string) *cobra.Command {
	t.Helper()

	for _, cmd := range root.Commands() {
		if cmd.Use == use {
			return cmd
		}
	}
	t.Fatalf("command %q not found", use)
	return nil
}

func mustCLITestDataPath(t *testing.T, name string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}
	return filepath.Join(filepath.Dir(currentFile), "testdata", name)
}

func mustCLIRepoRootPath(t *testing.T, elems ...string) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file path")
	}
	parts := append([]string{filepath.Dir(currentFile), "..", ".."}, elems...)
	return filepath.Join(parts...)
}

func mustWritePromptFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "prompt.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write prompt file: %v", err)
	}
	return path
}

type failOnWriteWriter struct {
	err error
}

var errFailOnWriteWriterUnset = errors.New("failOnWriteWriter: missing injected error")

func (w *failOnWriteWriter) Write(_ []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return 0, errFailOnWriteWriterUnset
}
