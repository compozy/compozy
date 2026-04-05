package run

import (
	"errors"
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

func TestBuildExecutionResultDoesNotInventSuccessForBlankJobStatus(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "exec-test-run")
	cfg := &config{
		mode:         model.ExecutionModeExec,
		ide:          model.IDECodex,
		model:        "gpt-5.4",
		outputFormat: model.OutputFormatJSON,
		runArtifacts: runArtifacts,
	}

	result := buildExecutionResult(cfg, []job{{
		safeName:      "exec",
		codeFiles:     []string{"exec"},
		outPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.prompt.md"),
		outLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		errLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}, []failInfo{{err: errors.New("setup failed")}}, nil)

	if result.Status != runStatusFailed {
		t.Fatalf("unexpected result status: %q", result.Status)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected one job result, got %d", len(result.Jobs))
	}
	if result.Jobs[0].Status != runStatusUnknown {
		t.Fatalf("expected blank job status to remain non-success, got %q", result.Jobs[0].Status)
	}
}

func TestBuildExecutionResultKeepsPrimaryFailureWhenTeardownAlsoFails(t *testing.T) {
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
		status:        runStatusFailed,
		exitCode:      42,
		outPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.prompt.md"),
		outLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		errLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}

	result := buildExecutionResult(
		cfg,
		jobs,
		[]failInfo{{err: errors.New("job failed")}},
		errors.New("ui shutdown failed"),
	)

	if result.Status != runStatusFailed {
		t.Fatalf("unexpected result status: %q", result.Status)
	}
	if result.Error != "job failed" {
		t.Fatalf("unexpected primary result error: %q", result.Error)
	}
	if result.TeardownError != "ui shutdown failed" {
		t.Fatalf("unexpected teardown error: %q", result.TeardownError)
	}
}

func TestBuildExecutionResultDoesNotCancelSuccessfulJobsOnTeardownFailure(t *testing.T) {
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
	}}

	result := buildExecutionResult(cfg, jobs, nil, errors.New("await UI failed"))

	if result.Status != runStatusSucceeded {
		t.Fatalf("unexpected result status: %q", result.Status)
	}
	if result.Error != "" {
		t.Fatalf("expected no primary error, got %q", result.Error)
	}
	if result.TeardownError != "await UI failed" {
		t.Fatalf("unexpected teardown error: %q", result.TeardownError)
	}
}
