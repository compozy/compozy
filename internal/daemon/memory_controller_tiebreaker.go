package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/controller"
	"github.com/compozy/agh/internal/memory/prompts"
	"github.com/compozy/agh/internal/session"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type transientModelInvoker interface {
	InvokeTransientModel(context.Context, session.TransientModelCall) (session.TransientModelResult, error)
}

type daemonMemoryControllerTiebreaker struct {
	invoker           transientModelInvoker
	roles             *roleResolver
	configSnapshot    func() aghconfig.Config
	workspaceResolver workspacepkg.RuntimeResolver
	globalCWD         string
}

var _ controller.Tiebreaker = (*daemonMemoryControllerTiebreaker)(nil)

type memoryControllerPromptData struct {
	Candidate memcontract.Candidate
	Targets   []controller.Target
	RuleTrace []memcontract.RuleHit
}

type memoryControllerModelOutput struct {
	Op         memcontract.Op `json:"op"`
	TargetID   string         `json:"target_id"`
	Confidence float32        `json:"confidence"`
	Reason     string         `json:"reason"`
}

type memoryControllerInvocation struct {
	route  roleAttemptRoute
	result session.TransientModelResult
}

func (t *daemonMemoryControllerTiebreaker) BreakTie(
	ctx context.Context,
	request controller.TiebreakerRequest,
) (controller.TiebreakerResult, error) {
	if t == nil || t.roles == nil {
		return controller.TiebreakerResult{}, errors.New("daemon: memory controller role resolver is required")
	}
	if t.invoker == nil {
		return controller.TiebreakerResult{}, errors.New("daemon: transient model invoker is required")
	}
	prepared, err := t.prepareCall(ctx, request)
	if err != nil {
		return controller.TiebreakerResult{}, err
	}

	callCtx, cancel := memoryControllerCallContext(
		ctx,
		prepared.options.Timeout,
		prepared.options.Config.Memory.Controller.MaxLatency,
	)
	defer cancel()
	startedAt := time.Now()
	lastRoute := roleAttemptRoute{
		Provider:        prepared.options.resolvedRole.Provider,
		Model:           prepared.options.resolvedRole.Model,
		ReasoningEffort: prepared.options.resolvedRole.ReasoningEffort,
	}
	invocation, invokeErr := invokeRoleWithFallback(
		callCtx,
		prepared.options.resolvedRole,
		memoryControllerCorrelation(request.Candidate),
		func(attemptCtx context.Context, route roleAttemptRoute) (memoryControllerInvocation, bool, error) {
			lastRoute = route
			result, callErr := t.invoker.InvokeTransientModel(attemptCtx, session.TransientModelCall{
				Config:          &prepared.options.Config,
				Provider:        route.Provider,
				Model:           route.Model,
				ReasoningEffort: route.ReasoningEffort,
				CWD:             prepared.cwd,
				Prompt:          prepared.prompt,
				MaxOutputBytes:  prepared.maxOutputBytes,
			})
			return memoryControllerInvocation{route: route, result: result}, result.Accepted, callErr
		},
	)
	return memoryControllerResult(
		invocation,
		lastRoute,
		prepared.options.PromptVersion,
		startedAt,
		invokeErr,
	)
}

type memoryControllerPreparedCall struct {
	options        memoryControllerCallOptions
	prompt         string
	maxOutputBytes int
	cwd            string
}

func (t *daemonMemoryControllerTiebreaker) prepareCall(
	ctx context.Context,
	request controller.TiebreakerRequest,
) (memoryControllerPreparedCall, error) {
	roles := t.roles
	if t.configSnapshot != nil {
		cfg := t.configSnapshot()
		roles = newRoleResolver(&cfg, t.roles.workspaceResolver, t.roles.agents, t.roles.events)
	}
	options, err := roles.resolveMemoryControllerCallOptions(ctx, request.Candidate.WorkspaceID)
	if err != nil {
		return memoryControllerPreparedCall{}, err
	}
	if !options.resolvedRole.Enabled {
		return memoryControllerPreparedCall{}, controller.ErrTiebreakerDisabled
	}
	prompt, err := renderMemoryControllerPrompt(request, options.TopK, options.PromptVersion)
	if err != nil {
		return memoryControllerPreparedCall{}, err
	}
	maxOutputBytes, err := memoryControllerOutputBytes(options.MaxTokensOut)
	if err != nil {
		return memoryControllerPreparedCall{}, err
	}
	cwd, err := t.resolveCWD(ctx, request.Candidate.WorkspaceID)
	if err != nil {
		return memoryControllerPreparedCall{}, err
	}
	return memoryControllerPreparedCall{
		options:        options,
		prompt:         prompt,
		maxOutputBytes: maxOutputBytes,
		cwd:            cwd,
	}, nil
}

