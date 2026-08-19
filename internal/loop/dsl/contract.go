package dsl

// Contract defines goal, verification, and stop semantics.
type Contract struct {
	Goal                    string           `json:"goal"                       yaml:"goal"`
	DefinitionOfDone        string           `json:"definition_of_done"         yaml:"definition_of_done"`
	Constraints             []string         `json:"constraints,omitempty"      yaml:"constraints,omitempty"`
	Boundaries              []string         `json:"boundaries,omitempty"       yaml:"boundaries,omitempty"`
	StopWhen                StopWhenSpec     `json:"stop_when,omitzero"         yaml:"stop_when,omitempty"`
	Verification            []GateCriterion  `json:"verification,omitempty"     yaml:"verification,omitempty"`
	IterationCap            int              `json:"iteration_cap"              yaml:"iteration_cap"`
	NoProgress              NoProgress       `json:"no_progress"                yaml:"no_progress"`
	Budget                  Budget           `json:"budget"                     yaml:"budget"`
	RuntimeDefaults         *RuntimeDefaults `json:"runtime_defaults,omitempty" yaml:"runtime_defaults,omitempty"`
	RuntimeRules            []RuntimeRule    `json:"runtime_rules,omitempty"    yaml:"runtime_rules,omitempty"`
	*ContractLifecycleState `                                                   yaml:",inline"`
	Extra                   map[string]any `json:"-"                          yaml:",inline"`
}

// Normalize gives optional ADR-018 fields empty values when absent.
func (c *Contract) Normalize() {
	if c.ContractLifecycleState == nil {
		c.ContractLifecycleState = &ContractLifecycleState{}
	}
	if c.Constraints == nil {
		c.Constraints = []string{}
	}
	if c.Boundaries == nil {
		c.Boundaries = []string{}
	}
	if c.Verification == nil {
		c.Verification = []GateCriterion{}
	}
	if c.TerminalStates == nil {
		c.TerminalStates = []TerminalState{}
	}
}

// NoProgress defines the generation-level progress signature.
type NoProgress struct {
	Window int            `json:"window" yaml:"window"`
	Extra  map[string]any `json:"-"      yaml:",inline"`
}

// Budget defines opt-in hard limits.
type Budget struct {
	Tokens       int32          `json:"tokens"                yaml:"tokens"`
	WallClockSec int32          `json:"wall_clock_sec"        yaml:"wall_clock_sec"`
	OnExceeded   BudgetExceeded `json:"on_exceeded,omitempty" yaml:"on_exceeded,omitempty"`
	Extra        map[string]any `json:"-"                     yaml:",inline"`
}

// BudgetExceeded controls the outcome for a set budget breach.
type BudgetExceeded string

const (
	// BudgetExceededHalt maps a breach to exhausted.
	BudgetExceededHalt BudgetExceeded = "halt"
	// BudgetExceededEscalate maps a breach to needs-approval.
	BudgetExceededEscalate BudgetExceeded = "escalate"
)

// TerminalState is the closed terminal outcome vocabulary.
type TerminalState string

const (
	// TerminalDone means the contract is verified.
	TerminalDone TerminalState = "done"
	// TerminalNoOp means the loop had nothing to do.
	TerminalNoOp TerminalState = "no-op"
	// TerminalBlocked means an explicit external dependency blocked progress.
	TerminalBlocked TerminalState = "blocked"
	// TerminalFailed means an unrecoverable failure occurred.
	TerminalFailed TerminalState = "failed"
	// TerminalExhausted means a hard stop limit tripped.
	TerminalExhausted TerminalState = "exhausted"
	// TerminalStalled means progress stopped.
	TerminalStalled TerminalState = "stalled"
	// TerminalCanceled means an operator canceled or killed the run.
	TerminalCanceled TerminalState = "canceled"
)

// IsKnownTerminalState reports whether value belongs to the closed terminal outcome vocabulary.
func IsKnownTerminalState(value TerminalState) bool {
	switch value {
	case TerminalDone,
		TerminalNoOp,
		TerminalBlocked,
		TerminalFailed,
		TerminalExhausted,
		TerminalStalled,
		TerminalCanceled:
		return true
	default:
		return false
	}
}
