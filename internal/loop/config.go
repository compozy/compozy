package loop

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

var emptyChecksJSON = json.RawMessage(`{}`)

// DefaultLoopDefaults returns the built-in fallback `[loops.defaults.*]` layer.
func DefaultLoopDefaults() LoopDefaults {
	halt := dsl.BudgetExceededHalt
	return LoopDefaults{
		Delivery: LoopConfig{
			IterationCap:     new(50),
			NoProgressWindow: new(3),
			GateMaxRevisions: new(10),
			BudgetTokens:     new(0),
			BudgetWallSec:    new(0),
			BudgetOnExceeded: new(halt),
			FanOutWidth:      new(4),
		},
		Watch: LoopConfig{
			IterationCap:     new(0),
			NoProgressWindow: new(2),
			BudgetTokens:     new(0),
			BudgetWallSec:    new(0),
			BudgetOnExceeded: new(halt),
			FanOutWidth:      new(2),
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
	clampNonNegativeMax(clamped.FanOutWidth, LoopMaxFanoutWidth)
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
	if cfg.ModelDefaults != nil {
		cloned.ModelDefaults = cfg.ModelDefaults.Clone()
	}
	return cloned
}

// Clone returns a deep copy of one raw model-default override layer.
func (cfg ModelDefaults) Clone() *ModelDefaults {
	cloned := ModelDefaults{}
	if cfg.Worker != nil {
		cloned.Worker = new(*cfg.Worker)
	}
	if cfg.Judge != nil {
		cloned.Judge = new(*cfg.Judge)
	}
	return &cloned
}

// ResolveEffectiveConfig merges definition, defaults, loop_config, and per-run layers.
func ResolveEffectiveConfig(
	resolved *ResolvedDefinition,
	defaults LoopDefaults,
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
	}
	mergeConfigLayer(&effective, definitionConfigLayer(resolved.Definition))
	mergeConfigLayer(&effective, defaults.forDefinition(resolved.Definition))
	if stored != nil {
		mergeConfigLayer(&effective, *stored)
	}
	mergeConfigLayer(&effective, perRun)
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
	modelWorker := ""
	modelJudge := ""
	if def.Contract.ModelDefaults != nil {
		modelWorker = def.Contract.ModelDefaults.Worker
		modelJudge = def.Contract.ModelDefaults.Judge
	}
	return LoopConfig{
		IterationCap:     new(def.Contract.IterationCap),
		BudgetTokens:     new(def.Contract.Budget.Tokens),
		BudgetWallSec:    new(def.Contract.Budget.WallClockSec),
		BudgetOnExceeded: new(onExceeded),
		NoProgressWindow: new(def.Contract.NoProgress.Window),
		FanOutWidth:      new(definitionFanOutWidth(def)),
		GateMaxRevisions: new(definitionGateMaxRevisions(def)),
		ModelDefaults:    modelDefaultsLayer(modelWorker, modelJudge),
	}
}

func modelDefaultsLayer(worker string, judge string) *ModelDefaults {
	worker = strings.TrimSpace(worker)
	judge = strings.TrimSpace(judge)
	if worker == "" && judge == "" {
		return nil
	}
	defaults := ModelDefaults{}
	if worker != "" {
		defaults.Worker = new(worker)
	}
	if judge != "" {
		defaults.Judge = new(judge)
	}
	return &defaults
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
	if layer.ModelDefaults != nil {
		if layer.ModelDefaults.Worker != nil {
			effective.ModelDefaults.Worker = strings.TrimSpace(*layer.ModelDefaults.Worker)
		}
		if layer.ModelDefaults.Judge != nil {
			effective.ModelDefaults.Judge = strings.TrimSpace(*layer.ModelDefaults.Judge)
		}
	}
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
	return nil
}

// ResolveInputs applies declared input defaults and validates required values.
func ResolveInputs(def dsl.Definition, inputs Inputs) (map[string]any, error) {
	resolved := cloneMap(inputs.Values)
	for name, input := range def.Inputs {
		if _, ok := resolved[name]; ok {
			continue
		}
		if input.Default != nil {
			resolved[name] = cloneAnyValue(input.Default)
			continue
		}
		if input.Required {
			return nil, fmt.Errorf("%w: input %q is required", ErrValidation, name)
		}
	}
	for name, value := range resolved {
		input, declared := def.Inputs[name]
		if !declared {
			continue
		}
		if err := validateInputType(name, input.Type, value); err != nil {
			return nil, err
		}
	}
	return resolved, nil
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

func validateInputType(name string, inputType dsl.InputType, value any) error {
	switch inputType {
	case dsl.InputTypeString, dsl.InputTypeFile, dsl.InputTypeAgent, dsl.InputTypeRef:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%w: input %q must be a string", ErrValidation, name)
		}
	case dsl.InputTypeNumber:
		switch value.(type) {
		case int, int64, float64, float32, json.Number:
		default:
			return fmt.Errorf("%w: input %q must be a number", ErrValidation, name)
		}
	case dsl.InputTypeBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%w: input %q must be a boolean", ErrValidation, name)
		}
	case "":
		return fmt.Errorf("%w: input %q type is required", ErrValidation, name)
	default:
		return fmt.Errorf("%w: input %q type is invalid: %q", ErrValidation, name, inputType)
	}
	return nil
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
