package model

import "strings"

// TaskRuntimeRule defines runtime overrides that apply to one task selector.
// Exactly one selector should be set in validated inputs.
type TaskRuntimeRule struct {
	ID              *string `toml:"id"               json:"id,omitempty"`
	Type            *string `toml:"type"             json:"type,omitempty"`
	IDE             *string `toml:"ide"              json:"ide,omitempty"`
	Model           *string `toml:"model"            json:"model,omitempty"`
	ReasoningEffort *string `toml:"reasoning_effort" json:"reasoning_effort,omitempty"`
}

// TaskRuntimeTarget identifies the task being resolved against runtime rules.
type TaskRuntimeTarget struct {
	ID   string
	Type string
}

// TaskRuntime describes the effective runtime fields that may vary per task.
type TaskRuntime struct {
	IDE             string `json:"ide,omitempty"`
	Model           string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
}

// TaskRuntimeTask identifies the PRD task whose runtime is being resolved.
type TaskRuntimeTask struct {
	ID       string `json:"id,omitempty"`
	SafeName string `json:"safe_name,omitempty"`
	Title    string `json:"title,omitempty"`
	Type     string `json:"type,omitempty"`
}

// CloneTaskRuntimeRules returns a deep copy of runtime rules so callers can
// merge or mutate execution-local copies safely.
func CloneTaskRuntimeRules(src []TaskRuntimeRule) []TaskRuntimeRule {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]TaskRuntimeRule, 0, len(src))
	for _, rule := range src {
		cloned = append(cloned, rule.clone())
	}
	return cloned
}

func (r TaskRuntimeRule) clone() TaskRuntimeRule {
	return TaskRuntimeRule{
		ID:              cloneOptionalString(r.ID),
		Type:            cloneOptionalString(r.Type),
		IDE:             cloneOptionalString(r.IDE),
		Model:           cloneOptionalString(r.Model),
		ReasoningEffort: cloneOptionalString(r.ReasoningEffort),
	}
}

func (r TaskRuntimeRule) HasSelector() bool {
	return r.ID != nil || r.Type != nil
}

func (r TaskRuntimeRule) HasOverride() bool {
	return r.IDE != nil || r.Model != nil || r.ReasoningEffort != nil
}

func (r TaskRuntimeRule) IsIDRule() bool {
	return r.ID != nil
}

func (r TaskRuntimeRule) IsTypeRule() bool {
	return r.Type != nil
}

func (r TaskRuntimeRule) Matches(target TaskRuntimeTarget) bool {
	switch {
	case r.ID != nil:
		return strings.TrimSpace(target.ID) != "" && strings.TrimSpace(*r.ID) == strings.TrimSpace(target.ID)
	case r.Type != nil:
		return strings.TrimSpace(target.Type) != "" && strings.TrimSpace(*r.Type) == strings.TrimSpace(target.Type)
	default:
		return false
	}
}

// Clone returns a deep copy of cfg so callers can resolve task-local runtime
// overrides without mutating the shared base run configuration.
func (cfg *RuntimeConfig) Clone() *RuntimeConfig {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	cloned.AddDirs = append([]string(nil), cfg.AddDirs...)
	cloned.TaskRuntimeRules = CloneTaskRuntimeRules(cfg.TaskRuntimeRules)
	return &cloned
}

// RuntimeForTask resolves the effective runtime for one task. Type selectors
// apply before id selectors, and later rules win within the same specificity.
func (cfg *RuntimeConfig) RuntimeForTask(target TaskRuntimeTarget) *RuntimeConfig {
	cloned := cfg.Clone()
	if cloned == nil {
		return nil
	}

	for _, rule := range cloned.TaskRuntimeRules {
		if !rule.IsTypeRule() || !rule.Matches(target) {
			continue
		}
		applyTaskRuntimeRule(cloned, rule)
	}
	for _, rule := range cloned.TaskRuntimeRules {
		if !rule.IsIDRule() || !rule.Matches(target) {
			continue
		}
		applyTaskRuntimeRule(cloned, rule)
	}
	cloned.TaskRuntimeRules = nil
	return cloned
}

func applyTaskRuntimeRule(cfg *RuntimeConfig, rule TaskRuntimeRule) {
	if cfg == nil {
		return
	}
	if rule.IDE != nil {
		cfg.IDE = strings.TrimSpace(*rule.IDE)
	}
	if rule.Model != nil {
		cfg.Model = strings.TrimSpace(*rule.Model)
	}
	if rule.ReasoningEffort != nil {
		cfg.ReasoningEffort = strings.TrimSpace(*rule.ReasoningEffort)
	}
}

// TaskRuntimeFromConfig returns the task-scoped runtime view for cfg.
func TaskRuntimeFromConfig(cfg *RuntimeConfig) TaskRuntime {
	if cfg == nil {
		return TaskRuntime{}
	}
	return TaskRuntime{
		IDE:             strings.TrimSpace(cfg.IDE),
		Model:           strings.TrimSpace(cfg.Model),
		ReasoningEffort: strings.TrimSpace(cfg.ReasoningEffort),
	}
}

// ApplyTaskRuntime copies task-scoped runtime fields onto cfg.
func ApplyTaskRuntime(cfg *RuntimeConfig, runtime TaskRuntime) {
	if cfg == nil {
		return
	}
	cfg.IDE = strings.TrimSpace(runtime.IDE)
	cfg.Model = strings.TrimSpace(runtime.Model)
	cfg.ReasoningEffort = strings.TrimSpace(runtime.ReasoningEffort)
}

func cloneOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := strings.TrimSpace(*value)
	return &cloned
}
