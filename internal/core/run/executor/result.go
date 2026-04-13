package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

const (
	runStatusSucceeded = "succeeded"
	runStatusFailed    = "failed"
	runStatusCanceled  = "canceled"
	runStatusUnknown   = "unknown"
)

type executionResult struct {
	RunID         string             `json:"run_id"`
	Mode          string             `json:"mode"`
	Status        string             `json:"status"`
	IDE           string             `json:"ide"`
	Model         string             `json:"model"`
	OutputFormat  string             `json:"output_format"`
	ArtifactsDir  string             `json:"artifacts_dir"`
	RunMetaPath   string             `json:"run_meta_path"`
	ResultPath    string             `json:"result_path,omitempty"`
	Usage         model.Usage        `json:"usage,omitempty"`
	Error         string             `json:"error,omitempty"`
	TeardownError string             `json:"teardown_error,omitempty"`
	Jobs          []executionJobInfo `json:"jobs"`
}

type executionJobInfo struct {
	SafeName      string      `json:"safe_name"`
	CodeFiles     []string    `json:"code_files,omitempty"`
	Status        string      `json:"status"`
	ExitCode      int         `json:"exit_code"`
	PromptPath    string      `json:"prompt_path"`
	StdoutLogPath string      `json:"stdout_log_path"`
	StderrLogPath string      `json:"stderr_log_path"`
	Usage         model.Usage `json:"usage,omitempty"`
	Error         string      `json:"error,omitempty"`
}

func buildExecutionResult(cfg *config, jobs []job, failures []failInfo, shutdownErr error) executionResult {
	result := executionResult{
		RunID:        cfg.RunArtifacts.RunID,
		Mode:         string(cfg.Mode),
		Status:       deriveRunStatus(jobs, failures),
		IDE:          cfg.IDE,
		Model:        cfg.Model,
		OutputFormat: string(cfg.OutputFormat),
		ArtifactsDir: cfg.RunArtifacts.RunDir,
		RunMetaPath:  cfg.RunArtifacts.RunMetaPath,
		ResultPath:   cfg.RunArtifacts.ResultPath,
		Jobs:         make([]executionJobInfo, 0, len(jobs)),
	}
	for idx := range jobs {
		item := &jobs[idx]
		result.Jobs = append(result.Jobs, executionJobInfo{
			SafeName:      item.SafeName,
			CodeFiles:     append([]string(nil), item.CodeFiles...),
			Status:        jobStatusOrDefault(item.Status),
			ExitCode:      item.ExitCode,
			PromptPath:    item.OutPromptPath,
			StdoutLogPath: item.OutLog,
			StderrLogPath: item.ErrLog,
			Usage:         item.Usage,
			Error:         item.Failure,
		})
		result.Usage.Add(item.Usage)
	}
	if len(failures) > 0 {
		result.Error = failures[0].Err.Error()
	}
	if shutdownErr != nil {
		result.TeardownError = shutdownErr.Error()
	}
	return result
}

func deriveRunStatus(jobs []job, failures []failInfo) string {
	for idx := range jobs {
		if jobs[idx].Status == runStatusCanceled {
			return runStatusCanceled
		}
	}
	if len(failures) > 0 {
		return runStatusFailed
	}
	return runStatusSucceeded
}

func jobStatusOrDefault(status string) string {
	if strings.TrimSpace(status) == "" {
		return runStatusUnknown
	}
	return status
}

func emitExecutionResult(cfg *config, result executionResult) error {
	if cfg == nil {
		return nil
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal exec result: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.RunArtifacts.ResultPath), 0o755); err != nil {
		return fmt.Errorf("create exec result directory: %w", err)
	}
	if err := os.WriteFile(cfg.RunArtifacts.ResultPath, payload, 0o600); err != nil {
		return fmt.Errorf("write exec result: %w", err)
	}
	if cfg.Mode != model.ExecutionModeExec {
		return nil
	}
	stdoutPayload := payload
	switch cfg.OutputFormat {
	case model.OutputFormatJSON:
	case model.OutputFormatRawJSON:
		stdoutPayload, err = json.Marshal(result)
		if err != nil {
			return fmt.Errorf("marshal raw exec result: %w", err)
		}
	default:
		return nil
	}
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", stdoutPayload); err != nil {
		return fmt.Errorf("write exec result stdout: %w", err)
	}
	return nil
}
