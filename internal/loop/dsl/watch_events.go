package dsl

// EventSubscription declares one internal watch-events subscription.
type EventSubscription struct {
	Kind   string `json:"kind"             yaml:"kind"`
	Filter string `json:"filter,omitempty" yaml:"filter,omitempty"`
}
