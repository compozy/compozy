package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	restartBoundaryEvent    = events.SessionRestartBoundary
	restartInterruptedEvent = events.SessionRestartInterrupted
	restartAttemptEvent     = events.SessionRestartAttempt
	restartResumedEvent     = events.SessionRestartResumed
	restartSkippedEvent     = events.SessionRestartSkipped
	restartContinueMessage  = "The daemon restarted while your previous turn was running. " +
		"Continue the interrupted work. " +
		"Verify the current state before repeating actions: a tool call without a recorded result may already have " +
		"completed. Do not assume that an interrupted action had no effect."
)

type restartCheckpoint struct {
	TurnID          string                      `json:"turn_id"`
	RuntimeRevision int64                       `json:"runtime_revision,omitempty"`
	Delivery        PromptDelivery              `json:"delivery,omitempty"`
	PolicyDigest    string                      `json:"policy_digest,omitempty"`
	Route           *store.SessionAcceptedRoute `json:"route,omitzero"`
	Permissions     string                      `json:"permissions,omitempty"`
	WorktreeID      string                      `json:"worktree_id,omitempty"`
	CWD             string                      `json:"cwd,omitempty"`
	Reason          string                      `json:"reason,omitempty"`
}

func (m *Manager) recordRestartBoundary(ctx context.Context, active *Session, req promptRequest) error {
	meta := active.Meta()
	digest, err := restartPolicyDigest(active.AgentDefinition())
	if err != nil {
		return err
	}
	checkpoint := restartCheckpointFromMeta(&meta, req.turnID)
	checkpoint.Delivery, checkpoint.PolicyDigest = req.delivery, digest
	return m.appendRestartEvent(ctx, &meta, restartBoundaryEvent, checkpoint, "")
}

func restartCheckpointFromMeta(meta *store.SessionMeta, turnID string) restartCheckpoint {
	_, revision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelectionValue())
	return restartCheckpoint{TurnID: turnID, Route: store.CloneSessionAcceptedRoute(meta.AcceptedRoute),
		RuntimeRevision: revision,
		Permissions:     meta.EffectivePermissionsValue(), WorktreeID: meta.WorktreeIDValue(), CWD: meta.CWDValue()}
}

func restartPolicyDigest(agent compozyconfig.AgentDef) (string, error) {
	policy := struct {
		Permissions, WorkspaceAccess string
		Tools, Toolsets, DenyTools   []string
		Skills                       compozyconfig.AgentSkillsConfig
		MCPServers                   []compozyconfig.MCPServer
	}{Permissions: agent.Permissions, WorkspaceAccess: agent.WorkspaceAccess,
		Tools: agent.Tools, Toolsets: agent.Toolsets, DenyTools: agent.DenyTools,
		Skills: agent.Skills, MCPServers: agent.MCPServers}
	data, err := json.Marshal(policy)
	if err != nil {
		return "", fmt.Errorf("session: fingerprint restart policy: %w", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func restartEventID(turnID, eventType string) string {
	digest := sha256.Sum256([]byte(turnID + ":" + eventType))
	return "ev_restart_" + hex.EncodeToString(digest[:16])
}

func (m *Manager) appendRestartEvent(
	ctx context.Context, meta *store.SessionMeta, eventType string, checkpoint restartCheckpoint, text string,
) error {
	id := restartEventID(checkpoint.TurnID, eventType)
	if eventType == restartAttemptEvent {
		var err error
		id, err = m.newRepairEventID()
		if err != nil {
			return err
		}
	} else {
		exists, err := m.restartEventExists(ctx, meta.ID, checkpoint.TurnID, eventType)
		if err != nil || exists {
			return err
		}
	}
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		return fmt.Errorf("session: encode restart checkpoint: %w", err)
	}
	event := acp.AgentEvent{SessionID: meta.ID, TurnID: checkpoint.TurnID, Type: eventType,
		Timestamp: m.now().UTC(), Text: text, Raw: raw}
	content, err := marshalAgentEvent(event)
	if err != nil {
		return err
	}
	persisted, inserted, err := m.appendDurableSessionEvent(ctx, meta.ID, store.SessionEvent{
		ID: id, SessionID: meta.ID, TurnID: checkpoint.TurnID,
		Type: eventType, AgentName: meta.AgentName, Content: content, Timestamp: event.Timestamp,
	})
	if err != nil {
		return fmt.Errorf("session: persist restart checkpoint: %w", err)
	}
	if inserted {
		m.publishSessionEventByID(ctx, meta.ID, persisted)
	}
	return nil
}

func decodeRestartCheckpoint(event store.SessionEvent) (restartCheckpoint, error) {
	agent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		return restartCheckpoint{}, err
	}
	var checkpoint restartCheckpoint
	if err := json.Unmarshal(agent.Raw, &checkpoint); err != nil {
		return restartCheckpoint{}, fmt.Errorf("session: decode restart checkpoint: %w", err)
	}
	if checkpoint.TurnID == "" || checkpoint.TurnID != event.TurnID {
		return restartCheckpoint{}, fmt.Errorf("session: restart checkpoint turn mismatch")
	}
	return checkpoint, nil
}

func (m *Manager) latestRestartBoundary(ctx context.Context, meta *store.SessionMeta) (restartCheckpoint, bool, error) {
	rows, err := m.Events(ctx, meta.ID, store.EventQuery{Types: []string{
		restartBoundaryEvent, acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry,
	}, Limit: 1, Archive: store.EventArchiveUnarchived})
	if err != nil || len(rows) == 0 {
		return restartCheckpoint{}, false, err
	}
	row := rows[0]
	if row.Type == restartBoundaryEvent {
		checkpoint, err := decodeRestartCheckpoint(row)
		return checkpoint, err == nil, err
	}
	checkpoint := restartCheckpointFromMeta(meta, row.TurnID)
	input, err := transcript.UnmarshalAgentEvent(row.Content)
	if err != nil {
		return restartCheckpoint{}, false, err
	}
	if strings.HasPrefix(strings.TrimSpace(input.Text), "/") {
		checkpoint.Delivery = PromptDeliveryMaintenance
	}
	return checkpoint, true, nil
}

func (m *Manager) restartTurnTerminal(ctx context.Context, id, turnID string) (bool, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{TurnID: turnID,
		Types: []string{acp.EventTypeDone, acp.EventTypeError}, Limit: 1})
	return len(rows) > 0, err
}

func (m *Manager) restartEventExists(ctx context.Context, id, turnID, eventType string) (bool, error) {
	rows, err := m.Events(ctx, id, store.EventQuery{ID: restartEventID(turnID, eventType), Limit: 1})
	return len(rows) > 0, err
}
