package executor

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
)

func TestBuildExecutionResultIncludesStatusUsageAndArtifactPaths(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "exec-test-run")
	cfg := &config{
		Mode:         model.ExecutionModeExec,
		IDE:          model.IDECodex,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatJSON,
		RunArtifacts: runArtifacts,
	}
	jobs := []job{{
		SafeName:      "exec",
		CodeFiles:     []string{"exec"},
		Status:        runStatusSucceeded,
		ExitCode:      0,
		OutPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.Prompt.md"),
		OutLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		ErrLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
		Usage: model.Usage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}}

	result := buildExecutionResult(cfg, jobs, nil, nil)

	if result.SchemaVersion != executionResultSchemaVersion {
		t.Fatalf("unexpected schema version: %d", result.SchemaVersion)
	}
	if result.Status != runStatusSucceeded {
		t.Fatalf("unexpected result Status: %q", result.Status)
	}
	if result.Usage.Total() != 15 {
		t.Fatalf("unexpected aggregate Usage: %#v", result.Usage)
	}
	if result.ResultPath != runArtifacts.ResultPath {
		t.Fatalf("unexpected result path: %q", result.ResultPath)
	}
	if len(result.Jobs) != 1 {
		t.Fatalf("expected one job result, got %d", len(result.Jobs))
	}
	if result.Jobs[0].PromptPath != jobs[0].OutPromptPath {
		t.Fatalf("unexpected prompt path: %q", result.Jobs[0].PromptPath)
	}
}

func TestBuildExecutionResultPreservesSpeedIntentAndIndependentJobResolutions(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "speed-result")
	cfg := &config{
		Mode:         model.ExecutionModePRDTasks,
		Speed:        kinds.SpeedFast,
		RunArtifacts: runArtifacts,
	}
	jobs := []job{
		{
			SafeName: "applied",
			Status:   runStatusSucceeded,
			SpeedResolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusApplied,
			},
		},
		{
			SafeName: "unsupported",
			Status:   runStatusSucceeded,
			SpeedResolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusUnsupported,
				Reason:    kinds.SpeedResolutionReasonCapabilityAbsent,
			},
		},
		{
			SafeName: "rejected",
			Status:   runStatusFailed,
			SpeedResolution: kinds.SpeedResolution{
				Requested: kinds.SpeedFast,
				Status:    kinds.SpeedResolutionStatusRejected,
				Reason:    kinds.SpeedResolutionReasonProviderRejected,
			},
		},
	}

	result := buildExecutionResult(cfg, jobs, nil, nil)

	if result.Speed != kinds.SpeedFast {
		t.Fatalf("result speed = %q, want fast", result.Speed)
	}
	if len(result.Jobs) != len(jobs) {
		t.Fatalf("result jobs = %d, want %d", len(result.Jobs), len(jobs))
	}
	for index := range jobs {
		if result.Jobs[index].SpeedResolution == nil {
			t.Fatalf("job %d speed resolution = nil", index)
		}
		if got, want := *result.Jobs[index].SpeedResolution, jobs[index].SpeedResolution; got != want {
			t.Fatalf("job %d speed resolution = %#v, want %#v", index, got, want)
		}
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, exists := raw["speed_resolution"]; exists {
		t.Fatalf("execution result invented aggregate speed_resolution: %s", data)
	}
}

