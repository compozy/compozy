package exec

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/compozy/compozy/internal/core/model"
	promptpkg "github.com/compozy/compozy/internal/core/prompt"
)

const execHookJobID = "exec"

type execPromptPostBuildPayload struct {
	RunID       string                `json:"run_id"`
	JobID       string                `json:"job_id"`
	PromptText  string                `json:"prompt_text"`
	BatchParams promptpkg.BatchParams `json:"batch_params"`
}

type execRunPreStartPayload struct {
	RunID     string              `json:"run_id"`
	Config    model.RuntimeConfig `json:"config"`
	Artifacts model.RunArtifacts  `json:"artifacts"`
}

type execRunPostStartPayload struct {
	RunID  string              `json:"run_id"`
	Config model.RuntimeConfig `json:"config"`
}

type execRunPreShutdownPayload struct {
	RunID  string `json:"run_id"`
	Reason string `json:"reason"`
}

type execRunPostShutdownPayload struct {
	RunID   string           `json:"run_id"`
	Reason  string           `json:"reason"`
	Summary model.RunSummary `json:"summary"`
}

func applyExecRunPreStartHook(ctx context.Context, state *execRunState, cfg *model.RuntimeConfig) error {
	if state == nil || state.runtimeManager == nil || cfg == nil {
		return nil
	}

	payload, err := model.DispatchMutableHook(
		ctx,
		state.runtimeManager,
		"run.pre_start",
		execRunPreStartPayload{
			RunID:     state.runArtifacts.RunID,
			Config:    cloneExecRuntimeConfig(cfg),
			Artifacts: state.runArtifacts,
		},
	)
	if err != nil {
		return fmt.Errorf("dispatch run.pre_start hook: %w", err)
	}

	applyExecRuntimeConfig(cfg, payload.Config)
	return nil
}

func applyExecPromptPostBuildHook(
	ctx context.Context,
	state *execRunState,
	promptText string,
) (string, error) {
	if state == nil || state.runtimeManager == nil {
		return promptText, nil
	}

	payload, err := model.DispatchMutableHook(
		ctx,
		state.runtimeManager,
		"prompt.post_build",
		execPromptPostBuildPayload{
			RunID:      state.runArtifacts.RunID,
			JobID:      execHookJobID,
			PromptText: promptText,
			BatchParams: promptpkg.BatchParams{
				Context:    ctx,
				RunID:      state.runArtifacts.RunID,
				JobID:      execHookJobID,
				RuntimeMgr: state.runtimeManager,
				Mode:       model.ExecutionModeExec,
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("dispatch prompt.post_build hook: %w", err)
	}

	return payload.PromptText, nil
}

func (s *execRunState) dispatchRunPostStart(cfg *model.RuntimeConfig) {
	if s == nil || s.runtimeManager == nil {
		return
	}

	model.DispatchObserverHook(
		s.ctx,
		s.runtimeManager,
		"run.post_start",
		execRunPostStartPayload{
			RunID:  s.runArtifacts.RunID,
			Config: cloneExecRuntimeConfig(cfg),
		},
	)
}

func (s *execRunState) dispatchRunPreShutdown(result execExecutionResult) {
	if s == nil || s.runtimeManager == nil {
		return
	}

	model.DispatchObserverHook(
		s.ctx,
		s.runtimeManager,
		"run.pre_shutdown",
		execRunPreShutdownPayload{
			RunID:  s.runArtifacts.RunID,
			Reason: execShutdownReason(result),
		},
	)
}

func (s *execRunState) dispatchRunPostShutdown(result execExecutionResult) {
	if s == nil || s.runtimeManager == nil {
		return
	}

	_, err := model.DispatchMutableHook(
		s.hookContext(),
		s.runtimeManager,
		"run.post_shutdown",
		execRunPostShutdownPayload{
			RunID:   s.runArtifacts.RunID,
			Reason:  execShutdownReason(result),
			Summary: execRunSummary(result),
		},
	)
	if err != nil {
		slog.Warn(
			"exec run.post_shutdown delivery failed",
			"component", "run.exec",
			"run_id", s.runArtifacts.RunID,
			"err", err,
		)
	}
}

func cloneExecRuntimeConfig(src *model.RuntimeConfig) model.RuntimeConfig {
	if src == nil {
		return model.RuntimeConfig{}
	}

	cloned := *src
	cloned.AddDirs = append([]string(nil), src.AddDirs...)
	return cloned
}

func applyExecRuntimeConfig(dst *model.RuntimeConfig, updated model.RuntimeConfig) {
	if dst == nil {
		return
	}

	*dst = updated
	dst.AddDirs = append([]string(nil), updated.AddDirs...)
}

func execRunSummary(result execExecutionResult) model.RunSummary {
	summary := model.RunSummary{
		Status:    result.status,
		JobsTotal: 1,
		Error:     errorString(result.err),
	}

	switch result.status {
	case runStatusSucceeded:
		summary.JobsSucceeded = 1
	case runStatusCanceled:
		summary.JobsCanceled = 1
	default:
		summary.JobsFailed = 1
	}

	return summary
}

func execShutdownReason(result execExecutionResult) string {
	return result.status
}

func (s *execRunState) hookContext() context.Context {
	if s == nil || s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}
