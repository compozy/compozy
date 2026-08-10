package daemon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/gate"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const loopRuntimeSessionPrefix = "loop"

type loopPromptSessionManager interface {
	Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error)
	Status(ctx context.Context, id string) (*session.Info, error)
	Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error)
	CancelPrompt(ctx context.Context, id string) error
}

type loopJudgeSessionManager interface {
	loopPromptSessionManager
	PromptWithOpts(ctx context.Context, id string, opts session.PromptOpts) (<-chan acp.AgentEvent, error)
	StopWithCause(ctx context.Context, id string, cause session.StopCause, detail string) error
}

type loopGateJudgeRunner struct {
	sessions            loopJudgeSessionManager
	globalWorkspacePath string
	policyGate          *loopSessionPolicyGate
	executions          *loopJudgeExecutionRegistry
}

var _ gate.JudgeRunner = (*loopGateJudgeRunner)(nil)

func (r *loopGateJudgeRunner) Judge(
	ctx context.Context,
	req gate.JudgeRequest,
) (_ gate.JudgeResponse, err error) {
	if r == nil || r.sessions == nil || r.policyGate == nil {
		return gate.JudgeResponse{}, errors.New("daemon: loop judge sessions are unavailable")
	}
	if ctx == nil {
		return gate.JudgeResponse{}, errors.New("daemon: loop judge context is required")
	}
	judgeCtx, finishExecution, err := r.beginExecution(ctx, req.CorrelationID)
	if err != nil {
		return gate.JudgeResponse{}, err
	}
	var executionCleanupErr error
	defer func() { finishExecution(executionCleanupErr) }()
	ctx = judgeCtx
	agent := strings.TrimSpace(req.Agent)
	if agent == "" {
		return gate.JudgeResponse{}, fmt.Errorf("%w: judge agent is required", looppkg.ErrValidation)
	}
	contractOverlay := looppkg.RenderContractBlock(req.Contract)
	opts := session.CreateOpts{
		AgentName:                    agent,
		Provider:                     strings.TrimSpace(req.Runtime.Provider),
		Model:                        strings.TrimSpace(req.Runtime.Model),
		ReasoningEffort:              strings.TrimSpace(req.Runtime.Reasoning),
		Name:                         loopRuntimeSessionName("gate", agent, req.CriterionID),
		ResolvedNetworkParticipation: req.NetworkParticipation,
		PromptOverlay:                contractOverlay,
		ContractOverlay:              contractOverlay,
		Type:                         session.SessionTypeSystem,
	}
	opts.NetworkOwnerKey = participation.OwnerKey(participation.OwnerRef{
		Kind: participation.OwnerKindLoopRun,
		ID:   req.LoopRunID,
	})
	if workspaceID := strings.TrimSpace(req.WorkspaceID); workspaceID != "" {
		opts.Workspace = workspaceID
	} else if strings.TrimSpace(r.globalWorkspacePath) != "" {
		opts.WorkspacePath = strings.TrimSpace(r.globalWorkspacePath)
	} else {
		return gate.JudgeResponse{}, errors.New("daemon: loop judge workspace is required")
	}
	if err := r.policyGate.apply(ctx, &opts, agent, nil); err != nil {
		return gate.JudgeResponse{}, err
	}
	opts.RuntimeMode = session.RuntimeModeVerdictOnly
	opts.Permissions = compozyconfig.PermissionModeDenyAll
	created, createErr := r.sessions.Create(ctx, opts)
	if created == nil {
		if createErr != nil {
			return gate.JudgeResponse{}, createErr
		}
		return gate.JudgeResponse{}, errors.New("daemon: loop judge session create returned nil")
	}
	sessionID := strings.TrimSpace(created.ID)
	if sessionID == "" {
		return gate.JudgeResponse{}, errors.New("daemon: loop judge session create returned an empty id")
	}
	defer func() {
		executionCleanupErr = r.stopJudgeSession(ctx, sessionID, err)
		err = errors.Join(err, executionCleanupErr)
	}()
	if err := r.bindExecution(req.CorrelationID, sessionID); err != nil {
		return gate.JudgeResponse{}, err
	}
	if createErr != nil {
		return gate.JudgeResponse{}, createErr
	}
	info := created.Info()
	if info == nil {
		return gate.JudgeResponse{}, errors.New("daemon: loop judge session create returned nil info")
	}
	result, err := collectLoopJudgeResult(ctx, r.sessions, sessionID, req)
	if err != nil {
		return gate.JudgeResponse{}, err
	}
	return gate.JudgeResponse{
		Raw: result.Text, TokensUsed: result.TokensUsed, TokensReported: result.TokensReported,
	}, nil
}

