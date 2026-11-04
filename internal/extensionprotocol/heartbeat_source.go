package protocol

import "encoding/json"

type HeartbeatSourceKind struct {
	Kind         string          `json:"kind"`
	Title        string          `json:"title"`
	Description  string          `json:"description,omitempty"`
	ParamsSchema json.RawMessage `json:"params_schema,omitempty"`
}
