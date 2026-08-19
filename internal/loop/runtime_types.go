package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
)

// RuntimeSpec is the provider/model/reasoning intent resolved by the Loop engine.
type RuntimeSpec = dsl.RuntimeSpec

// RuntimeDefaults contains worker and judge default runtime intent.
type RuntimeDefaults = dsl.RuntimeDefaults

// RuntimeMatch selects one task identity dimension.
type RuntimeMatch = dsl.RuntimeMatch

// RuntimeRule is one task runtime selection rule.
type RuntimeRule = dsl.RuntimeRule

// RuntimeSource identifies the layer that supplied one resolved runtime field.
type RuntimeSource string

const (
	runtimeFieldProvider  = "provider"
	runtimeFieldModel     = "model"
	runtimeFieldReasoning = "reasoning"

	// RuntimeSourceRun identifies a per-run runtime rule.
	RuntimeSourceRun RuntimeSource = "run"
	// RuntimeSourceFrontmatter identifies task frontmatter.
	RuntimeSourceFrontmatter RuntimeSource = "frontmatter"
	// RuntimeSourceConfig identifies a configured runtime rule.
	RuntimeSourceConfig RuntimeSource = "config"
	// RuntimeSourceInput identifies a typed runtime input referenced by a node.
	RuntimeSourceInput RuntimeSource = "input"
	// RuntimeSourceNode identifies rendered node params.runtime.
	RuntimeSourceNode RuntimeSource = "node"
	// RuntimeSourceDefault identifies effective runtime_defaults.
	RuntimeSourceDefault RuntimeSource = "default"
	// RuntimeSourceCriterion identifies an agent-judge criterion runtime.
	RuntimeSourceCriterion RuntimeSource = "criterion"
	// RuntimeSourceAgent identifies a value completed by the agent definition.
	RuntimeSourceAgent RuntimeSource = "agent"
)

// RuntimeProvenance records the source of every resolved runtime field.
type RuntimeProvenance struct {
	Provider  RuntimeSource `json:"provider,omitempty"`
	Model     RuntimeSource `json:"model,omitempty"`
	Reasoning RuntimeSource `json:"reasoning,omitempty"`
}

// ResolvedRuntime is field-merged runtime intent plus per-field provenance.
type ResolvedRuntime struct {
	Runtime RuntimeSpec       `json:"runtime"`
	Source  RuntimeProvenance `json:"source"`
}

// RuntimeLayers are the ordered runtime-selection layers owned by the engine.
type RuntimeLayers struct {
	Defaults    RuntimeSpec
	ConfigRules []RuntimeRule
	RunRules    []RuntimeRule
}

// ItemRuntime carries task identity and authored runtime layers for one item.
type ItemRuntime struct {
	TaskID      string
	TaskType    string
	Complexity  string
	Frontmatter RuntimeSpec
	Node        RuntimeSpec
	Input       RuntimeSpec
}

// RuntimeValidationItem is one deterministic runtime validation diagnostic.
type RuntimeValidationItem struct {
	TaskID string `json:"task_id,omitempty"`
	Field  string `json:"field"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// RuntimeValidationError reports one or more invalid runtime fields.
type RuntimeValidationError struct {
	Items []RuntimeValidationItem `json:"runtime_validation"`
}

// Error implements error.
func (e *RuntimeValidationError) Error() string {
	if e == nil || len(e.Items) == 0 {
		return "loop: runtime validation failed"
	}
	item := e.Items[0]
	message := fmt.Sprintf(
		"loop: runtime validation failed for %s=%q: %s",
		item.Field,
		item.Value,
		item.Reason,
	)
	if item.Reason == "retired_key" || item.Reason == "unknown_field" {
		message += "; see MIGRATION_GUIDE.md#per-task-runtime-selection"
	}
	return message
}

// Unwrap keeps runtime validation in the loop validation error family.
func (e *RuntimeValidationError) Unwrap() error {
	return ErrValidation
}

// RuntimeCatalog validates provider identity and preserves exact model IDs for the provider boundary.
type RuntimeCatalog interface {
	CanonicalProvider(string) string
	ValidateRuntime(context.Context, RuntimeSpec) error
}

// WorkspaceRuntimeCatalog resolves workspace-specific provider validation authority.
type WorkspaceRuntimeCatalog interface {
	ForWorkspace(context.Context, WorkspaceID) (RuntimeCatalog, error)
}

// NewRuntimeValidationError constructs a normalized validation error.
func NewRuntimeValidationError(items ...RuntimeValidationItem) error {
	normalized := make([]RuntimeValidationItem, 0, len(items))
	for _, item := range items {
		item.TaskID = strings.TrimSpace(item.TaskID)
		item.Field = strings.TrimSpace(item.Field)
		item.Value = strings.TrimSpace(item.Value)
		item.Reason = strings.TrimSpace(item.Reason)
		if item.Field == "" || item.Reason == "" {
			continue
		}
		normalized = append(normalized, item)
	}
	if len(normalized) == 0 {
		return nil
	}
	return &RuntimeValidationError{Items: normalized}
}

// AsRuntimeValidationError extracts structured runtime diagnostics.
func AsRuntimeValidationError(err error) (*RuntimeValidationError, bool) {
	return errors.AsType[*RuntimeValidationError](err)
}
