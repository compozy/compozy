package run

import (
	"encoding/json"
	"fmt"
	"os"
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
	RunID        string             `json:"run_id"`
	Mode         string             `json:"mode"`
	Status       string             `json:"status"`
	IDE          string             `json:"ide"`
	Model        string             `json:"model"`
	OutputFormat string             `json:"output_format"`
	ArtifactsDir string             `json:"artifacts_dir"`
	RunMetaPath  string             `json:"run_meta_path"`
	ResultPath   string             `json:"result_path,omitempty"`
	Usage        model.Usage        `json:"usage,omitempty"`
	Error        string             `json:"error,omitempty"`
	Jobs         []executionJobInfo `json:"jobs"`
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
		RunID:        cfg.runArtifacts.RunID,
		Mode:         string(cfg.mode),
		Status:       deriveRunStatus(jobs, failures, shutdownErr),
		IDE:          cfg.ide,
		Model:        cfg.model,
		OutputFormat: string(cfg.outputFormat),
		ArtifactsDir: cfg.runArtifacts.RunDir,
		RunMetaPath:  cfg.runArtifacts.RunMetaPath,
		ResultPath:   cfg.runArtifacts.ResultPath,
		Jobs:         make([]executionJobInfo, 0, len(jobs)),
	}
	for idx := range jobs {
		item := &jobs[idx]
		result.Jobs = append(result.Jobs, executionJobInfo{
			SafeName:      item.safeName,
			CodeFiles:     append([]string(nil), item.codeFiles...),
			Status:        jobStatusOrDefault(item.status),
			ExitCode:      item.exitCode,
			PromptPath:    item.outPromptPath,
			StdoutLogPath: item.outLog,
			StderrLogPath: item.errLog,
			Usage:         item.usage,
			Error:         item.failure,
		})
		result.Usage.Add(item.usage)
	}
	if shutdownErr != nil {
		result.Error = shutdownErr.Error()
		return result
	}
	if len(failures) > 0 {
		result.Error = failures[0].err.Error()
	}
	return result
}

func deriveRunStatus(jobs []job, failures []failInfo, shutdownErr error) string {
	if shutdownErr != nil {
		return runStatusCanceled
	}
	if len(failures) > 0 {
		for idx := range jobs {
			if jobs[idx].status == runStatusCanceled {
				return runStatusCanceled
			}
		}
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
	if cfg == nil || cfg.outputFormat != model.OutputFormatJSON {
		return nil
	}

	payload, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal exec result: %w", err)
	}
	if err := os.WriteFile(cfg.runArtifacts.ResultPath, payload, 0o600); err != nil {
		return fmt.Errorf("write exec result: %w", err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", payload); err != nil {
		return fmt.Errorf("write exec result stdout: %w", err)
	}
	return nil
}
