package run

import (
	"bytes"
	"errors"
	"io"
	"os"
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

func TestBuildExecutionResultKeepsCanceledStatusWhenFailuresArePresent(t *testing.T) {
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
		status:        runStatusCanceled,
		exitCode:      130,
		outPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.prompt.md"),
		outLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		errLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}

	result := buildExecutionResult(
		cfg,
		jobs,
		[]failInfo{{err: errors.New("job failed")}},
		errors.New("teardown failed"),
	)

	if result.Status != runStatusCanceled {
		t.Fatalf("unexpected result status: %q", result.Status)
	}
	if result.Error != "job failed" {
		t.Fatalf("unexpected primary result error: %q", result.Error)
	}
	if result.TeardownError != "teardown failed" {
		t.Fatalf("unexpected teardown error: %q", result.TeardownError)
	}
}

func TestEmitExecutionResultWritesArtifactForTextModeWithoutStdout(t *testing.T) {
	runArtifacts := model.NewRunArtifacts(t.TempDir(), "workflow-run")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := &config{
		mode:         model.ExecutionModePRDTasks,
		ide:          model.IDECodex,
		model:        "gpt-5.4",
		outputFormat: model.OutputFormatText,
		runArtifacts: runArtifacts,
	}
	result := executionResult{
		RunID:        runArtifacts.RunID,
		Mode:         string(cfg.mode),
		Status:       runStatusSucceeded,
		IDE:          cfg.ide,
		Model:        cfg.model,
		OutputFormat: string(cfg.outputFormat),
		ArtifactsDir: runArtifacts.RunDir,
		RunMetaPath:  runArtifacts.RunMetaPath,
		ResultPath:   runArtifacts.ResultPath,
	}

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	if err := emitExecutionResult(cfg, result); err != nil {
		t.Fatalf("emitExecutionResult: %v", err)
	}
	if err := writePipe.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}

	stdoutBytes, err := io.ReadAll(readPipe)
	if err != nil {
		t.Fatalf("read stdout pipe: %v", err)
	}
	if err := readPipe.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	resultBytes, err := os.ReadFile(runArtifacts.ResultPath)
	if err != nil {
		t.Fatalf("read result artifact: %v", err)
	}
	if !bytes.Contains(resultBytes, []byte(`"status": "succeeded"`)) {
		t.Fatalf("unexpected result artifact payload: %s", string(resultBytes))
	}
	if len(stdoutBytes) != 0 {
		t.Fatalf("expected text mode to keep stdout quiet, got %q", string(stdoutBytes))
	}
}
