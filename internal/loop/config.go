package loop

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
)

var emptyChecksJSON = json.RawMessage(`{}`)

// DefaultLoopDefaults returns the built-in fallback `[loops.defaults.*]` layer.
func DefaultLoopDefaults() LoopDefaults {
	halt := dsl.BudgetExceededHalt
	deliveryLifecycle := DefaultLifecycleConfig()
	watchLifecycle := deliveryLifecycle.Clone()
	return LoopDefaults{
		Delivery: LoopConfig{
			IterationCap:     new(50),
			NoProgressWindow: new(3),
			GateMaxRevisions: new(10),
			BudgetTokens:     new(0),
			BudgetWallSec:    new(0),
			BudgetOnExceeded: new(halt),
			FanOutWidth:      new(4),
			Lifecycle:        &deliveryLifecycle,
		},
		Watch: LoopConfig{
			IterationCap:     new(0),
			NoProgressWindow: new(2),
			BudgetTokens:     new(0),
			BudgetWallSec:    new(0),
			BudgetOnExceeded: new(halt),
			FanOutWidth:      new(2),
			Lifecycle:        &watchLifecycle,
		},
	}
}

// ClampLoopConfig clamps one override layer before merge or persistence.
func ClampLoopConfig(cfg LoopConfig) LoopConfig {
	clamped := cfg.Clone()
	clampNonNegative(clamped.IterationCap)
	clampNonNegative(clamped.BudgetTokens)
	clampNonNegative(clamped.BudgetWallSec)
	clampNonNegativeMax(clamped.NoProgressWindow, LoopMaxNoProgressWindow)
	clampNonNegative(clamped.FanOutWidth)
	clampNonNegativeMax(clamped.GateMaxRevisions, LoopMaxGateRevisions)
	if clamped.BudgetOnExceeded != nil && *clamped.BudgetOnExceeded == "" {
		value := dsl.BudgetExceededHalt
		clamped.BudgetOnExceeded = &value
	}
	return clamped
}

// Clone returns a deep copy of a raw config layer.
func (cfg LoopConfig) Clone() LoopConfig {
	cloned := LoopConfig{
		EnabledChecks: cloneRawMessage(cfg.EnabledChecks),
	}
	if cfg.HumanGateEnabled != nil {
		cloned.HumanGateEnabled = new(*cfg.HumanGateEnabled)
	}
	if cfg.ReattemptStrategy != nil {
		cloned.ReattemptStrategy = new(*cfg.ReattemptStrategy)
	}
	if cfg.IterationCap != nil {
		cloned.IterationCap = new(*cfg.IterationCap)
	}
	if cfg.BudgetTokens != nil {
		cloned.BudgetTokens = new(*cfg.BudgetTokens)
	}
	if cfg.BudgetWallSec != nil {
		cloned.BudgetWallSec = new(*cfg.BudgetWallSec)
	}
	if cfg.BudgetOnExceeded != nil {
		cloned.BudgetOnExceeded = new(*cfg.BudgetOnExceeded)
	}
	if cfg.NoProgressWindow != nil {
		cloned.NoProgressWindow = new(*cfg.NoProgressWindow)
	}
	if cfg.FanOutWidth != nil {
		cloned.FanOutWidth = new(*cfg.FanOutWidth)
	}
	if cfg.GateMaxRevisions != nil {
		cloned.GateMaxRevisions = new(*cfg.GateMaxRevisions)
	}
	cloned.RuntimeDefaults = cloneRuntimeDefaults(cfg.RuntimeDefaults)
	cloned.RuntimeRules = cloneRuntimeRules(cfg.RuntimeRules)
	if cfg.Environment != nil {
		environment := *cfg.Environment
		cloned.Environment = &environment
	}
	if cfg.Lifecycle != nil {
		lifecycle := cfg.Lifecycle.Clone()
		cloned.Lifecycle = &lifecycle
	}
	if cfg.RequestExpireAfter != nil {
		cloned.RequestExpireAfter = new(*cfg.RequestExpireAfter)
	}
	return cloned
}

// ResolveEffectiveConfig merges definition, defaults, loop_config, and per-run layers.
func ResolveEffectiveConfig(
	resolved *ResolvedDefinition,
	defaults LoopDefaults,
	stored *LoopConfig,
	perRun LoopConfig,
) (EffectiveConfig, error) {
	return resolveEffectiveConfig(resolved, defaults, nil, stored, perRun)
}

