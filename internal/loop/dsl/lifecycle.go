package dsl

// NodeLifecycleState keeps optional reliability grammar off the hot Node value.
type NodeLifecycleState struct {
	Deadline       string          `json:"deadline,omitempty"        yaml:"deadline,omitempty"`
	ResultContract *ResultContract `json:"result_contract,omitempty" yaml:"result_contract,omitempty"`
	OnError        *ErrorPolicy    `json:"on_error,omitempty"        yaml:"on_error,omitempty"`
	TriggerEffects `                                                   yaml:",inline"`
	OnParentClose  ParentClosePolicy `json:"on_parent_close,omitempty" yaml:"on_parent_close,omitempty"`
}

// BackoffSpec bounds the retry delay for one action node.
type BackoffSpec struct {
	Base string `json:"base,omitempty" yaml:"base,omitempty"`
	Max  string `json:"max,omitempty"  yaml:"max,omitempty"`
}

// ResultContract declares the payload fields that represent an application failure.
type ResultContract struct {
	FailureField string `json:"failure_field"           yaml:"failure_field"`
	MessageField string `json:"message_field,omitempty" yaml:"message_field,omitempty"`
}

// ErrorPolicy combines error flow with observational effects.
type ErrorPolicy struct {
	Route     NodeID       `json:"route,omitempty"      yaml:"route,omitempty"`
	AllowFail bool         `json:"allow_fail,omitempty" yaml:"allow_fail,omitempty"`
	Effects   []EffectSpec `json:"effects,omitempty"    yaml:"effects,omitempty"`
}

// ParentClosePolicy controls what happens to an awaited child when its parent closes.
type ParentClosePolicy string

const (
	// ParentCloseTerminate closes the child according to the parent's terminal outcome.
	ParentCloseTerminate ParentClosePolicy = "terminate"
	// ParentCloseAbandon leaves the child running independently.
	ParentCloseAbandon ParentClosePolicy = "abandon"
	parentCloseCancel  ParentClosePolicy = "cancel"
)

// IsKnownParentClosePolicy reports whether value is in the closed parent-close vocabulary.
func IsKnownParentClosePolicy(value ParentClosePolicy) bool {
	switch value {
	case ParentCloseTerminate, ParentCloseAbandon:
		return true
	default:
		return false
	}
}

// NormalizeStoredParentClosePolicy maps durable predecessor values onto the authored vocabulary.
func NormalizeStoredParentClosePolicy(value ParentClosePolicy) (ParentClosePolicy, bool) {
	switch value {
	case ParentCloseTerminate, ParentCloseAbandon:
		return value, true
	case parentCloseCancel:
		return ParentCloseTerminate, true
	default:
		return "", false
	}
}
