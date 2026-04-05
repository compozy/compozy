package run

import (
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestBuildExecutionResultIncludesStatusUsageAndArtifactPaths(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "exec-test-run")
	cfg := &config{
		mode:         model.ExecutionModeExec,
		ide:          model.IDECodex,
		model:        "gpt-5.4",
		outputFormat: model.OutputFormatJSON,
		runArtifacts: runArtifacts,
	}
	jobs := []job{{
		safeName:      "exec",
		codeFiles:     []string{"exec"},
		status:        runStatusSucceeded,
		exitCode:      0,
		outPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.prompt.md"),
		outLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		errLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
		usage: model.Usage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}}

	result := buildExecutionResult(cfg, jobs, nil, nil)

	if result.Status != runStatusSucceeded {
		t.Fatalf("unexpected result status: %q", result.Status)
	}
	if result.Usage.Total() != 15 {
		t.Fatalf("unexpected aggregate usage: %#v", result.Usage)
	}
	if result.ResultPath != runArtifacts.ResultPath {
		t.Fatalf("unexpected result path: %q", result.ResultPath)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected one job result, got %d", len(result.Jobs))
	}
	if result.Jobs[0].PromptPath != jobs[0].outPromptPath {
		t.Fatalf("unexpected prompt path: %q", result.Jobs[0].PromptPath)
	}
}