func (r *loopGateJudgeRunner) stopJudgeSession(ctx context.Context, sessionID string, terminalErr error) error {
	if terminalErr == nil {
		terminalErr = ctx.Err()
	}
	stopCause, detail := loopJudgeStopCause(terminalErr)
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultShutdownTimeout)
	defer cancel()
	if err := r.sessions.StopWithCause(stopCtx, sessionID, stopCause, detail); err != nil {
		return fmt.Errorf("daemon: stop loop judge session %q: %w", sessionID, err)
	}
	return nil
}

func loopJudgeStopCause(err error) (session.StopCause, string) {
	if err == nil {
		return session.CauseCompleted, "loop judge completed"
	}
	if errors.Is(err, context.Canceled) {
		return session.CauseUserRequested, "loop judge canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return session.CauseTimeout, "loop judge timed out"
	}
	return session.CauseFailed, "loop judge failed"
}

type loopGateCommandRunner struct {
	workspaceResolver workspacepkg.RuntimeResolver
}

var _ gate.CommandRunner = loopGateCommandRunner{}

func (r loopGateCommandRunner) RunCommand(
	ctx context.Context,
	req gate.CommandRequest,
) (gate.CommandResult, error) {
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return gate.CommandResult{}, fmt.Errorf("%w: command check is required", looppkg.ErrValidation)
	}
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	if workspaceID == "" {
		return gate.CommandResult{}, fmt.Errorf("%w: command workspace_id is required", looppkg.ErrValidation)
	}
	if r.workspaceResolver == nil {
		return gate.CommandResult{}, workspacepkg.ErrWorkspaceResolverUnavailable
	}
	resolved, err := r.workspaceResolver.Resolve(ctx, workspaceID)
	if err != nil {
		return gate.CommandResult{}, fmt.Errorf("daemon: resolve command workspace %q: %w", workspaceID, err)
	}
	rootDir := strings.TrimSpace(resolved.RootDir)
	if rootDir == "" {
		return gate.CommandResult{}, fmt.Errorf(
			"daemon: command workspace %q: %w",
			workspaceID,
			workspacepkg.ErrWorkspaceRootMissing,
		)
	}
	// #nosec G204 -- shell composition is the authored command contract. Runtime values can reach
	// this boundary only through command templates whose emitted actions end in shellQuote.
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = rootDir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return gate.CommandResult{}, err
		}
	}
	return gate.CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

type lazyToolRegistry struct {
	current func() tools.Registry
}

var _ tools.Registry = lazyToolRegistry{}
var _ gate.ToolCaller = lazyToolRegistry{}

func (r lazyToolRegistry) List(ctx context.Context, scope tools.Scope) ([]tools.ToolView, error) {
	registry, err := r.registry()
	if err != nil {
		return nil, err
	}
	return registry.List(ctx, scope)
}

func (r lazyToolRegistry) Search(
	ctx context.Context,
	scope tools.Scope,
	q tools.SearchQuery,
) ([]tools.ToolView, error) {
	registry, err := r.registry()
	if err != nil {
		return nil, err
	}
	return registry.Search(ctx, scope, q)
}

func (r lazyToolRegistry) Get(ctx context.Context, scope tools.Scope, id tools.ToolID) (tools.ToolView, error) {
	registry, err := r.registry()
	if err != nil {
		return tools.ToolView{}, err
	}
	return registry.Get(ctx, scope, id)
}

