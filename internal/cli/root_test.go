package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	core "github.com/compozy/looper/internal/looper"
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
		"looper fix-reviews",
		"looper start",
		"fix-reviews",
		"start",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected root help to include %q\noutput:\n%s", snippet, output)
		}
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

	required := []string{"--pr", "--issues-dir", "--batch-size", "--grouped", "--form"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected fix-reviews help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--name", "--tasks-dir", "--include-completed"}
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

	required := []string{"--name", "--tasks-dir", "--include-completed", "--form"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected start help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--pr", "--issues-dir", "--batch-size", "--grouped"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected start help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestBuildConfigNormalizesReviewAddDirs(t *testing.T) {
	t.Parallel()

	state := newCommandState(core.ModePRReview)
	state.autoCommit = true
	state.timeout = "10m"
	state.addDirs = []string{"../shared", "../docs", "../shared"}

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
}

func TestBuildConfigUsesTaskFlagsForStartWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(core.ModePRDTasks)
	state.name = "multi-repo"
	state.tasksDir = "tasks/prd-multi-repo"
	state.includeCompleted = true

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.PR != "multi-repo" {
		t.Fatalf("expected PR field to carry task name, got %q", cfg.PR)
	}
	if cfg.IssuesDir != "tasks/prd-multi-repo" {
		t.Fatalf("expected IssuesDir to carry tasks dir, got %q", cfg.IssuesDir)
	}
	if !cfg.IncludeCompleted {
		t.Fatalf("expected IncludeCompleted=true in config")
	}
	if cfg.Mode != core.ModePRDTasks {
		t.Fatalf("expected start mode, got %q", cfg.Mode)
	}
}

func TestFormInputsApplyForReviewWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(core.ModePRReview)
	cmd := newTestCommand(state)
	cmd.Flags().String("pr", "", "review identifier")
	cmd.Flags().String("issues-dir", "", "review dir")
	cmd.Flags().Int("batch-size", 1, "batch size")
	cmd.Flags().Bool("grouped", false, "grouped")

	fi := &formInputs{
		identifier: "259",
		inputDir:   "ai-docs/reviews-pr-259/issues",
		batchSize:  "3",
		addDirs:    " ../shared, ../docs ,, ../shared \n ../workspace ",
		grouped:    true,
	}

	fi.apply(cmd, state)

	if state.pr != "259" {
		t.Fatalf("expected review identifier to map to pr, got %q", state.pr)
	}
	if state.issuesDir != "ai-docs/reviews-pr-259/issues" {
		t.Fatalf("expected review dir to map to issuesDir, got %q", state.issuesDir)
	}
	if state.batchSize != 3 {
		t.Fatalf("expected batch size 3, got %d", state.batchSize)
	}
	if !state.grouped {
		t.Fatalf("expected grouped=true")
	}
	wantDirs := []string{"../shared", "../docs", "../workspace"}
	if !reflect.DeepEqual(state.addDirs, wantDirs) {
		t.Fatalf("unexpected addDirs from form\nwant: %#v\ngot:  %#v", wantDirs, state.addDirs)
	}
}

func TestFormInputsApplyForStartWorkflow(t *testing.T) {
	t.Parallel()

	state := newCommandState(core.ModePRDTasks)
	cmd := newTestCommand(state)
	cmd.Flags().String("name", "", "task name")
	cmd.Flags().String("tasks-dir", "", "tasks dir")
	cmd.Flags().Bool("include-completed", false, "include completed")

	fi := &formInputs{
		identifier:       "multi-repo",
		inputDir:         "tasks/prd-multi-repo",
		includeCompleted: true,
	}

	fi.apply(cmd, state)

	if state.name != "multi-repo" {
		t.Fatalf("expected task name to map to name, got %q", state.name)
	}
	if state.tasksDir != "tasks/prd-multi-repo" {
		t.Fatalf("expected tasks dir to map to tasksDir, got %q", state.tasksDir)
	}
	if !state.includeCompleted {
		t.Fatalf("expected includeCompleted=true")
	}
}

func executeRootCommand(args ...string) (string, error) {
	cmd := NewRootCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func newTestCommand(state *commandState) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	addCommonFlags(cmd, state)
	return cmd
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
