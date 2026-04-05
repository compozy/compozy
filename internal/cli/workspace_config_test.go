package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	core "github.com/compozy/compozy/internal/core"
)

var cliWorkingDirMu sync.Mutex

func TestApplyWorkspaceDefaultsLoadsNearestWorkspaceConfig(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
access_mode = "default"
timeout = "5m"
add_dirs = ["../shared", "../docs"]

[start]
include_completed = true
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("include-completed", false, "include completed")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if mustEvalSymlinksCLITest(t, state.workspaceRoot) != mustEvalSymlinksCLITest(t, root) {
		t.Fatalf("unexpected workspace root\nwant: %q\ngot:  %q", root, state.workspaceRoot)
	}
	if state.ide != "claude" {
		t.Fatalf("unexpected ide default: %q", state.ide)
	}
	if state.accessMode != "default" {
		t.Fatalf("unexpected access mode default: %q", state.accessMode)
	}
	if state.timeout != "5m" {
		t.Fatalf("unexpected timeout default: %q", state.timeout)
	}
	if !state.includeCompleted {
		t.Fatalf("expected includeCompleted=true")
	}
	wantDirs := []string{"../shared", "../docs"}
	if !reflect.DeepEqual(state.addDirs, wantDirs) {
		t.Fatalf("unexpected addDirs\nwant: %#v\ngot:  %#v", wantDirs, state.addDirs)
	}
}

func TestApplyWorkspaceDefaultsDoesNotOverrideChangedFlags(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"

[fix_reviews]
batch_size = 4
`)

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)
	cmd.Flags().Int("batch-size", 1, "batch size")

	chdirCLITest(t, startDir)

	if err := cmd.Flags().Set("ide", "gemini"); err != nil {
		t.Fatalf("set ide: %v", err)
	}
	if err := cmd.Flags().Set("batch-size", "2"); err != nil {
		t.Fatalf("set batch-size: %v", err)
	}
	state.batchSize = 2

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.ide != "gemini" {
		t.Fatalf("expected explicit ide flag to win, got %q", state.ide)
	}
	if state.batchSize != 2 {
		t.Fatalf("expected explicit batch-size flag to win, got %d", state.batchSize)
	}
}

func TestApplyWorkspaceDefaultsUsesExecOverridesOverDefaults(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
model = "sonnet"
output_format = "text"

[exec]
ide = "codex"
model = "gpt-5.4"
output_format = "raw-json"
verbose = true
`)

	state := newCommandState(commandKindExec, core.ModeExec)
	cmd := newTestCommand(state)
	cmd.Flags().String("format", "", "output format")
	cmd.Flags().Bool("verbose", false, "verbose logging")

	chdirCLITest(t, startDir)

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.ide != "codex" {
		t.Fatalf("expected exec.ide to override defaults.ide, got %q", state.ide)
	}
	if state.model != "gpt-5.4" {
		t.Fatalf("expected exec.model to override defaults.model, got %q", state.model)
	}
	if state.outputFormat != "raw-json" {
		t.Fatalf("expected exec.output_format to override defaults.output_format, got %q", state.outputFormat)
	}
	if !state.verbose {
		t.Fatal("expected exec.verbose to enable verbose logging")
	}
}

func TestApplyWorkspaceDefaultsPreservesExplicitExecFormatFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
output_format = "text"

[exec]
output_format = "json"
`)

	state := newCommandState(commandKindExec, core.ModeExec)
	cmd := newTestCommand(state)
	cmd.Flags().String("format", "", "output format")

	chdirCLITest(t, startDir)

	if err := cmd.Flags().Set("format", "text"); err != nil {
		t.Fatalf("set format: %v", err)
	}
	state.outputFormat = "text"

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.outputFormat != "text" {
		t.Fatalf("expected explicit format flag to win, got %q", state.outputFormat)
	}
}

func TestNewFormInputsFromStatePreservesResolvedDefaults(t *testing.T) {
	t.Parallel()

	state := &commandState{
		name:             "demo",
		tasksDir:         "/tmp/demo/.compozy/tasks/demo",
		ide:              "claude",
		model:            "sonnet",
		addDirs:          []string{"../shared", "../docs"},
		reasoningEffort:  "high",
		includeCompleted: true,
		autoCommit:       true,
	}

	inputs := newFormInputsFromState(state)

	if inputs.name != "demo" || inputs.ide != "claude" || inputs.model != "sonnet" {
		t.Fatalf("unexpected string inputs: %#v", inputs)
	}
	if inputs.addDirs != "../shared, ../docs" {
		t.Fatalf("unexpected addDirs input: %q", inputs.addDirs)
	}
	if !inputs.includeCompleted || !inputs.autoCommit {
		t.Fatalf("expected boolean defaults to be preserved: %#v", inputs)
	}
}

func TestNewFormInputsFromStateQuotesAddDirsContainingCommas(t *testing.T) {
	t.Parallel()

	state := &commandState{
		addDirs: []string{"../docs,archive", "../shared"},
	}

	inputs := newFormInputsFromState(state)
	if inputs.addDirs != "\"../docs,archive\", ../shared" {
		t.Fatalf("unexpected addDirs input: %q", inputs.addDirs)
	}
}

func writeCLIWorkspaceConfig(t *testing.T, workspaceRoot, content string) {
	t.Helper()

	configDir := filepath.Join(workspaceRoot, ".compozy")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func mustEvalSymlinksCLITest(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks for %s: %v", path, err)
	}
	return resolved
}

func chdirCLITest(t *testing.T, dir string) {
	t.Helper()

	cliWorkingDirMu.Lock()

	originalWD, err := os.Getwd()
	if err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		cliWorkingDirMu.Unlock()
		t.Fatalf("chdir: %v", err)
	}

	t.Cleanup(func() {
		defer cliWorkingDirMu.Unlock()
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
}