func resolveEffectiveConfig(
	resolved *ResolvedDefinition,
	defaults LoopDefaults,
	inheritedEnvironment *dsl.EnvironmentSpec,
	stored *LoopConfig,
	perRun LoopConfig,
) (EffectiveConfig, error) {
	if resolved == nil {
		return EffectiveConfig{}, fmt.Errorf("%w: resolved definition is required", ErrValidation)
	}
	effective := EffectiveConfig{
		ReattemptStrategy: ReattemptFailedOnly,
		EnabledChecks:     cloneRawMessage(emptyChecksJSON),
		BudgetOnExceeded:  dsl.BudgetExceededHalt,
		Environment:       dsl.EnvironmentSpec{Mode: dsl.EnvironmentRoot},
		Lifecycle:         DefaultLifecycleConfig(),
	}
	mergeConfigLayer(&effective, definitionConfigLayer(resolved.Definition))
	mergeConfigLayer(&effective, defaults.forDefinition(resolved.Definition))
	if inheritedEnvironment != nil {
		inherited := *inheritedEnvironment
		mergeConfigLayer(&effective, LoopConfig{Environment: &inherited})
	}
	if stored != nil {
		mergeConfigLayer(&effective, *stored)
	}
	runRules := cloneRuntimeRules(perRun.RuntimeRules)
	perRun.RuntimeRules = nil
	mergeConfigLayer(&effective, perRun)
	effective.RunRuntimeRules = runRules
	if err := validateEffectiveConfig(effective); err != nil {
		return EffectiveConfig{}, err
	}
	return effective, nil
}

func (defaults LoopDefaults) forDefinition(def dsl.Definition) LoopConfig {
	if definitionLooksWatch(def) {
		return defaults.Watch
	}
	return defaults.Delivery
}

func definitionLooksWatch(def dsl.Definition) bool {
	for _, node := range def.Graph.Nodes {
		if node.Class == dsl.NodeClassSource && node.Kind == string(dsl.SourceWatchSource) {
			return true
		}
	}
	return false
}

func definitionConfigLayer(def dsl.Definition) LoopConfig {
	onExceeded := def.Contract.Budget.OnExceeded
	if onExceeded == "" {
		onExceeded = dsl.BudgetExceededHalt
	}
	runtimeDefaults, runtimeRules := definitionRuntimeConfig(def)
	return LoopConfig{
		IterationCap:     new(def.Contract.IterationCap),
		BudgetTokens:     new(int(def.Contract.Budget.Tokens)),
		BudgetWallSec:    new(int(def.Contract.Budget.WallClockSec)),
		BudgetOnExceeded: new(onExceeded),
		NoProgressWindow: new(def.Contract.NoProgress.Window),
		FanOutWidth:      new(definitionFanOutWidth(def)),
		GateMaxRevisions: new(definitionGateMaxRevisions(def)),
		RuntimeDefaults:  runtimeDefaults,
		RuntimeRules:     runtimeRules,
	}
}

func definitionFanOutWidth(def dsl.Definition) int {
	width := 0
	for _, node := range def.Graph.Nodes {
		if node.Class != dsl.NodeClassControl || node.Kind != string(dsl.ControlFanOut) {
			continue
		}
		width = max(width, node.MaxParallel)
	}
	return width
}

func definitionGateMaxRevisions(def dsl.Definition) int {
	revisions := 0
	for _, node := range def.Graph.Nodes {
		if node.Class != dsl.NodeClassControl || node.Kind != string(dsl.ControlGate) {
			continue
		}
		revisions = max(revisions, node.MaxRevisions)
	}
	return revisions
}

func mergeConfigLayer(effective *EffectiveConfig, layer LoopConfig) {
	layer = ClampLoopConfig(layer)
	if layer.HumanGateEnabled != nil {
		effective.HumanGateEnabled = *layer.HumanGateEnabled
	}
	if layer.ReattemptStrategy != nil {
		effective.ReattemptStrategy = *layer.ReattemptStrategy
	}
	if len(layer.EnabledChecks) > 0 {
		effective.EnabledChecks = cloneRawMessage(layer.EnabledChecks)
	}
	if layer.IterationCap != nil {
		effective.IterationCap = *layer.IterationCap
	}
	if layer.BudgetTokens != nil {
		effective.BudgetTokens = *layer.BudgetTokens
	}
	if layer.BudgetWallSec != nil {
		effective.BudgetWallSec = *layer.BudgetWallSec
	}
	if layer.BudgetOnExceeded != nil {
		effective.BudgetOnExceeded = *layer.BudgetOnExceeded
	}
	if layer.NoProgressWindow != nil {
		effective.NoProgressWindow = *layer.NoProgressWindow
	}
	if layer.FanOutWidth != nil {
		effective.FanOutWidth = *layer.FanOutWidth
	}
	if layer.GateMaxRevisions != nil {
		effective.GateMaxRevisions = *layer.GateMaxRevisions
	}
	if layer.Lifecycle != nil {
		mergeLifecycleLayer(&effective.Lifecycle, *layer.Lifecycle)
	}
	if layer.RequestExpireAfter != nil {
		effective.RequestExpireAfter = strings.TrimSpace(*layer.RequestExpireAfter)
	}
	if layer.Environment != nil {
		effective.Environment = normalizeEnvironmentSpec(*layer.Environment)
	}
	mergeRuntimeConfigLayer(effective, layer)
}

