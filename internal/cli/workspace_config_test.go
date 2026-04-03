package cli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	core "github.com/compozy/compozy/internal/core"
)

func TestApplyWorkspaceDefaultsLoadsNearestWorkspaceConfig(t *testing.T) {
	root := t.TempDir()
	startDir := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(startDir, 0o755); err != nil {
		t.Fatalf("mkdir start dir: %v", err)
	}
	writeCLIWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
timeout = "5m"
add_dirs = ["../shared", "../docs"]

[start]
include_completed = true
`)

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("include-completed", false, "include completed")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(startDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if mustEvalSymlinksCLITest(t, state.workspaceRoot) != mustEvalSymlinksCLITest(t, root) {
		t.Fatalf("unexpected workspace root\nwant: %q\ngot:  %q", root, state.workspaceRoot)
	}
	if state.ide != "claude" {
		t.Fatalf("unexpected ide default: %q", state.ide)
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
grouped = true
`)

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
	cmd := newTestCommand(state)
	cmd.Flags().Bool("grouped", false, "grouped")

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalWD); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(startDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := cmd.Flags().Set("ide", "gemini"); err != nil {
		t.Fatalf("set ide: %v", err)
	}
	if err := cmd.Flags().Set("grouped", "false"); err != nil {
		t.Fatalf("set grouped: %v", err)
	}

	if err := state.applyWorkspaceDefaults(context.Background(), cmd); err != nil {
		t.Fatalf("apply workspace defaults: %v", err)
	}

	if state.ide != "gemini" {
		t.Fatalf("expected explicit ide flag to win, got %q", state.ide)
	}
	if state.grouped {
		t.Fatalf("expected explicit grouped flag to win")
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
		tailLines:        0,
		reasoningEffort:  "high",
		timeout:          "5m",
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
	if inputs.tailLines != "0" {
		t.Fatalf("unexpected tailLines input: %q", inputs.tailLines)
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
