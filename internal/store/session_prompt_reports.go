package store

import (
	"encoding/json"
	"time"
)

type SessionPromptReport struct {
	ID         string          `json:"report_id"`
	InstanceID string          `json:"instance_id"`
	SourceKey  string          `json:"source_key"`
	Kind       string          `json:"kind"`
	Payload    json.RawMessage `json:"payload"`
}

type IndeterminatePromptReport struct {
	At        time.Time
	ID        string
	SessionID string
	TurnID    string
}
