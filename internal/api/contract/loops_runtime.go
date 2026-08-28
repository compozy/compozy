package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// LoopRuntimeValidationItemPayload is one deterministic runtime preflight diagnostic.
type LoopRuntimeValidationItemPayload struct {
	TaskID string `json:"task_id,omitempty"`
	Field  string `json:"field"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// LoopRuntimeSpec is one provider/model/reasoning/speed/ACP-option selection layer.
type LoopRuntimeSpec struct {
	Provider   string                    `json:"provider,omitempty"`
	Model      string                    `json:"model,omitempty"`
	Reasoning  string                    `json:"reasoning,omitempty"`
	Speed      Speed                     `json:"speed,omitempty"`
	ACPOptions []AgentACPOptionSelection `json:"acp_options,omitempty"`
}

// UnmarshalJSON keeps runtime objects closed while the surrounding Loop DSL remains extensible.
func (s *LoopRuntimeSpec) UnmarshalJSON(data []byte) error {
	type runtimeSpecAlias LoopRuntimeSpec
	var decoded runtimeSpecAlias
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return fmt.Errorf(
			"decode runtime: %w; see MIGRATION_GUIDE.md#per-task-runtime-selection",
			err,
		)
	}
	*s = LoopRuntimeSpec(decoded)
	return nil
}

// LoopRuntimeDefaults contains worker and judge runtime intent.
type LoopRuntimeDefaults struct {
	//nolint:modernize // OpenAPI requires omitempty; JSON uses omitzero.
	Worker LoopRuntimeSpec `json:"worker,omitempty,omitzero"`
	//nolint:modernize // OpenAPI requires omitempty; JSON uses omitzero.
	Judge LoopRuntimeSpec `json:"judge,omitempty,omitzero"`
}

// LoopRuntimeMatch selects exactly one task id, one task type, one complexity,
// or a type and complexity conjunction. The conjunction is AND and id is exclusive.
type LoopRuntimeMatch struct {
	ID         string `json:"id,omitempty"`
	Type       string `json:"type,omitempty"`
	Complexity string `json:"complexity,omitempty"`
}

// LoopRuntimeRule applies field-merged runtime intent with specificity
// id > type+complexity > type > complexity; later equal-specificity rules win per field.
type LoopRuntimeRule struct {
	Match   LoopRuntimeMatch `json:"match"`
	Runtime LoopRuntimeSpec  `json:"runtime"`
}

// LoopRuntimeProvenance identifies the source of each applied field.
type LoopRuntimeProvenance struct {
	Provider   string            `json:"provider,omitempty"`
	Model      string            `json:"model,omitempty"`
	Reasoning  string            `json:"reasoning,omitempty"`
	Speed      string            `json:"speed,omitempty"`
	ACPOptions map[string]string `json:"acp_options,omitempty"`
}

// LoopResolvedRuntime is the runtime applied by the daemon plus field provenance.
type LoopResolvedRuntime struct {
	Provider        string                    `json:"provider,omitempty"`
	Model           string                    `json:"model,omitempty"`
	Reasoning       string                    `json:"reasoning,omitempty"`
	Speed           Speed                     `json:"speed,omitempty"`
	ACPOptions      []AgentACPOptionSelection `json:"acp_options,omitempty"`
	SpeedResolution *SpeedResolution          `json:"speed_resolution,omitempty"`
	Source          LoopRuntimeProvenance     `json:"source"`
}
