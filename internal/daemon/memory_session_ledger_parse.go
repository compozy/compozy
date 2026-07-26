package daemon

import (
	"bufio"
	"bytes"

	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"fmt"

	"os"

	"strings"

	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	extractorpkg "github.com/compozy/agh/internal/memory/extractor"

	localprovider "github.com/compozy/agh/internal/memory/provider/local"
)

type sessionLedgerMetaLine struct {
	Type          string `json:"type"`
	Version       int    `json:"version"`
	SessionID     string `json:"session_id"`
	WorkspaceID   string `json:"workspace_id"`
	SpawnParentID string `json:"spawn_parent_id,omitempty"`
	RootSessionID string `json:"root_session_id,omitempty"`
	SpawnDepth    int    `json:"spawn_depth,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
	EndedAt       string `json:"ended_at,omitempty"`
}

type sessionLedgerEventLine struct {
	Type      string          `json:"type"`
	Sequence  int64           `json:"sequence"`
	EventType string          `json:"event_type"`
	Content   json.RawMessage `json:"content,omitempty"`
	Timestamp string          `json:"timestamp,omitempty"`
}

type sessionLedgerLineType struct {
	Type string `json:"type"`
}

func readSessionLedger(path string) (contract.MemorySessionLedgerResponse, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return contract.MemorySessionLedgerResponse{}, fmt.Errorf(
			"daemon: read memory session ledger %q: %w",
			path,
			err,
		)
	}
	checksum := sha256.Sum256(data)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	response := contract.MemorySessionLedgerResponse{
		Events: make([]contract.MemorySessionLedgerEntryPayload, 0),
	}
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var lineType sessionLedgerLineType
		if err := json.Unmarshal(raw, &lineType); err != nil {
			return contract.MemorySessionLedgerResponse{}, fmt.Errorf("daemon: decode ledger line type: %w", err)
		}
		switch lineType.Type {
		case "ledger_meta":
			var meta sessionLedgerMetaLine
			if err := json.Unmarshal(raw, &meta); err != nil {
				return contract.MemorySessionLedgerResponse{}, fmt.Errorf("daemon: decode ledger meta: %w", err)
			}
			response.Meta = contract.MemorySessionLedgerMetaPayload{
				Version:         meta.Version,
				SessionID:       strings.TrimSpace(meta.SessionID),
				WorkspaceID:     strings.TrimSpace(meta.WorkspaceID),
				RootSessionID:   strings.TrimSpace(meta.RootSessionID),
				ParentSessionID: strings.TrimSpace(meta.SpawnParentID),
				SpawnDepth:      meta.SpawnDepth,
				Path:            path,
				Checksum:        hex.EncodeToString(checksum[:]),
				CreatedAt:       parseMemoryTime(meta.StartedAt),
			}
			if stoppedAt := parseMemoryTime(meta.EndedAt); !stoppedAt.IsZero() {
				response.Meta.StoppedAt = &stoppedAt
			}
		case "session_event":
			var event sessionLedgerEventLine
			if err := json.Unmarshal(raw, &event); err != nil {
				return contract.MemorySessionLedgerResponse{}, fmt.Errorf("daemon: decode ledger event: %w", err)
			}
			response.Events = append(response.Events, contract.MemorySessionLedgerEntryPayload{
				Sequence:  event.Sequence,
				EventType: strings.TrimSpace(event.EventType),
				EmittedAt: parseMemoryTime(event.Timestamp),
				Payload:   ledgerPayload(event.Content),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return contract.MemorySessionLedgerResponse{}, fmt.Errorf("daemon: scan memory session ledger: %w", err)
	}
	if response.Meta.SessionID == "" {
		return contract.MemorySessionLedgerResponse{}, fmt.Errorf("%w: memory session ledger %q", os.ErrInvalid, path)
	}
	if response.Meta.CreatedAt.IsZero() {
		response.Meta.CreatedAt = time.Now().UTC()
	}
	return response, nil
}

func ledgerPayload(raw json.RawMessage) map[string]any {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err == nil {
		return payload
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{"raw": string(raw)}
	}
	return map[string]any{memoryRuntimeValueKey: value}
}

func isToolLedgerEvent(eventType string) bool {
	normalized := strings.ToLower(strings.TrimSpace(eventType))
	return normalized == acp.EventTypeToolCall ||
		normalized == acp.EventTypeToolResult ||
		strings.Contains(normalized, "tool")
}

func parseMemoryTime(raw string) time.Time {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed.UTC()
	}
	parsed, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed.UTC()
	}
	return time.Time{}
}

func intFromInt64(value int64) int {
	if value > int64(^uint(0)>>1) {
		return int(^uint(0) >> 1)
	}
	return int(value)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

var _ memcontract.MemoryProvider = (*localprovider.Provider)(nil)
var _ extractorpkg.ProposalSink = (*daemonMemoryProposalSink)(nil)
