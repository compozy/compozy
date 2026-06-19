package recovery

import (
	"context"
	"errors"
	"strings"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/model"
	execpkg "github.com/compozy/compozy/internal/core/run/exec"
	"github.com/compozy/compozy/internal/core/workspace"
)

const agenticRemediationName = "agentic"

type preparedPromptExecutor func(
	context.Context,
	*model.RuntimeConfig,
	string,
	*reusableagents.ExecutionContext,
	execpkg.SessionMCPBuilder,
) (execpkg.PreparedPromptResult, error)

// AgenticRemediation launches a recovery agent through the exec pipeline.
type AgenticRemediation struct {
	executePreparedPrompt preparedPromptExecutor
}

// AgenticRemediationOption configures AgenticRemediation.
type AgenticRemediationOption func(*AgenticRemediation)

// WithPreparedPromptExecutor overrides the exec helper for tests.
func WithPreparedPromptExecutor(fn preparedPromptExecutor) AgenticRemediationOption {
	return func(strategy *AgenticRemediation) {
		if fn != nil {
			strategy.executePreparedPrompt = fn
		}
	}
}

// NewAgenticRemediation constructs the default agentic recovery strategy.
func NewAgenticRemediation(opts ...AgenticRemediationOption) *AgenticRemediation {
	strategy := &AgenticRemediation{
		executePreparedPrompt: execpkg.ExecutePreparedPrompt,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(strategy)
		}
	}
	return strategy
}

var _ RemediationStrategy = (*AgenticRemediation)(nil)

// Name returns the stable strategy name used in events and logs.
func (a *AgenticRemediation) Name() string {
	return agenticRemediationName
}

// Remediate captures a diff audit, runs the recovery agent, and parses its
// constrained JSON verdict.
func (a *AgenticRemediation) Remediate(ctx context.Context, in RemediationInput) (TriageVerdict, error) {
	if ctx == nil {
		return rejectVerdict("recovery context is nil"), errors.New("agentic remediation: nil context")
	}
	if a == nil || a.executePreparedPrompt == nil {
		return rejectVerdict(
				"recovery strategy is not configured",
			), errors.New(
				"agentic remediation: missing prompt executor",
			)
	}
	systemPrompt, err := buildRecoverySystemPrompt(in)
	if err != nil {
		return rejectVerdict("failed to build recovery system prompt"), err
	}
	runtimeCfg := buildRecoveryRuntimeConfig(in, systemPrompt)
	audit, err := BeginDiffAudit(ctx, runtimeCfg.WorkspaceRoot, auditArtifactsForOutcome(in.Outcome))
	if err != nil {
		return rejectVerdict("failed to capture recovery diff audit baseline"), err
	}
	result, runErr := a.executePreparedPrompt(ctx, &runtimeCfg, buildRecoveryPrompt(), nil, nil)
	_, auditErr := audit.Complete(ctx)
	if auditErr != nil && runErr != nil {
		return rejectVerdict("recovery run and diff audit completion failed"), errors.Join(runErr, auditErr)
	}
	if auditErr != nil {
		return rejectVerdict("failed to complete recovery diff audit"), auditErr
	}
	if runErr != nil {
		return rejectVerdict("recovery agent run failed"), runErr
	}
	return ParseTriageVerdict(result.Output), nil
}

func buildRecoveryRuntimeConfig(in RemediationInput, systemPrompt string) model.RuntimeConfig {
	cfg := model.RuntimeConfig{}
	if in.FailedConfig != nil {
		cfg = *in.FailedConfig
	}
	recoveryCfg := in.Recovery.ApplyDefaults()
	applyRecoveryRuntime(&cfg, recoveryCfg)
	cfg.Mode = model.ExecutionModeExec
	cfg.OutputFormat = model.OutputFormatText
	cfg.TUI = false
	cfg.Persist = true
	cfg.DaemonOwned = true
	cfg.AgentName = ""
	cfg.RunID = ""
	cfg.ParentRunID = strings.TrimSpace(in.Outcome.RunID)
	cfg.PromptText = ""
	cfg.ResolvedPromptText = ""
	cfg.PromptFile = ""
	cfg.ReadPromptStdin = false
	cfg.SystemPrompt = systemPrompt
	cfg.RecoveryAttempt = 1
	cfg.Recursive = false
	if cfg.Timeout <= 0 {
		cfg.Timeout = model.DefaultActivityTimeout
	}
	cfg.ApplyDefaults()
	return cfg
}

func applyRecoveryRuntime(cfg *model.RuntimeConfig, recoveryCfg workspace.AgentRecoveryConfig) {
	if cfg == nil {
		return
	}
	if value := stringPtrValue(recoveryCfg.IDE); value != "" {
		cfg.IDE = value
	}
	if value := stringPtrValue(recoveryCfg.Model); value != "" {
		cfg.Model = value
	}
	if value := stringPtrValue(recoveryCfg.ReasoningEffort); value != "" {
		cfg.ReasoningEffort = value
	}
}

func auditArtifactsForOutcome(outcome RunOutcome) model.RunArtifacts {
	runDir := strings.TrimSpace(outcome.ArtifactsDir)
	return model.RunArtifacts{
		RunID:       strings.TrimSpace(outcome.RunID),
		RunDir:      runDir,
		RecoveryDir: "",
	}
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