func TestExecutionResultHistoricalJSONCompatibility(t *testing.T) {
	t.Parallel()

	const historical = `{"schema_version":1,"run_id":"run-old","mode":"prd","status":"succeeded","ide":"codex","model":"gpt-5.5","output_format":"json","artifacts_dir":"/tmp/run","run_meta_path":"/tmp/run/meta.json","jobs":[{"safe_name":"task_01","status":"succeeded","exit_code":0,"prompt_path":"","stdout_log_path":"","stderr_log_path":""}]}`

	var result executionResult
	if err := json.Unmarshal([]byte(historical), &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if result.Speed != "" {
		t.Fatalf("historical result speed = %q, want empty", result.Speed)
	}
	if len(result.Jobs) != 1 || result.Jobs[0].SpeedResolution != nil {
		t.Fatalf("historical job speed resolution = %#v, want nil", result.Jobs)
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(data), `"speed"`) || strings.Contains(string(data), `"speed_resolution"`) {
		t.Fatalf("historical result JSON unexpectedly contains speed fields: %s", data)
	}
}

func TestBuildExecutionResultDoesNotInventSuccessForBlankJobStatus(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "exec-test-run")
	cfg := &config{
		Mode:         model.ExecutionModeExec,
		IDE:          model.IDECodex,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatJSON,
		RunArtifacts: runArtifacts,
	}

	result := buildExecutionResult(cfg, []job{{
		SafeName:      "exec",
		CodeFiles:     []string{"exec"},
		OutPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.Prompt.md"),
		OutLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		ErrLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}, []failInfo{{Err: errors.New("setup failed")}}, nil)

	if result.Status != runStatusFailed {
		t.Fatalf("unexpected result Status: %q", result.Status)
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
		Mode:         model.ExecutionModeExec,
		IDE:          model.IDECodex,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatJSON,
		RunArtifacts: runArtifacts,
	}
	jobs := []job{{
		SafeName:      "exec",
		CodeFiles:     []string{"exec"},
		Status:        runStatusFailed,
		ExitCode:      42,
		OutPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.Prompt.md"),
		OutLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		ErrLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}

	result := buildExecutionResult(
		cfg,
		jobs,
		[]failInfo{{Err: errors.New("job failed")}},
		errors.New("ui shutdown failed"),
	)

	if result.Status != runStatusFailed {
		t.Fatalf("unexpected result Status: %q", result.Status)
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
		Mode:         model.ExecutionModeExec,
		IDE:          model.IDECodex,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatJSON,
		RunArtifacts: runArtifacts,
	}
	jobs := []job{{
		SafeName:      "exec",
		CodeFiles:     []string{"exec"},
		Status:        runStatusSucceeded,
		ExitCode:      0,
		OutPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.Prompt.md"),
		OutLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		ErrLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}

	result := buildExecutionResult(cfg, jobs, nil, errors.New("await UI failed"))

	if result.Status != runStatusSucceeded {
		t.Fatalf("unexpected result Status: %q", result.Status)
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
		Mode:         model.ExecutionModeExec,
		IDE:          model.IDECodex,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatJSON,
		RunArtifacts: runArtifacts,
	}
	jobs := []job{{
		SafeName:      "exec",
		CodeFiles:     []string{"exec"},
		Status:        runStatusCanceled,
		ExitCode:      130,
		OutPromptPath: filepath.Join(runArtifacts.JobsDir, "exec.Prompt.md"),
		OutLog:        filepath.Join(runArtifacts.JobsDir, "exec.out.log"),
		ErrLog:        filepath.Join(runArtifacts.JobsDir, "exec.err.log"),
	}}

	result := buildExecutionResult(
		cfg,
		jobs,
		[]failInfo{{Err: errors.New("job failed")}},
		errors.New("teardown failed"),
	)

	if result.Status != runStatusCanceled {
		t.Fatalf("unexpected result Status: %q", result.Status)
	}
	if result.Error != "job failed" {
		t.Fatalf("unexpected primary result error: %q", result.Error)
	}
	if result.TeardownError != "teardown failed" {
		t.Fatalf("unexpected teardown error: %q", result.TeardownError)
	}
}

func TestBuildExecutionResultPrefersFailedStatusOverFailFastCanceledJobs(t *testing.T) {
	t.Parallel()

	runArtifacts := model.NewRunArtifacts(t.TempDir(), "exec-test-run")
	cfg := &config{
		Mode:         model.ExecutionModeExec,
		IDE:          model.IDECursor,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatJSON,
		RunArtifacts: runArtifacts,
	}
	jobs := []job{
		{
			SafeName:      "first",
			CodeFiles:     []string{"first"},
			Status:        runStatusFailed,
			ExitCode:      -1,
			OutPromptPath: filepath.Join(runArtifacts.JobsDir, "first.Prompt.md"),
			OutLog:        filepath.Join(runArtifacts.JobsDir, "first.out.log"),
			ErrLog:        filepath.Join(runArtifacts.JobsDir, "first.err.log"),
		},
		{
			SafeName:      "second",
			CodeFiles:     []string{"second"},
			Status:        runStatusCanceled,
			ExitCode:      -1,
			OutPromptPath: filepath.Join(runArtifacts.JobsDir, "second.Prompt.md"),
			OutLog:        filepath.Join(runArtifacts.JobsDir, "second.out.log"),
			ErrLog:        filepath.Join(runArtifacts.JobsDir, "second.err.log"),
		},
	}

	result := buildExecutionResult(
		cfg,
		jobs,
		[]failInfo{{Err: errors.New("cursor-agent is not authenticated")}},
		nil,
	)

	if result.Status != runStatusFailed {
		t.Fatalf("unexpected result Status: %q", result.Status)
	}
	if result.Error != "cursor-agent is not authenticated" {
		t.Fatalf("unexpected primary result error: %q", result.Error)
	}
}

func TestEmitExecutionResultWritesArtifactForTextModeWithoutStdout(t *testing.T) {
	runArtifacts := model.NewRunArtifacts(t.TempDir(), "workflow-run")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := &config{
		Mode:         model.ExecutionModePRDTasks,
		IDE:          model.IDECodex,
		Model:        "gpt-5.5",
		OutputFormat: model.OutputFormatText,
		RunArtifacts: runArtifacts,
	}
	result := executionResult{
		RunID:        runArtifacts.RunID,
		Mode:         string(cfg.Mode),
		Status:       runStatusSucceeded,
		IDE:          cfg.IDE,
		Model:        cfg.Model,
		OutputFormat: string(cfg.OutputFormat),
		ArtifactsDir: runArtifacts.RunDir,
		RunMetaPath:  runArtifacts.RunMetaPath,
		ResultPath:   runArtifacts.ResultPath,
	}

	stdoutBytes := captureExecutionStdout(t, func() {
		if err := emitExecutionResult(cfg, result); err != nil {
			t.Fatalf("emitExecutionResult: %v", err)
		}
	})

	resultBytes, err := os.ReadFile(runArtifacts.ResultPath)
	if err != nil {
		t.Fatalf("read result artifact: %v", err)
	}
	if !bytes.Contains(resultBytes, []byte(`"status": "succeeded"`)) {
		t.Fatalf("unexpected result artifact payload: %s", string(resultBytes))
	}
	if !bytes.Contains(resultBytes, []byte(`"schema_version": 1`)) {
		t.Fatalf("expected schema version in result artifact: %s", string(resultBytes))
	}
	if len(stdoutBytes) != 0 {
		t.Fatalf("expected text mode to keep stdout quiet, got %q", string(stdoutBytes))
	}
}

func TestEmitExecutionResultKeepsWorkflowJSONModesQuietOnStdout(t *testing.T) {
	runArtifacts := model.NewRunArtifacts(t.TempDir(), "workflow-run")
	if err := os.MkdirAll(runArtifacts.RunDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	for _, format := range []model.OutputFormat{model.OutputFormatJSON, model.OutputFormatRawJSON} {
		cfg := &config{
			Mode:         model.ExecutionModePRDTasks,
			IDE:          model.IDECodex,
			Model:        "gpt-5.5",
			OutputFormat: format,
			RunArtifacts: runArtifacts,
		}
		result := executionResult{
			RunID:        runArtifacts.RunID,
			Mode:         string(cfg.Mode),
			Status:       runStatusSucceeded,
			IDE:          cfg.IDE,
			Model:        cfg.Model,
			OutputFormat: string(cfg.OutputFormat),
			ArtifactsDir: runArtifacts.RunDir,
			RunMetaPath:  runArtifacts.RunMetaPath,
			ResultPath:   runArtifacts.ResultPath,
		}

		stdoutBytes := captureExecutionStdout(t, func() {
			if err := emitExecutionResult(cfg, result); err != nil {
				t.Fatalf("emitExecutionResult: %v", err)
			}
		})

		if len(stdoutBytes) != 0 {
			t.Fatalf("expected workflow %s mode to keep stdout quiet, got %q", format, string(stdoutBytes))
		}
	}
}

func captureExecutionStdout(t *testing.T, run func()) []byte {
	t.Helper()

	captureExecuteStreamsMu.Lock()
	defer captureExecuteStreamsMu.Unlock()

	originalStdout := os.Stdout
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writePipe
	defer func() {
		os.Stdout = originalStdout
	}()

	run()

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
	return stdoutBytes
}
