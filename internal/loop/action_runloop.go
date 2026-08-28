package loop

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// RunLoopActionExecutor starts a child loop for await or detach semantics.
type RunLoopActionExecutor struct {
	starter ActionLoopStarter
}

// Execute starts the child through loop.Service.Start.
func (e *RunLoopActionExecutor) Execute(
	ctx context.Context,
	node dsl.Node,
	in ActionExecutionInput,
) (ActionRawResult, error) {
	if e == nil || e.starter == nil {
		return ActionRawResult{}, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "loop_starter"},
		)
	}
	runCtx, cancel, err := actionContextWithNodeTimeout(ctx, node.Timeout)
	if err != nil {
		return ActionRawResult{}, err
	}
	defer cancel()
	params, err := actionParams(node, in)
	if err != nil {
		return ActionRawResult{}, err
	}
	var spec dsl.RunLoopParams
	if err := dsl.NodeParams(params).Decode(&spec); err != nil {
		return ActionRawResult{}, fmt.Errorf("decode run-loop params: %w", err)
	}
	if spec.Mode == "" {
		spec.Mode = dsl.RunLoopAwait
	}
	configOverrides, err := materializeRunLoopConfigOverrides(params)
	if err != nil {
		return ActionRawResult{}, err
	}
	child, err := e.starter.Start(runCtx, in.WorkspaceID, spec.Loop, Inputs{
		ProfileID:            in.ToolScope.ProfileID,
		Values:               spec.Inputs,
		ParentLoopRunID:      in.LoopRunID,
		ConfigOverrides:      configOverrides,
		InheritedEnvironment: cloneEnvironmentSpec(in.EnvironmentValue()),
	}, in.Actor)
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("start child loop %q: %w", spec.Loop, err)
	}
	if child == nil {
		return ActionRawResult{}, fmt.Errorf("%w: loop starter returned nil run", ErrActionDependencyMissing)
	}
	payload := map[string]any{metadataLoopRunIDKey: string(child.ID)}
	if spec.Mode == dsl.RunLoopAwait {
		payload["status"] = generationOutputAwaitingChild
	}
	structured, err := json.Marshal(payload)
	if err != nil {
		return ActionRawResult{}, fmt.Errorf("marshal run-loop output: %w", err)
	}
	raw := ActionRawResult{
		Structured:     structured,
		Value:          payload,
		ChildLoopRunID: child.ID,
	}
	if spec.Mode == dsl.RunLoopAwait {
		raw.Status = generationOutputAwaitingChild
	}
	// Detach intentionally emits no awaiting status; the coordinator should not wait for the child terminal wake.
	return raw, nil
}

func materializeRunLoopConfigOverrides(params map[string]any) (LoopConfig, error) {
	raw, ok := params["config_overrides"]
	if !ok || raw == nil {
		return LoopConfig{}, nil
	}
	normalized, err := normalizeJSONValue(raw)
	if err != nil {
		return LoopConfig{}, fmt.Errorf("run-loop config_overrides: normalize value: %w", err)
	}
	overrides, ok := normalized.(map[string]any)
	if !ok {
		return LoopConfig{}, fmt.Errorf(
			"run-loop config_overrides: expected object, got %T",
			normalized,
		)
	}
	return decodeRunLoopConfigOverrides(overrides)
}

func decodeRunLoopConfigOverrides(raw map[string]any) (LoopConfig, error) {
	if len(raw) == 0 {
		return LoopConfig{}, nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return LoopConfig{}, fmt.Errorf("run-loop config_overrides: encode JSON: %w", err)
	}
	var publicConfig runLoopPublicConfigOverrides
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&publicConfig); err != nil {
		return LoopConfig{}, fmt.Errorf("run-loop config_overrides: decode JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return LoopConfig{}, errors.New("run-loop config_overrides: expected one JSON value")
		}
		return LoopConfig{}, fmt.Errorf("run-loop config_overrides: decode trailing JSON: %w", err)
	}
	return publicConfig.loopConfig(), nil
}

// runLoopPublicConfigOverrides excludes operator-owned lifecycle and request-expiry policy.
type runLoopPublicConfigOverrides struct {
	HumanGateEnabled  *bool                `json:"human_gate_enabled,omitempty"`
	ReattemptStrategy *ReattemptStrategy   `json:"reattempt_strategy,omitempty"`
	EnabledChecks     json.RawMessage      `json:"enabled_checks_json,omitempty"`
	IterationCap      *int                 `json:"iteration_cap,omitempty"`
	BudgetTokens      *int                 `json:"budget_tokens,omitempty"`
	BudgetWallSec     *int                 `json:"budget_wall_sec,omitempty"`
	BudgetOnExceeded  *dsl.BudgetExceeded  `json:"budget_on_exceeded,omitempty"`
	NoProgressWindow  *int                 `json:"no_progress_window,omitempty"`
	FanOutWidth       *int                 `json:"fan_out_width,omitempty"`
	GateMaxRevisions  *int                 `json:"gate_max_revisions,omitempty"`
	RuntimeDefaults   *RuntimeDefaults     `json:"runtime_defaults,omitempty"`
	RuntimeRules      []RuntimeRule        `json:"runtime_rules,omitempty"`
	Environment       *dsl.EnvironmentSpec `json:"environment,omitempty"`
}

func (c runLoopPublicConfigOverrides) loopConfig() LoopConfig {
	return LoopConfig{
		HumanGateEnabled:  c.HumanGateEnabled,
		ReattemptStrategy: c.ReattemptStrategy,
		EnabledChecks:     c.EnabledChecks,
		IterationCap:      c.IterationCap,
		BudgetTokens:      c.BudgetTokens,
		BudgetWallSec:     c.BudgetWallSec,
		BudgetOnExceeded:  c.BudgetOnExceeded,
		NoProgressWindow:  c.NoProgressWindow,
		FanOutWidth:       c.FanOutWidth,
		GateMaxRevisions:  c.GateMaxRevisions,
		RuntimeDefaults:   c.RuntimeDefaults,
		RuntimeRules:      c.RuntimeRules,
		Environment:       c.Environment,
	}
}

// Harvest returns the run-loop child id and await status.
func (e *RunLoopActionExecutor) Harvest(_ context.Context, raw ActionRawResult, node dsl.Node) (ActionOutput, error) {
	if err := validateSyncHarvest(node); err != nil {
		return ActionOutput{}, err
	}
	return outputFromRaw(raw)
}
