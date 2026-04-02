package model_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
)

func TestRuntimeConfigApplyDefaults(t *testing.T) {
	t.Parallel()

	cfg := &model.RuntimeConfig{}
	cfg.ApplyDefaults()

	if cfg.Concurrent != 1 {
		t.Fatalf("unexpected concurrent default: %d", cfg.Concurrent)
	}
	if cfg.BatchSize != 1 {
		t.Fatalf("unexpected batch size default: %d", cfg.BatchSize)
	}
	if cfg.IDE != model.IDECodex {
		t.Fatalf("unexpected ide default: %q", cfg.IDE)
	}
	if cfg.ReasoningEffort != "medium" {
		t.Fatalf("unexpected reasoning default: %q", cfg.ReasoningEffort)
	}
	if cfg.Mode != model.ExecutionModePRReview {
		t.Fatalf("unexpected mode default: %q", cfg.Mode)
	}
	if cfg.Timeout != model.DefaultActivityTimeout {
		t.Fatalf("unexpected timeout default: %s", cfg.Timeout)
	}
	if cfg.RetryBackoffMultiplier != 1.5 {
		t.Fatalf("unexpected retry multiplier default: %f", cfg.RetryBackoffMultiplier)
	}
}

func TestPathHelpers(t *testing.T) {
	t.Parallel()

	if got := model.TasksBaseDir(); got != filepath.Join(".compozy", "tasks") {
		t.Fatalf("unexpected tasks base dir: %q", got)
	}
	if got := model.TaskDirectory("acp-integration"); got != filepath.Join(".compozy", "tasks", "acp-integration") {
		t.Fatalf("unexpected task directory: %q", got)
	}
	baseDir := filepath.Join(string(filepath.Separator), "tmp", "workflows")
	if got := model.ArchivedTasksDir(baseDir); got != filepath.Join(baseDir, "_archived") {
		t.Fatalf("unexpected archived tasks dir: %q", got)
	}
}

func TestIsActiveWorkflowDirName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "regular", input: "workflow-one", want: true},
		{name: "empty", input: "", want: false},
		{name: "hidden", input: ".hidden", want: false},
		{name: "archived", input: model.ArchivedWorkflowDirName, want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := model.IsActiveWorkflowDirName(tc.input); got != tc.want {
				t.Fatalf("unexpected active workflow result for %q: got %v want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestJobIssueCount(t *testing.T) {
	t.Parallel()

	job := model.Job{
		Groups: map[string][]model.IssueEntry{
			"group-a": {{Name: "issue-1"}, {Name: "issue-2"}},
			"group-b": {{Name: "issue-3"}},
		},
	}

	if got := job.IssueCount(); got != 3 {
		t.Fatalf("unexpected issue count: %d", got)
	}
}

func TestUsageTotalUsesExplicitTotalWhenPresent(t *testing.T) {
	t.Parallel()

	usage := model.Usage{InputTokens: 2, OutputTokens: 3, TotalTokens: 99}
	if got := usage.Total(); got != 99 {
		t.Fatalf("unexpected usage total: %d", got)
	}

	usage = model.Usage{InputTokens: 2, OutputTokens: 3}
	if got := usage.Total(); got != 5 {
		t.Fatalf("unexpected derived usage total: %d", got)
	}
}

func TestRuntimeConfigApplyDefaultsPreservesExplicitValues(t *testing.T) {
	t.Parallel()

	cfg := &model.RuntimeConfig{
		Concurrent:             3,
		BatchSize:              2,
		IDE:                    model.IDEClaude,
		TailLines:              10,
		ReasoningEffort:        "high",
		Mode:                   model.ExecutionModePRDTasks,
		Timeout:                30 * time.Second,
		RetryBackoffMultiplier: 2,
	}
	cfg.ApplyDefaults()

	if cfg.Concurrent != 3 || cfg.BatchSize != 2 || cfg.IDE != model.IDEClaude ||
		cfg.Mode != model.ExecutionModePRDTasks {
		t.Fatalf("apply defaults should preserve explicit values: %#v", cfg)
	}
}
