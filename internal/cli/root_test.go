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
		"looper fetch-reviews",
		"looper fix-reviews",
		"looper start",
		"fetch-reviews",
		"fix-reviews",
		"start",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected root help to include %q\noutput:\n%s", snippet, output)
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

	required := []string{"--provider", "--pr", "--name", "--round", "--form"}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected fetch-reviews help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--reviews-dir", "--tasks-dir", "--batch-size", "--grouped", "--include-resolved"}
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
		"--name",
		"--round",
		"--reviews-dir",
		"--batch-size",
		"--grouped",
		"--include-resolved",
		"--form",
	}
	for _, snippet := range required {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected fix-reviews help to include %q\noutput:\n%s", snippet, output)
		}
	}

	forbidden := []string{"--provider", "--pr", "--tasks-dir", "--include-completed"}
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

	forbidden := []string{"--pr", "--provider", "--reviews-dir", "--batch-size", "--grouped", "--include-resolved"}
	for _, snippet := range forbidden {
		if strings.Contains(output, snippet) {
			t.Fatalf("expected start help to omit %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestBuildConfigNormalizesReviewAddDirs(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindFixReviews, core.ModePRReview)
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

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.name = "multi-repo"
	state.tasksDir = "tasks/prd-multi-repo"
	state.includeCompleted = true

	cfg, err := state.buildConfig()
	if err != nil {
		t.Fatalf("buildConfig: %v", err)
	}
	if cfg.Name != "multi-repo" {
		t.Fatalf("expected Name field to carry task name, got %q", cfg.Name)
	}
	if cfg.TasksDir != "tasks/prd-multi-repo" {
		t.Fatalf("expected TasksDir to carry tasks dir, got %q", cfg.TasksDir)
	}
	if !cfg.IncludeCompleted {
		t.Fatalf("expected IncludeCompleted=true in config")
	}
	if cfg.Mode != core.ModePRDTasks {
		t.Fatalf("expected start mode, got %q", cfg.Mode)
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
	cmd.Flags().Bool("grouped", false, "grouped")
	cmd.Flags().Bool("include-resolved", false, "include resolved")

	fi := &formInputs{
		name:            "my-feature",
		reviewsDir:      "tasks/prd-my-feature/reviews-001",
		round:           "2",
		batchSize:       "3",
		addDirs:         " ../shared, ../docs ,, ../shared \n ../workspace ",
		grouped:         true,
		includeResolved: true,
	}

	fi.apply(cmd, state)

	if state.name != "my-feature" {
		t.Fatalf("expected name to be applied, got %q", state.name)
	}
	if state.reviewsDir != "tasks/prd-my-feature/reviews-001" {
		t.Fatalf("expected reviews dir to map to reviewsDir, got %q", state.reviewsDir)
	}
	if state.round != 2 {
		t.Fatalf("expected round 2, got %d", state.round)
	}
	if state.batchSize != 3 {
		t.Fatalf("expected batch size 3, got %d", state.batchSize)
	}
	if !state.grouped {
		t.Fatalf("expected grouped=true")
	}
	if !state.includeResolved {
		t.Fatalf("expected includeResolved=true")
	}
	wantDirs := []string{"../shared", "../docs", "../workspace"}
	if !reflect.DeepEqual(state.addDirs, wantDirs) {
		t.Fatalf("unexpected addDirs from form\nwant: %#v\ngot:  %#v", wantDirs, state.addDirs)
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
		tasksDir:         "tasks/prd-multi-repo",
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
