package dsl

// EffectSpec describes one emitted event or tool call. Lint enforces the XOR.
type EffectSpec struct {
	Emit *EmitSpec      `json:"emit,omitempty" yaml:"emit,omitempty"`
	Tool string         `json:"tool,omitempty" yaml:"tool,omitempty"`
	With map[string]any `json:"with,omitempty" yaml:"with,omitempty"`
}

// EmitSpec describes one internal event effect.
type EmitSpec struct {
	Kind    string         `json:"kind"              yaml:"kind"`
	Payload map[string]any `json:"payload,omitempty" yaml:"payload,omitempty"`
}

// TriggerEffects contains the six node-lifecycle effect lists.
type TriggerEffects struct {
	OnRetry      []EffectSpec `json:"on_retry,omitempty"      yaml:"on_retry,omitempty"`
	OnSuccess    []EffectSpec `json:"on_success,omitempty"    yaml:"on_success,omitempty"`
	OnPause      []EffectSpec `json:"on_pause,omitempty"      yaml:"on_pause,omitempty"`
	OnTimeout    []EffectSpec `json:"on_timeout,omitempty"    yaml:"on_timeout,omitempty"`
	OnCancel     []EffectSpec `json:"on_cancel,omitempty"     yaml:"on_cancel,omitempty"`
	OnQuarantine []EffectSpec `json:"on_quarantine,omitempty" yaml:"on_quarantine,omitempty"`
}

// TerminalEffects contains the exactly-once contract terminal effect lists.
type TerminalEffects struct {
	OnDone      []EffectSpec `json:"on_done,omitempty"      yaml:"on_done,omitempty"`
	OnNoOp      []EffectSpec `json:"on_noop,omitempty"      yaml:"on_noop,omitempty"`
	OnBlocked   []EffectSpec `json:"on_blocked,omitempty"   yaml:"on_blocked,omitempty"`
	OnFailed    []EffectSpec `json:"on_failed,omitempty"    yaml:"on_failed,omitempty"`
	OnExhausted []EffectSpec `json:"on_exhausted,omitempty" yaml:"on_exhausted,omitempty"`
	OnStalled   []EffectSpec `json:"on_stalled,omitempty"   yaml:"on_stalled,omitempty"`
	OnCanceled  []EffectSpec `json:"on_canceled,omitempty"  yaml:"on_canceled,omitempty"`
}

// ContractLifecycleState keeps optional terminal effects off the hot Contract value.
type ContractLifecycleState struct {
	TerminalStates  []TerminalState `json:"terminal_states,omitempty" yaml:"terminal_states,omitempty"`
	TerminalEffects `                                                 yaml:",inline"`
}