func (r lazyToolRegistry) Call(
	ctx context.Context,
	scope tools.Scope,
	req tools.CallRequest,
) (tools.ToolResult, error) {
	registry, err := r.registry()
	if err != nil {
		return tools.ToolResult{}, err
	}
	return registry.Call(ctx, scope, req)
}

func (r lazyToolRegistry) registry() (tools.Registry, error) {
	if r.current == nil {
		return nil, errors.New("daemon: tool registry resolver is unavailable")
	}
	registry := r.current()
	if registry == nil {
		return nil, errors.New("daemon: tool registry is unavailable")
	}
	return registry, nil
}

type lazyLoopStarter struct {
	current func() looppkg.ActionLoopStarter
}

var _ looppkg.ActionLoopStarter = lazyLoopStarter{}

func (s lazyLoopStarter) Start(
	ctx context.Context,
	ws looppkg.WorkspaceID,
	name string,
	inputs looppkg.Inputs,
	actor taskpkg.ActorContext,
) (*looppkg.Run, error) {
	if s.current == nil {
		return nil, errors.New("daemon: loop starter resolver is unavailable")
	}
	starter := s.current()
	if starter == nil {
		return nil, errors.New("daemon: loop starter is unavailable")
	}
	return starter.Start(ctx, ws, name, inputs, actor)
}

func collectLoopPromptResult(
	ctx context.Context,
	sessions loopPromptSessionManager,
	sessionID string,
	req looppkg.ActionPromptRequest,
) (looppkg.ActionPromptResult, error) {
	if strings.TrimSpace(sessionID) == "" {
		return looppkg.ActionPromptResult{}, errors.New("daemon: loop prompt session id is required")
	}
	events, err := sessions.Prompt(ctx, sessionID, req.Message)
	if err != nil {
		return looppkg.ActionPromptResult{}, err
	}
	var agentText strings.Builder
	var allText strings.Builder
	var tokensUsed int64
	tokensReported := false
	for event := range events {
		// Chunks are streaming deltas of one turn, not lines: any injected
		// separator lands inside JSON string literals and corrupts the answer.
		// The transcript projector concatenates the same way.
		if strings.TrimSpace(event.Text) != "" {
			if event.Type == acp.EventTypeAgentMessage {
				agentText.WriteString(event.Text)
			}
			allText.WriteString(event.Text)
		}
		if tokens, ok := loopPromptTokensUsed(event.Usage); ok {
			tokensUsed = tokens
			tokensReported = true
			if req.UsageReporter != nil {
				req.UsageReporter.ReportActionTokensUsed(tokens)
			}
		}
	}
	// Agent-message chunks are the action answer; thought and user chunks only
	// backstop providers that never emit an agent message on this turn.
	text := agentText.String()
	if strings.TrimSpace(text) == "" {
		text = allText.String()
	}
	return looppkg.ActionPromptResult{
		Text:           text,
		TokensUsed:     tokensUsed,
		TokensReported: tokensReported,
	}, nil
}

func loopPromptTokensUsed(usage *acp.TokenUsage) (int64, bool) {
	if usage == nil {
		return 0, false
	}
	if usage.TotalTokens != nil {
		return max(*usage.TotalTokens, 0), true
	}
	if usage.InputTokens == nil && usage.OutputTokens == nil {
		return 0, false
	}
	var total int64
	if usage.InputTokens != nil {
		total += max(*usage.InputTokens, 0)
	}
	if usage.OutputTokens != nil {
		total += max(*usage.OutputTokens, 0)
	}
	return total, true
}

func loopRuntimeSessionName(kind string, agent string, suffix string) string {
	parts := []string{
		loopRuntimeSessionPrefix,
		strings.TrimSpace(kind),
		strings.TrimSpace(agent),
		strings.TrimSpace(suffix),
	}
	return strings.Join(nonEmptyStrings(parts), " ")
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
