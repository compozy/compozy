package loop

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	DefaultNodeInventoryLimit = 50
	MaxNodeInventoryLimit     = 200
)

// NodeInventoryState is the closed workspace node-inventory vocabulary.
type NodeInventoryState string

const (
	NodeInventoryWaiting     NodeInventoryState = "waiting"
	NodeInventoryQuarantined NodeInventoryState = "quarantined"
	NodeInventoryAttention   NodeInventoryState = "attention"
	NodeInventoryRetrying    NodeInventoryState = "retrying"
)

// Valid reports whether the state belongs to the public inventory vocabulary.
func (s NodeInventoryState) Valid() bool {
	switch s {
	case NodeInventoryWaiting, NodeInventoryQuarantined, NodeInventoryAttention, NodeInventoryRetrying:
		return true
	default:
		return false
	}
}

// NodeInventoryQuery filters one workspace-scoped inventory page.
type NodeInventoryQuery struct {
	WorkspaceID WorkspaceID
	State       NodeInventoryState
	LoopName    string
	RunID       RunID
	Cursor      string
	Limit       int
}

// NodeInventoryItem is one stable-ordered inventory row with exactly one state detail.
type NodeInventoryItem struct {
	State      NodeInventoryState
	LoopRunID  RunID
	LoopName   string
	Generation int
	NodeID     NodeID
	ItemIndex  int
	StateAt    time.Time
	Wait       *NodeWait
	Control    *NodeControl
	Output     *GenerationOutput
	Attempt    *NodeAttempt
}

// NodeInventoryPage is one page and its opaque continuation cursor.
type NodeInventoryPage struct {
	Items      []NodeInventoryItem
	NextCursor string
}

// NodeInventoryReader owns workspace-scoped state inventory reads.
type NodeInventoryReader interface {
	ListNodeInventory(context.Context, NodeInventoryQuery) (NodeInventoryPage, error)
}

// NodeInventoryCursor is the decoded keyset cursor bound to the original filters.
type NodeInventoryCursor struct {
	WorkspaceID WorkspaceID        `json:"workspace_id"`
	State       NodeInventoryState `json:"state"`
	LoopName    string             `json:"loop,omitempty"`
	RunID       RunID              `json:"run_id,omitempty"`
	StateAt     time.Time          `json:"state_at"`
	LoopRunID   RunID              `json:"loop_run_id"`
	Generation  int                `json:"generation"`
	NodeID      NodeID             `json:"node_id"`
	ItemIndex   int                `json:"item_index"`
}

// NormalizeNodeInventoryQuery validates public filters and applies the default page size.
func NormalizeNodeInventoryQuery(query NodeInventoryQuery) (NodeInventoryQuery, error) {
	query.LoopName = strings.TrimSpace(query.LoopName)
	query.Cursor = strings.TrimSpace(query.Cursor)
	if strings.TrimSpace(string(query.WorkspaceID)) == "" {
		return NodeInventoryQuery{}, fmt.Errorf("%w: workspace_id is required", ErrValidation)
	}
	if !query.State.Valid() {
		return NodeInventoryQuery{}, fmt.Errorf("%w: node inventory state is invalid: %q", ErrValidation, query.State)
	}
	if query.Limit == 0 {
		query.Limit = DefaultNodeInventoryLimit
	}
	if query.Limit < 1 || query.Limit > MaxNodeInventoryLimit {
		return NodeInventoryQuery{}, fmt.Errorf(
			"%w: node inventory limit must be between 1 and %d",
			ErrValidation,
			MaxNodeInventoryLimit,
		)
	}
	return query, nil
}

// DecodeNodeInventoryCursor validates an opaque cursor against its workspace and filters.
func DecodeNodeInventoryCursor(query NodeInventoryQuery) (*NodeInventoryCursor, error) {
	if query.Cursor == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(query.Cursor)
	if err != nil {
		return nil, fmt.Errorf("%w: node inventory cursor is invalid", ErrValidation)
	}
	var cursor NodeInventoryCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return nil, fmt.Errorf("%w: node inventory cursor is invalid", ErrValidation)
	}
	if cursor.WorkspaceID != query.WorkspaceID || cursor.State != query.State ||
		cursor.LoopName != query.LoopName || cursor.RunID != query.RunID || cursor.StateAt.IsZero() ||
		cursor.LoopRunID == "" || cursor.NodeID == "" || cursor.Generation < 0 || cursor.ItemIndex < 0 {
		return nil, fmt.Errorf("%w: node inventory cursor does not match the requested scope", ErrValidation)
	}
	return &cursor, nil
}

// EncodeNodeInventoryCursor creates an opaque continuation cursor from the last returned row.
func EncodeNodeInventoryCursor(query NodeInventoryQuery, item NodeInventoryItem) (string, error) {
	cursor := NodeInventoryCursor{
		WorkspaceID: query.WorkspaceID,
		State:       query.State,
		LoopName:    query.LoopName,
		RunID:       query.RunID,
		StateAt:     item.StateAt.UTC(),
		LoopRunID:   item.LoopRunID,
		Generation:  item.Generation,
		NodeID:      item.NodeID,
		ItemIndex:   item.ItemIndex,
	}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("encode node inventory cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
