package dsl

// Contract defines goal, verification, and stop semantics.
type Contract struct {
	Goal             string          `json:"goal"                      yaml:"goal"`
	DefinitionOfDone string          `json:"definition_of_done"        yaml:"definition_of_done"`
	Constraints      []string        `json:"constraints,omitempty"     yaml:"constraints,omitempty"`
	Boundaries       []string        `json:"boundaries,omitempty"      yaml:"boundaries,omitempty"`
	StopWhen         string          `json:"stop_when,omitempty"       yaml:"stop_when,omitempty"`
	Verification     []GateCriterion `json:"verification,omitempty"    yaml:"verification,omitempty"`
	TerminalStates   []TerminalState `json:"terminal_states,omitempty" yaml:"terminal_states,omitempty"`
	IterationCap     int             `json:"iteration_cap"             yaml:"iteration_cap"`
	NoProgress       NoProgress      `json:"no_progress"               yaml:"no_progress"`
	Budget           Budget          `json:"budget"                    yaml:"budget"`
	ModelDefaults    *ModelDefaults  `json:"model_defaults,omitempty"  yaml:"model_defaults,omitempty"`
	Extra            map[string]any  `json:"-"                         yaml:",inline"`
}

// Normalize gives optional ADR-018 fields empty values when absent.
func (c *Contract) Normalize() {
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
	if c.NoProgress.HashFields == nil {
		c.NoProgress.HashFields = []string{}
	}
}

// NoProgress defines the generation-level progress signature.
type NoProgress struct {
	Window     int            `json:"window"                yaml:"window"`
	HashFields []string       `json:"hash_fields,omitempty" yaml:"hash_fields,omitempty"`
	Extra      map[string]any `json:"-"                     yaml:",inline"`
}

// Budget defines opt-in hard limits.
type Budget struct {
	Tokens       int            `json:"tokens"                yaml:"tokens"`
	WallClockSec int            `json:"wall_clock_sec"        yaml:"wall_clock_sec"`
	OnExceeded   BudgetExceeded `json:"on_exceeded,omitempty" yaml:"on_exceeded,omitempty"`
	Extra        map[string]any `json:"-"                     yaml:",inline"`
}

// ModelDefaults defines default models for loop-owned worker and judge sessions.
type ModelDefaults struct {
	Worker string `json:"worker,omitempty" yaml:"worker,omitempty"`
	Judge  string `json:"judge,omitempty"  yaml:"judge,omitempty"`
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
)

// IsKnownTerminalState reports whether value belongs to the closed terminal outcome vocabulary.
func IsKnownTerminalState(value TerminalState) bool {
	switch value {
	case TerminalDone, TerminalNoOp, TerminalBlocked, TerminalFailed, TerminalExhausted, TerminalStalled:
		return true
	default:
		return false
	}
}
