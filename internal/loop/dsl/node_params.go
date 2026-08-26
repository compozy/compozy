package dsl

// ResponderAgentPolicy is the closed agent-response policy for human requests.
type ResponderAgentPolicy string

const (
	// ResponderAgentsDeny keeps requests human-only, the default.
	ResponderAgentsDeny ResponderAgentPolicy = "deny"
	// ResponderAgentsAllow opts capable agents into responding.
	ResponderAgentsAllow ResponderAgentPolicy = "allow"
)

// Valid reports whether the policy belongs to the closed responder enum.
func (p ResponderAgentPolicy) Valid() bool {
	return p == "" || p == ResponderAgentsDeny || p == ResponderAgentsAllow
}

// ResponderSpec controls which non-human principals may answer a request.
type ResponderSpec struct {
	Agents ResponderAgentPolicy `json:"agents,omitempty" yaml:"agents,omitempty"`
}

// AskParams is the canonical schema for control kind ask.
type AskParams struct {
	Prompt     string         `json:"prompt"               yaml:"prompt"`
	Context    map[string]any `json:"context,omitempty"    yaml:"context,omitempty"`
	Expect     Schema         `json:"expect"               yaml:"expect"`
	Responders *ResponderSpec `json:"responders,omitempty" yaml:"responders,omitempty"`
	Expires    *WaitExpiry    `json:"expires,omitempty"    yaml:"expires,omitempty"`
	Extra      map[string]any `json:"-"                    yaml:",inline"`
}

// ReviewDecision is one allowed pre-execution review outcome.
type ReviewDecision string

const (
	ReviewDecisionApprove ReviewDecision = "approve"
	ReviewDecisionEdit    ReviewDecision = "edit"
	ReviewDecisionReject  ReviewDecision = "reject"
	ReviewDecisionRespond ReviewDecision = "respond"
)

// Valid reports whether the decision belongs to the closed review enum.
func (d ReviewDecision) Valid() bool {
	switch d {
	case ReviewDecisionApprove, ReviewDecisionEdit, ReviewDecisionReject, ReviewDecisionRespond:
		return true
	default:
		return false
	}
}

// RejectPolicy selects the forward route taken by a rejected review.
type RejectPolicy struct {
	Route NodeID         `json:"route" yaml:"route"`
	Extra map[string]any `json:"-"     yaml:",inline"`
}

// ReviewSpec gates an action before its task run is created.
type ReviewSpec struct {
	Decisions  []ReviewDecision `json:"decisions,omitempty"  yaml:"decisions,omitempty"`
	When       string           `json:"when,omitempty"       yaml:"when,omitempty"`
	Prompt     string           `json:"prompt,omitempty"     yaml:"prompt,omitempty"`
	Responders *ResponderSpec   `json:"responders,omitempty" yaml:"responders,omitempty"`
	OnReject   *RejectPolicy    `json:"on_reject,omitempty"  yaml:"on_reject,omitempty"`
	Expires    *WaitExpiry      `json:"expires,omitempty"    yaml:"expires,omitempty"`
	Extra      map[string]any   `json:"-"                    yaml:",inline"`
}

// EffectiveDecisions returns the authored allowlist or the safe default.
func (r ReviewSpec) EffectiveDecisions() []ReviewDecision {
	if len(r.Decisions) == 0 {
		return []ReviewDecision{ReviewDecisionApprove, ReviewDecisionReject}
	}
	return append([]ReviewDecision(nil), r.Decisions...)
}

// RunAgentParams is the canonical schema for action kind run-agent.
type RunAgentParams struct {
	Agent        string          `json:"agent"                   yaml:"agent"`
	Prompt       string          `json:"prompt"                  yaml:"prompt"`
	OutputSchema Schema          `json:"output_schema,omitempty" yaml:"output_schema,omitempty"`
	Environment  EnvironmentSpec `json:"environment,omitzero"    yaml:"environment,omitempty"`
	Runtime      RuntimeSpec     `json:"runtime,omitzero"        yaml:"runtime,omitempty"`
	AllowedTools []string        `json:"allowed_tools,omitempty" yaml:"allowed_tools,omitempty"`
	MaxTurns     int             `json:"max_turns,omitempty"     yaml:"max_turns,omitempty"`
	Extra        map[string]any  `json:"-"                       yaml:",inline"`
}