func memoryControllerResult(
	invocation memoryControllerInvocation,
	lastRoute roleAttemptRoute,
	promptVersion string,
	startedAt time.Time,
	invokeErr error,
) (controller.TiebreakerResult, error) {
	call := &memcontract.LLMCall{
		Model:         firstRoleValue(invocation.route.Model, lastRoute.Model),
		PromptVersion: promptVersion,
		Latency:       time.Since(startedAt),
		RawResponse:   strings.TrimSpace(invocation.result.Output),
	}
	if invokeErr != nil {
		call.Error = invokeErr.Error()
		return controller.TiebreakerResult{Call: call}, invokeErr
	}
	parsed, err := parseMemoryControllerOutput(invocation.result.Output)
	if err != nil {
		call.Error = err.Error()
		return controller.TiebreakerResult{Call: call}, err
	}
	return controller.TiebreakerResult{
		Op:         parsed.Op,
		TargetID:   strings.TrimSpace(parsed.TargetID),
		Confidence: parsed.Confidence,
		Reason:     strings.TrimSpace(parsed.Reason),
		Call:       call,
	}, nil
}

func renderMemoryControllerPrompt(
	request controller.TiebreakerRequest,
	topK int,
	promptVersion string,
) (string, error) {
	template, err := prompts.ParseTemplate(prompts.NameDecide, promptVersion)
	if err != nil {
		return "", fmt.Errorf("daemon: load memory controller prompt %q: %w", promptVersion, err)
	}
	targets := request.Targets
	if topK > 0 && len(targets) > topK {
		targets = targets[:topK]
	}
	var rendered bytes.Buffer
	if err := template.Execute(&rendered, memoryControllerPromptData{
		Candidate: request.Candidate,
		Targets:   targets,
		RuleTrace: request.RuleTrace,
	}); err != nil {
		return "", fmt.Errorf("daemon: render memory controller prompt: %w", err)
	}
	return rendered.String(), nil
}

func parseMemoryControllerOutput(raw string) (memoryControllerModelOutput, error) {
	decoder := json.NewDecoder(strings.NewReader(strings.TrimSpace(raw)))
	decoder.DisallowUnknownFields()
	var output memoryControllerModelOutput
	if err := decoder.Decode(&output); err != nil {
		return memoryControllerModelOutput{}, fmt.Errorf("daemon: decode memory controller output: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return memoryControllerModelOutput{}, errors.New(
			"daemon: memory controller output must contain exactly one JSON object",
		)
	}
	if err := output.Op.Validate(); err != nil {
		return memoryControllerModelOutput{}, fmt.Errorf("daemon: validate memory controller output op: %w", err)
	}
	if output.Confidence < 0 || output.Confidence > 1 {
		return memoryControllerModelOutput{}, fmt.Errorf(
			"daemon: memory controller confidence must be between 0 and 1: %f",
			output.Confidence,
		)
	}
	if strings.TrimSpace(output.Reason) == "" {
		return memoryControllerModelOutput{}, errors.New("daemon: memory controller reason is required")
	}
	return output, nil
}

func memoryControllerOutputBytes(maxTokens int) (int, error) {
	if maxTokens <= 0 || maxTokens > math.MaxInt/4 {
		return 0, fmt.Errorf("daemon: invalid memory controller output-token bound: %d", maxTokens)
	}
	return maxTokens * 4, nil
}

func memoryControllerCallContext(
	ctx context.Context,
	roleTimeout time.Duration,
	controllerTimeout time.Duration,
) (context.Context, context.CancelFunc) {
	timeout := roleTimeout
	if controllerTimeout > 0 && (timeout <= 0 || controllerTimeout < timeout) {
		timeout = controllerTimeout
	}
	if timeout > 0 {
		return context.WithTimeout(ctx, timeout)
	}
	return context.WithCancel(ctx)
}

func memoryControllerCorrelation(candidate memcontract.Candidate) roleInvocationCorrelation {
	correlation := roleInvocationCorrelation{
		WorkspaceID: strings.TrimSpace(candidate.WorkspaceID),
		AgentName:   strings.TrimSpace(candidate.AgentName),
	}
	if candidate.Metadata != nil {
		correlation.SessionID = strings.TrimSpace(candidate.Metadata["session_id"])
	}
	return correlation
}

func (t *daemonMemoryControllerTiebreaker) resolveCWD(ctx context.Context, workspaceID string) (string, error) {
	if target := strings.TrimSpace(workspaceID); target != "" {
		if t.workspaceResolver == nil {
			return "", errors.New("daemon: workspace resolver is required for memory controller call")
		}
		resolved, err := t.workspaceResolver.Resolve(ctx, target)
		if err != nil {
			return "", fmt.Errorf("daemon: resolve memory controller workspace %q: %w", target, err)
		}
		if cwd := strings.TrimSpace(resolved.RootDir); cwd != "" {
			return cwd, nil
		}
	}
	if cwd := strings.TrimSpace(t.globalCWD); cwd != "" {
		return cwd, nil
	}
	return "", errors.New("daemon: memory controller working directory is required")
}
