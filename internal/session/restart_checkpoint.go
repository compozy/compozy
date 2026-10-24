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
	restartContinuationKind = "restart_continuation"
	restartInterruptedEvent = events.SessionRestartInterrupted
	restartAttemptEvent     = events.SessionRestartAttempt
	restartResumedEvent     = events.SessionRestartResumed
	restartSkippedEvent     = events.SessionRestartSkipped
	restartContinueMessage  = "The daemon restarted while your previous turn was running. " +
		"Continue the interrupted work. " +
		"Verify the current state before repeating actions: a tool call without a recorded result may already have " +
		"completed. Do not assume that an interrupted action had no effect."
)

func (m *Manager) recordRestartBoundary(active *Session, req promptRequest) error {
	meta := active.Meta()
	digest, err := restartPolicyDigest(active.AgentDefinition())
	if err != nil {
		return err
	}
	checkpoint := restartCheckpointFromMeta(&meta, req.turnID)
	checkpoint.Delivery, checkpoint.PolicyDigest = string(req.delivery), digest
	active.mu.Lock()
	active.restartBoundary = store.CloneSessionRestartCheckpoint(&checkpoint)
	active.mu.Unlock()
	return m.persistSessionMetadataOnly(active)
}

func restartCheckpointFromMeta(meta *store.SessionMeta, turnID string) store.SessionRestartCheckpoint {
	_, revision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelectionValue())
	return store.SessionRestartCheckpoint{TurnID: turnID, Route: store.CloneSessionAcceptedRoute(meta.AcceptedRoute),
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
	data, err := json.Marshal(policy, json.Deterministic(true))
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
	ctx context.Context,
	meta *store.SessionMeta,
	eventType string,
	checkpoint store.SessionRestartCheckpoint,
	text string,
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

func decodeRestartCheckpoint(event store.SessionEvent) (store.SessionRestartCheckpoint, error) {
	agent, err := transcript.UnmarshalAgentEvent(event.Content)
	if err != nil {
		return store.SessionRestartCheckpoint{}, err
	}
	var checkpoint store.SessionRestartCheckpoint
	if err := json.Unmarshal(agent.Raw, &checkpoint); err != nil {
		return store.SessionRestartCheckpoint{}, fmt.Errorf("session: decode restart checkpoint: %w", err)
	}
	if checkpoint.TurnID == "" || checkpoint.TurnID != event.TurnID {
		return store.SessionRestartCheckpoint{}, fmt.Errorf("session: restart checkpoint turn mismatch")
	}
	return checkpoint, nil
}

func (m *Manager) latestRestartBoundary(
	ctx context.Context,
	meta *store.SessionMeta,
) (store.SessionRestartCheckpoint, bool, error) {
	rows, err := m.Events(ctx, meta.ID, store.EventQuery{Types: []string{
		acp.EventTypeUserMessage, acp.EventTypeSyntheticReentry,
	}, Limit: 1, Archive: store.EventArchiveUnarchived})
	if err != nil || len(rows) == 0 {
		return store.SessionRestartCheckpoint{}, false, err
	}
	row := rows[0]
	if checkpoint := meta.RestartBoundary; checkpoint != nil {
		return *store.CloneSessionRestartCheckpoint(checkpoint), checkpoint.TurnID == row.TurnID, nil
	}
	checkpoint := restartCheckpointFromMeta(meta, row.TurnID)
	input, err := transcript.UnmarshalAgentEvent(row.Content)
	if err != nil {
		return store.SessionRestartCheckpoint{}, false, err
	}
	if strings.HasPrefix(strings.TrimSpace(input.Text), "/") {
		checkpoint.Delivery = string(PromptDeliveryMaintenance)
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
