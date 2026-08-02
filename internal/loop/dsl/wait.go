package dsl

// WaitAheadArrival controls how an event observed before entry is handled.
type WaitAheadArrival string

const (
	// WaitAheadConsumeOnEntry consumes the matching event when the cell enters the wait.
	WaitAheadConsumeOnEntry WaitAheadArrival = "consume_on_entry"
	// WaitAheadReject rejects events that arrived before the cell entered the wait.
	WaitAheadReject WaitAheadArrival = "reject"
)

// IsKnownWaitAheadArrival reports whether value belongs to the closed wait vocabulary.
func IsKnownWaitAheadArrival(value WaitAheadArrival) bool {
	switch value {
	case WaitAheadConsumeOnEntry, WaitAheadReject:
		return true
	default:
		return false
	}
}

// WaitParams declares exactly one timer, timestamp, or event wait.
type WaitParams struct {
	For          string             `json:"for,omitempty"           yaml:"for,omitempty"`
	Until        string             `json:"until,omitempty"         yaml:"until,omitempty"`
	Event        *EventSubscription `json:"event,omitempty"         yaml:"event,omitempty"`
	Expect       Schema             `json:"expect,omitempty"        yaml:"expect,omitempty"`
	AheadArrival WaitAheadArrival   `json:"ahead_arrival,omitempty" yaml:"ahead_arrival,omitempty"`
	Expires      *WaitExpiry        `json:"expires,omitempty"       yaml:"expires,omitempty"`
	Extra        map[string]any     `json:"-"                       yaml:",inline"`
}

// WaitExpiry describes an optional expiry path for waits and human gates.
type WaitExpiry struct {
	After    string         `json:"after"              yaml:"after"`
	Escalate []EffectSpec   `json:"escalate,omitempty" yaml:"escalate,omitempty"`
	Route    NodeID         `json:"route,omitempty"    yaml:"route,omitempty"`
	Extra    map[string]any `json:"-"                  yaml:",inline"`
}