func validateEffectiveConfig(cfg EffectiveConfig) error {
	if cfg.ReattemptStrategy != ReattemptFailedOnly && cfg.ReattemptStrategy != ReattemptFullBody {
		return fmt.Errorf("%w: reattempt_strategy is invalid: %q", ErrValidation, cfg.ReattemptStrategy)
	}
	switch cfg.BudgetOnExceeded {
	case dsl.BudgetExceededHalt, dsl.BudgetExceededEscalate:
	default:
		return fmt.Errorf("%w: budget_on_exceeded is invalid: %q", ErrValidation, cfg.BudgetOnExceeded)
	}
	if len(cfg.EnabledChecks) == 0 {
		return fmt.Errorf("%w: enabled_checks_json is required", ErrValidation)
	}
	if !json.Valid(cfg.EnabledChecks) {
		return fmt.Errorf("%w: enabled_checks_json must be valid JSON", ErrValidation)
	}
	if _, err := ResolveNodeLifecycleConfig(dsl.Node{}, nil, cfg.Lifecycle); err != nil {
		return err
	}
	if _, err := ResolveActionEnvironment(dsl.EnvironmentSpec{}, cfg.Environment); err != nil {
		return err
	}
	if cfg.RequestExpireAfter != "" {
		duration, err := time.ParseDuration(cfg.RequestExpireAfter)
		if err != nil || duration <= 0 {
			return fmt.Errorf("%w: request_expire_after must be a positive duration", ErrValidation)
		}
	}
	return nil
}

func mergeLifecycleLayer(target *LifecycleConfig, layer LifecycleConfig) {
	mergeLifecycleInt(&target.RetryMaxAttempts, layer.RetryMaxAttempts)
	mergeLifecycleDuration(&target.RetryBackoffBase, layer.RetryBackoffBase)
	mergeLifecycleDuration(&target.RetryBackoffMax, layer.RetryBackoffMax)
	mergeLifecycleDuration(&target.LivenessSilenceWindow, layer.LivenessSilenceWindow)
	mergeLifecycleInt(&target.ResumeDeathStreakLimit, layer.ResumeDeathStreakLimit)
	if layer.PredicateCostLimit != nil {
		target.PredicateCostLimit = new(*layer.PredicateCostLimit)
	}
	mergeLifecycleInt(&target.WaitAdmissionAttempts, layer.WaitAdmissionAttempts)
	mergeLifecycleDuration(&target.WaitAdmissionInterval, layer.WaitAdmissionInterval)
	mergeLifecycleDuration(&target.AdmissionHorizon, layer.AdmissionHorizon)
	if len(layer.Autopause) > 0 {
		target.Autopause = append(target.Autopause, layer.Autopause...)
	}
}

func mergeLifecycleInt(target **int, value *int) {
	if value != nil {
		*target = new(*value)
	}
}

func mergeLifecycleDuration(target **time.Duration, value *time.Duration) {
	if value != nil {
		*target = new(*value)
	}
}

// DeriveDisplayCost derives a display-only cost value from token count and price.
func DeriveDisplayCost(tokens int64, pricePerTokenUSD float64) DisplayCost {
	if tokens < 0 {
		tokens = 0
	}
	if math.IsNaN(pricePerTokenUSD) || math.IsInf(pricePerTokenUSD, 0) || pricePerTokenUSD < 0 {
		pricePerTokenUSD = 0
	}
	return DisplayCost{
		Tokens:           tokens,
		PricePerTokenUSD: pricePerTokenUSD,
		USD:              float64(tokens) * pricePerTokenUSD,
	}
}

func clampNonNegative(value *int) {
	if value == nil {
		return
	}
	if *value < 0 {
		*value = 0
	}
}

func clampNonNegativeMax(value *int, ceiling int) {
	clampNonNegative(value)
	if value == nil {
		return
	}
	if *value > ceiling {
		*value = ceiling
	}
}

func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneAnyValue(value)
	}
	return cloned
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for idx, item := range typed {
			cloned[idx] = cloneAnyValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func normalizeLoopName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: loop name is required", ErrValidation)
	}
	clean, err := ValidateName(trimmed)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return clean, nil
}
