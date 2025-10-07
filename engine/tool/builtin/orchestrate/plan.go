package orchestrate

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/invopop/jsonschema"
	"github.com/mitchellh/mapstructure"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

type Plan struct {
	ID          string         `json:"id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Bindings    map[string]any `json:"bindings,omitempty"`
	Steps       []Step         `json:"steps"`
}

type Step struct {
	ID          string          `json:"id"`
	Name        string          `json:"name,omitempty"`
	Description string          `json:"description,omitempty"`
	Type        StepType        `json:"type"`
	Status      StepStatus      `json:"status"`
	Agent       *AgentStep      `json:"agent,omitempty"`
	Parallel    *ParallelStep   `json:"parallel,omitempty"`
	Transitions StepTransitions `json:"transitions,omitempty"`
}

type AgentStep struct {
	AgentID   string         `json:"agent_id"`
	ActionID  string         `json:"action_id,omitempty"`
	Prompt    string         `json:"prompt,omitempty"`
	With      map[string]any `json:"with,omitempty"`
	ResultKey string         `json:"result_key,omitempty"`
	TimeoutMs int            `json:"timeout_ms,omitempty"`
}

type ParallelStep struct {
	Steps          []AgentStep    `json:"steps"`
	MaxConcurrency int            `json:"max_concurrency,omitempty"`
	MergeStrategy  MergeStrategy  `json:"merge_strategy,omitempty"`
	ResultKey      string         `json:"result_key,omitempty"`
	Bindings       map[string]any `json:"bindings,omitempty"`
}

type StepTransitions struct {
	AllowedEvents    []StepEvent `json:"allowed_events,omitempty"`
	FailureBranchIDs []string    `json:"failure_branch_ids,omitempty"`
	DefaultNext      string      `json:"default_next_step,omitempty"`
}

type StepType string

const (
	StepTypeAgent    StepType = "agent"
	StepTypeParallel StepType = "parallel"
)

var stepTypes = map[StepType]struct{}{
	StepTypeAgent:    {},
	StepTypeParallel: {},
}

type StepStatus string

const (
	StepStatusPending StepStatus = "pending"
	StepStatusRunning StepStatus = "running"
	StepStatusSuccess StepStatus = "success"
	StepStatusFailed  StepStatus = "failed"
	StepStatusPartial StepStatus = "partial"
	StepStatusSkipped StepStatus = "skipped"
)

var stepStatuses = map[StepStatus]struct{}{
	StepStatusPending: {},
	StepStatusRunning: {},
	StepStatusSuccess: {},
	StepStatusFailed:  {},
	StepStatusPartial: {},
	StepStatusSkipped: {},
}

type StepEvent string

const (
	StepEventStartPlan       StepEvent = "start_plan"
	StepEventPlannerFinished StepEvent = "planner_finished"
	StepEventValidationFail  StepEvent = "validation_failed"
	StepEventDispatchStep    StepEvent = "dispatch_step"
	StepEventStepSuccess     StepEvent = "step_succeeded"
	StepEventStepFailed      StepEvent = "step_failed"
	StepEventParallelDone    StepEvent = "parallel_complete"
	StepEventTimeout         StepEvent = "timeout"
	StepEventPanic           StepEvent = "panic"
)

var stepEvents = map[StepEvent]struct{}{
	StepEventStartPlan:       {},
	StepEventPlannerFinished: {},
	StepEventValidationFail:  {},
	StepEventDispatchStep:    {},
	StepEventStepSuccess:     {},
	StepEventStepFailed:      {},
	StepEventParallelDone:    {},
	StepEventTimeout:         {},
	StepEventPanic:           {},
}

type MergeStrategy string

const (
	MergeStrategyCollect      MergeStrategy = "collect"
	MergeStrategyFirstSuccess MergeStrategy = "first_success"
)

var mergeStrategies = map[MergeStrategy]struct{}{
	MergeStrategyCollect:      {},
	MergeStrategyFirstSuccess: {},
}

var (
	resultKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
	stepIDPattern    = regexp.MustCompile(`^[a-zA-Z0-9_.:-]+$`)
)

func (Plan) JSONSchemaExtend(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	sc.AdditionalProperties = jsonschema.TrueSchema
	setPropertyPattern(sc, "id", stepIDPattern.String())
	setPropertyMinItems(sc, "steps", 1)
	setMapAdditionalProperties(sc, "bindings")
	sc.Required = appendUnique(sc.Required, "steps")
}

func (Step) JSONSchemaExtend(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	setPropertyPattern(sc, "id", stepIDPattern.String())
	sc.Required = appendUnique(sc.Required, "id", "type", "status")
	sc.AllOf = append(sc.AllOf,
		&jsonschema.Schema{
			If:   schemaConstEq("type", string(StepTypeAgent)),
			Then: schemaRequireWithout("agent", "parallel"),
		},
		&jsonschema.Schema{
			If:   schemaConstEq("type", string(StepTypeParallel)),
			Then: schemaRequireWithout("parallel", "agent"),
		},
	)
}

func (AgentStep) JSONSchemaExtend(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	setMapAdditionalProperties(sc, "with")
	setPropertyPattern(sc, "result_key", resultKeyPattern.String())
	sc.Required = appendUnique(sc.Required, "agent_id")
}

func (ParallelStep) JSONSchemaExtend(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	setPropertyMinItems(sc, "steps", 1)
	setMapAdditionalProperties(sc, "bindings")
	setPropertyPattern(sc, "result_key", resultKeyPattern.String())
	sc.Required = appendUnique(sc.Required, "steps")
}

func (StepTransitions) JSONSchemaExtend(sc *jsonschema.Schema) {
	if sc == nil {
		return
	}
	if prop, ok := schemaProperty(sc, "allowed_events"); ok {
		prop.UniqueItems = true
	}
	setPropertyPattern(sc, "default_next_step", stepIDPattern.String())
	if prop, ok := schemaProperty(sc, "failure_branch_ids"); ok {
		prop.Items = &jsonschema.Schema{Type: "string", Pattern: stepIDPattern.String(), MinLength: uint64Ptr(1)}
	}
}

func (StepType) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{string(StepTypeAgent), string(StepTypeParallel)},
	}
}

func (StepStatus) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(StepStatusPending),
			string(StepStatusRunning),
			string(StepStatusSuccess),
			string(StepStatusFailed),
			string(StepStatusPartial),
			string(StepStatusSkipped),
		},
	}
}

func (MergeStrategy) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{string(MergeStrategyCollect), string(MergeStrategyFirstSuccess)},
	}
}

func (StepEvent) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		Enum: []any{
			string(StepEventStartPlan),
			string(StepEventPlannerFinished),
			string(StepEventValidationFail),
			string(StepEventDispatchStep),
			string(StepEventStepSuccess),
			string(StepEventStepFailed),
			string(StepEventParallelDone),
			string(StepEventTimeout),
			string(StepEventPanic),
		},
	}
}

func (p *Plan) Validate() error {
	if p == nil {
		return errors.New("plan is nil")
	}
	var errs []error
	if len(p.Steps) == 0 {
		errs = append(errs, errors.New("plan requires at least one step"))
	}
	stepPaths := map[string]string{}
	resultOwners := map[string]string{}
	for idx := range p.Steps {
		step := &p.Steps[idx]
		path := fmt.Sprintf("steps[%d]", idx)
		if err := step.validate(path); err != nil {
			errs = append(errs, err)
		}
		if step.ID != "" {
			if prev, ok := stepPaths[step.ID]; ok {
				errs = append(errs, fmt.Errorf("%s.id duplicates %s.id", path, prev))
			} else {
				stepPaths[step.ID] = path
			}
		}
		for _, key := range step.resultKeys() {
			if key == "" {
				continue
			}
			if prev, ok := resultOwners[key]; ok {
				errs = append(errs, fmt.Errorf("%s uses result_key %q already used by %s", path, key, prev))
			} else {
				resultOwners[key] = path
			}
		}
	}
	for idx := range p.Steps {
		step := &p.Steps[idx]
		path := fmt.Sprintf("steps[%d]", idx)
		if err := step.validateTransitions(path, stepPaths); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *Step) validate(path string) error {
	var errs []error
	if s.ID == "" {
		errs = append(errs, fmt.Errorf("%s.id is required", path))
	} else if !stepIDPattern.MatchString(s.ID) {
		errs = append(errs, fmt.Errorf("%s.id must match %s", path, stepIDPattern.String()))
	}
	if s.Type == "" {
		errs = append(errs, fmt.Errorf("%s.type is required", path))
	} else if _, ok := stepTypes[s.Type]; !ok {
		errs = append(errs, fmt.Errorf("%s.type %q is invalid", path, s.Type))
	}
	if s.Status == "" {
		errs = append(errs, fmt.Errorf("%s.status is required", path))
	} else if _, ok := stepStatuses[s.Status]; !ok {
		errs = append(errs, fmt.Errorf("%s.status %q is invalid", path, s.Status))
	}
	if err := s.validateTypeSpecific(path); err != nil {
		errs = append(errs, err)
	}
	if err := s.Transitions.validate(path + ".transitions"); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *Step) validateTypeSpecific(path string) error {
	switch s.Type {
	case StepTypeAgent:
		return s.validateAgentStep(path)
	case StepTypeParallel:
		return s.validateParallelStep(path)
	default:
		return nil
	}
}

func (s *Step) validateAgentStep(path string) error {
	var errs []error
	if s.Agent == nil {
		errs = append(errs, fmt.Errorf("%s.agent is required for agent steps", path))
	} else if err := s.Agent.validate(path + ".agent"); err != nil {
		errs = append(errs, err)
	}
	if s.Parallel != nil {
		errs = append(errs, fmt.Errorf("%s.parallel must be omitted for agent steps", path))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *Step) validateParallelStep(path string) error {
	var errs []error
	if s.Parallel == nil {
		errs = append(errs, fmt.Errorf("%s.parallel is required for parallel steps", path))
	} else if err := s.Parallel.validate(path + ".parallel"); err != nil {
		errs = append(errs, err)
	}
	if s.Agent != nil {
		errs = append(errs, fmt.Errorf("%s.agent must be omitted for parallel steps", path))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *Step) validateTransitions(path string, stepIndex map[string]string) error {
	var errs []error
	if s.Transitions.DefaultNext != "" {
		if s.Transitions.DefaultNext == s.ID {
			errs = append(
				errs,
				fmt.Errorf("%s.default_next_step must not reference the current step", path),
			)
		}
		if _, ok := stepIndex[s.Transitions.DefaultNext]; !ok {
			errs = append(
				errs,
				fmt.Errorf("%s.default_next_step references unknown step %q", path, s.Transitions.DefaultNext),
			)
		}
	}
	for idx, ref := range s.Transitions.FailureBranchIDs {
		subPath := fmt.Sprintf("%s.failure_branch_ids[%d]", path, idx)
		if _, ok := stepIndex[ref]; !ok {
			errs = append(errs, fmt.Errorf("%s references unknown step %q", subPath, ref))
		}
		if ref == s.ID {
			errs = append(errs, fmt.Errorf("%s must not reference the current step", subPath))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *Step) resultKeys() []string {
	if s == nil {
		return nil
	}
	switch s.Type {
	case StepTypeAgent:
		if s.Agent != nil && s.Agent.ResultKey != "" {
			return []string{s.Agent.ResultKey}
		}
	case StepTypeParallel:
		if s.Parallel == nil {
			return nil
		}
		keys := make([]string, 0, len(s.Parallel.Steps)+1)
		if s.Parallel.ResultKey != "" {
			keys = append(keys, s.Parallel.ResultKey)
		}
		for idx := range s.Parallel.Steps {
			if s.Parallel.Steps[idx].ResultKey != "" {
				keys = append(keys, s.Parallel.Steps[idx].ResultKey)
			}
		}
		return keys
	}
	return nil
}

func (a *AgentStep) validate(path string) error {
	var errs []error
	if a.AgentID == "" {
		errs = append(errs, fmt.Errorf("%s.agent_id is required", path))
	}
	if a.ResultKey != "" && !resultKeyPattern.MatchString(a.ResultKey) {
		errs = append(errs, fmt.Errorf("%s.result_key must match %s", path, resultKeyPattern.String()))
	}
	if a.TimeoutMs < 0 {
		errs = append(errs, fmt.Errorf("%s.timeout_ms must be >= 0", path))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (p *ParallelStep) validate(path string) error {
	var errs []error
	if len(p.Steps) == 0 {
		errs = append(errs, fmt.Errorf("%s.steps must contain at least one agent step", path))
	}
	if p.ResultKey != "" && !resultKeyPattern.MatchString(p.ResultKey) {
		errs = append(errs, fmt.Errorf("%s.result_key must match %s", path, resultKeyPattern.String()))
	}
	if p.MaxConcurrency < 0 {
		errs = append(errs, fmt.Errorf("%s.max_concurrency must be non-negative", path))
	}
	if p.MergeStrategy != "" {
		if _, ok := mergeStrategies[p.MergeStrategy]; !ok {
			errs = append(errs, fmt.Errorf("%s.merge_strategy %q is invalid", path, p.MergeStrategy))
		}
	}
	for idx := range p.Steps {
		if err := p.Steps[idx].validate(fmt.Sprintf("%s.steps[%d]", path, idx)); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (t StepTransitions) validate(path string) error {
	var errs []error
	eventSeen := map[StepEvent]struct{}{}
	for idx, evt := range t.AllowedEvents {
		subPath := fmt.Sprintf("%s.allowed_events[%d]", path, idx)
		if _, ok := stepEvents[evt]; !ok {
			errs = append(errs, fmt.Errorf("%s %q is invalid", subPath, evt))
			continue
		}
		if _, ok := eventSeen[evt]; ok {
			errs = append(errs, fmt.Errorf("%s duplicates event %q", subPath, evt))
		} else {
			eventSeen[evt] = struct{}{}
		}
	}
	branchSeen := map[string]struct{}{}
	for idx, ref := range t.FailureBranchIDs {
		subPath := fmt.Sprintf("%s.failure_branch_ids[%d]", path, idx)
		if ref == "" {
			errs = append(errs, fmt.Errorf("%s must not be empty", subPath))
			continue
		}
		if _, ok := branchSeen[ref]; ok {
			errs = append(errs, fmt.Errorf("%s duplicates reference %q", subPath, ref))
		} else {
			branchSeen[ref] = struct{}{}
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func DecodePlanMap(payload map[string]any) (Plan, error) {
	var plan Plan
	if payload == nil {
		return plan, errors.New("plan payload is nil")
	}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &plan,
	})
	if err != nil {
		return plan, fmt.Errorf("failed to build plan decoder: %w", err)
	}
	if err := decoder.Decode(payload); err != nil {
		return plan, fmt.Errorf("failed to decode plan payload: %w", err)
	}
	if err := plan.Validate(); err != nil {
		return plan, err
	}
	return plan, nil
}

func schemaProperty(sc *jsonschema.Schema, name string) (*jsonschema.Schema, bool) {
	if sc == nil || sc.Properties == nil {
		return nil, false
	}
	if value, ok := sc.Properties.Get(name); ok {
		return value, true
	}
	return nil, false
}

func setPropertyPattern(sc *jsonschema.Schema, name, pattern string) {
	if prop, ok := schemaProperty(sc, name); ok {
		prop.Pattern = pattern
	}
}

func setPropertyMinItems(sc *jsonschema.Schema, name string, minItems uint64) {
	if prop, ok := schemaProperty(sc, name); ok {
		prop.MinItems = uint64Ptr(minItems)
	}
}

func setMapAdditionalProperties(sc *jsonschema.Schema, name string) {
	if prop, ok := schemaProperty(sc, name); ok {
		prop.AdditionalProperties = jsonschema.TrueSchema
	}
}

func schemaConstEq(field string, value any) *jsonschema.Schema {
	obj := &jsonschema.Schema{Type: "object"}
	obj.Properties = orderedmap.New[string, *jsonschema.Schema]()
	obj.Properties.Set(field, &jsonschema.Schema{Const: value})
	return obj
}

func schemaRequireWithout(required, forbidden string) *jsonschema.Schema {
	result := &jsonschema.Schema{Required: []string{required}}
	if forbidden != "" {
		result.Not = &jsonschema.Schema{Required: []string{forbidden}}
	}
	return result
}

func appendUnique(list []string, values ...string) []string {
	seen := make(map[string]struct{}, len(list))
	for _, item := range list {
		seen[item] = struct{}{}
	}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		list = append(list, value)
	}
	return list
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}
