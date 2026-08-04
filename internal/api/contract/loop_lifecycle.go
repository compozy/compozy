package contract

// LoopBackoffSpec bounds retry delays.
type LoopBackoffSpec struct {
	Base string `json:"base,omitempty"`
	Max  string `json:"max,omitempty"`
}

// LoopRetrySpec configures node retry policy.
type LoopRetrySpec struct {
	MaxAttempts  int              `json:"max_attempts,omitempty"`
	OnFailure    string           `json:"on_failure,omitempty"`
	Backoff      *LoopBackoffSpec `json:"backoff,omitempty"`
	NonRetryable []string         `json:"non_retryable,omitempty"`
}

// LoopResultContract identifies application-failure fields in a node result.
type LoopResultContract struct {
	FailureField string `json:"failure_field"`
	MessageField string `json:"message_field,omitempty"`
}

// LoopEmitSpec describes an emitted internal event.
type LoopEmitSpec struct {
	Kind    string         `json:"kind"`
	Payload map[string]any `json:"payload,omitempty"`
}

// LoopEffectSpec describes one emitted event or tool call.
type LoopEffectSpec struct {
	Emit *LoopEmitSpec  `json:"emit,omitempty"`
	Tool string         `json:"tool,omitempty"`
	With map[string]any `json:"with,omitempty"`
}

// LoopErrorPolicy combines error flow with observational effects.
type LoopErrorPolicy struct {
	Route     string           `json:"route,omitempty"`
	AllowFail bool             `json:"allow_fail,omitempty"`
	Effects   []LoopEffectSpec `json:"effects,omitempty"`
}

// LoopWaitExpiry describes an optional expiry path for waits and gates.
type LoopWaitExpiry struct {
	After    string           `json:"after"`
	Escalate []LoopEffectSpec `json:"escalate,omitempty"`
	Route    string           `json:"route,omitempty"`
}

// LoopNodeLifecycleState carries optional node reliability fields.
type LoopNodeLifecycleState struct {
	Deadline       string                `json:"deadline,omitempty"`
	ResultContract *LoopResultContract   `json:"result_contract,omitempty"`
	OnError        *LoopErrorPolicy      `json:"on_error,omitempty"`
	OnRetry        []LoopEffectSpec      `json:"on_retry,omitempty"`
	OnSuccess      []LoopEffectSpec      `json:"on_success,omitempty"`
	OnPause        []LoopEffectSpec      `json:"on_pause,omitempty"`
	OnTimeout      []LoopEffectSpec      `json:"on_timeout,omitempty"`
	OnCancel       []LoopEffectSpec      `json:"on_cancel,omitempty"`
	OnQuarantine   []LoopEffectSpec      `json:"on_quarantine,omitempty"`
	OnParentClose  LoopParentClosePolicy `json:"on_parent_close,omitempty"`
	Expires        *LoopWaitExpiry       `json:"expires,omitempty"`
}

// LoopContractLifecycleState carries optional terminal reactions.
type LoopContractLifecycleState struct {
	OnDone      []LoopEffectSpec `json:"on_done,omitempty"`
	OnNoOp      []LoopEffectSpec `json:"on_noop,omitempty"`
	OnBlocked   []LoopEffectSpec `json:"on_blocked,omitempty"`
	OnFailed    []LoopEffectSpec `json:"on_failed,omitempty"`
	OnExhausted []LoopEffectSpec `json:"on_exhausted,omitempty"`
	OnStalled   []LoopEffectSpec `json:"on_stalled,omitempty"`
	OnCanceled  []LoopEffectSpec `json:"on_canceled,omitempty"`
}

// LoopParentClosePolicy is the closed run-loop parent-close vocabulary.
type LoopParentClosePolicy string

const (
	// LoopParentCloseTerminate closes the child with its parent.
	LoopParentCloseTerminate LoopParentClosePolicy = "terminate"
	// LoopParentCloseCancel requests a graceful child cancellation.
	LoopParentCloseCancel LoopParentClosePolicy = "cancel"
	// LoopParentCloseAbandon leaves the child running independently.
	LoopParentCloseAbandon LoopParentClosePolicy = "abandon"
)

// LoopParentClosePolicyValues returns the closed parent-close vocabulary.
func LoopParentClosePolicyValues() []string {
	return []string{
		string(LoopParentCloseTerminate),
		string(LoopParentCloseCancel),
		string(LoopParentCloseAbandon),
	}
}