// GoalParams is the canonical schema for action kind goal.
type GoalParams struct {
	Agent        string          `json:"agent"                   yaml:"agent"`
	Objective    string          `json:"objective"               yaml:"objective"`
	Judge        []GateCriterion `json:"judge"                   yaml:"judge"`
	MaxTurns     int             `json:"max_turns"               yaml:"max_turns"`
	OnExhausted  string          `json:"on_exhausted,omitempty"  yaml:"on_exhausted,omitempty"`
	OutputSchema *Schema         `json:"output_schema,omitempty" yaml:"output_schema,omitempty"`
	Environment  EnvironmentSpec `json:"environment,omitzero"    yaml:"environment,omitempty"`
	Runtime      RuntimeSpec     `json:"runtime,omitzero"        yaml:"runtime,omitempty"`
	Extra        map[string]any  `json:"-"                       yaml:",inline"`
}

// EnvironmentMode selects the execution root for one agent action.
type EnvironmentMode string

const (
	// EnvironmentRoot runs from the parent workspace root.
	EnvironmentRoot EnvironmentMode = "root"
	// EnvironmentWorktree runs from one existing ready worktree.
	EnvironmentWorktree EnvironmentMode = "worktree"
	// EnvironmentPerRun creates one worktree for the execution instance.
	EnvironmentPerRun EnvironmentMode = "per_run"
	// EnvironmentDirectory runs from a contained directory under the selected root.
	EnvironmentDirectory EnvironmentMode = "directory"
)

// EnvironmentSpec describes the authored execution root for an agent action.
type EnvironmentSpec struct {
	Mode        EnvironmentMode `json:"mode"                   yaml:"mode"`
	WorktreeRef string          `json:"worktree_ref,omitempty" yaml:"worktree_ref,omitempty"`
	Directory   string          `json:"directory,omitempty"    yaml:"directory,omitempty"`
}

// RunLoopParams is the canonical schema for action kind run-loop.
type RunLoopParams struct {
	Loop            string         `json:"loop"                       yaml:"loop"`
	Inputs          map[string]any `json:"inputs,omitempty"           yaml:"inputs,omitempty"`
	Mode            RunLoopMode    `json:"mode,omitempty"             yaml:"mode,omitempty"`
	ConfigOverrides map[string]any `json:"config_overrides,omitempty" yaml:"config_overrides,omitempty"`
	Extra           map[string]any `json:"-"                          yaml:",inline"`
}

// RunLoopMode controls child-loop completion semantics.
type RunLoopMode string

const (
	// RunLoopAwait waits for a child loop terminal event.
	RunLoopAwait RunLoopMode = "await"
	// RunLoopDetach returns immediately with the child run id.
	RunLoopDetach RunLoopMode = "detach"
)

// TransformParams is the canonical schema for action kind transform.
type TransformParams struct {
	Map   map[string]TransformMapping `json:"map" yaml:"map"`
	Extra map[string]any              `json:"-"   yaml:",inline"`
}

// TransformMapping maps one output key from a namespace path, literal, or template.
type TransformMapping struct {
	From     string         `json:"from,omitempty"     yaml:"from,omitempty"`
	Value    any            `json:"value,omitempty"    yaml:"value,omitempty"`
	Template string         `json:"template,omitempty" yaml:"template,omitempty"`
	Extra    map[string]any `json:"-"                  yaml:",inline"`
}

// SessionSpec configures action session binding.
type SessionSpec struct {
	Handle   string         `json:"handle,omitempty"   yaml:"handle,omitempty"`
	Isolated bool           `json:"isolated,omitempty" yaml:"isolated,omitempty"`
	Mode     string         `json:"mode,omitempty"     yaml:"mode,omitempty"`
	Extra    map[string]any `json:"-"                  yaml:",inline"`
}

// RetrySpec configures node retry policy.
type RetrySpec struct {
	MaxAttempts  int            `json:"max_attempts,omitempty"  yaml:"max_attempts,omitempty"`
	OnFailure    string         `json:"on_failure,omitempty"    yaml:"on_failure,omitempty"`
	Backoff      *BackoffSpec   `json:"backoff,omitempty"       yaml:"backoff,omitempty"`
	NonRetryable []string       `json:"non_retryable,omitempty" yaml:"non_retryable,omitempty"`
	Extra        map[string]any `json:"-"                       yaml:",inline"`
}

// HarvestSpec configures action output harvest semantics.
type HarvestSpec struct {
	Kind        string         `json:"kind,omitempty"         yaml:"kind,omitempty"`
	Window      string         `json:"window,omitempty"       yaml:"window,omitempty"`
	Responder   string         `json:"responder,omitempty"    yaml:"responder,omitempty"`
	ContentRule string         `json:"content_rule,omitempty" yaml:"content_rule,omitempty"`
	Extra       map[string]any `json:"-"                      yaml:",inline"`
}

// FileParseKind is the closed file-import parse mode vocabulary.
type FileParseKind string

const (
	// FileParseJSON imports JSON files.
	FileParseJSON FileParseKind = "json"
	// FileParseText imports text files.
	FileParseText FileParseKind = "text"
)
