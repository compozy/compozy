package contract

import "encoding/json"

type LoopDiffEndpoint struct {
	RunID      string        `json:"run_id"`
	Generation int64         `json:"generation"`
	Status     LoopRunStatus `json:"status"`
	AsOf       bool          `json:"as_of,omitempty"`
}

type LoopDiffValue struct {
	Inline json.RawMessage `json:"inline,omitempty"`
	Size   int             `json:"size,omitempty"`
	Hash   string          `json:"hash,omitempty"`
}

type LoopDiffInputRow struct {
	Key     string        `json:"key"`
	Base    LoopDiffValue `json:"base"`
	Against LoopDiffValue `json:"against"`
}

type LoopDiffNodeRow struct {
	NodeID    string        `json:"node_id"`
	ItemIndex int           `json:"item_index,omitempty"`
	Change    string        `json:"change"`
	Base      LoopDiffValue `json:"base,omitempty"`
	Against   LoopDiffValue `json:"against,omitempty"`
	Cause     string        `json:"cause,omitempty"`
}

type LoopDiffTerminalRow struct {
	Base    LoopRunStatus `json:"base"`
	Against LoopRunStatus `json:"against"`
}

type LoopDiffResponse struct {
	Kind                 string               `json:"kind"`
	Base                 LoopDiffEndpoint     `json:"base"`
	Against              LoopDiffEndpoint     `json:"against"`
	Inputs               []LoopDiffInputRow   `json:"inputs"`
	Nodes                []LoopDiffNodeRow    `json:"nodes"`
	Terminal             *LoopDiffTerminalRow `json:"terminal,omitempty"`
	DefinitionDivergence bool                 `json:"definition_divergence,omitempty"`
}

type RerunLoopRequest struct {
	FromNode  string `json:"from_node"`
	ItemIndex *int   `json:"item_index,omitempty"`
	Reason    string `json:"reason,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type RerunLoopResponse struct {
	RunID            string   `json:"run_id"`
	Generation       int64    `json:"generation"`
	ParentGeneration int64    `json:"parent_generation"`
	RerunNodes       []string `json:"rerun_nodes"`
	Carried          int      `json:"carried"`
	Replayed         bool     `json:"replayed,omitempty"`
}

type ForkLoopRequest struct {
	Generation int64          `json:"generation"`
	Inputs     map[string]any `json:"inputs,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
}

type ForkLoopResponse struct {
	Run      LoopRunPayload `json:"run"`
	Replayed bool           `json:"replayed,omitempty"`
}
