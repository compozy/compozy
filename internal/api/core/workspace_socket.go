package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

const (
	// WorkspaceEventSocketType is the live event message type for daemon workspace sockets.
	WorkspaceEventSocketType = "workspace.event"
	// WorkspaceHeartbeatSocketType is the heartbeat message type for daemon workspace sockets.
	WorkspaceHeartbeatSocketType = "workspace.heartbeat"
	// WorkspaceOverflowSocketType is the overflow message type for daemon workspace sockets.
	WorkspaceOverflowSocketType = "workspace.overflow"
	// WorkspaceErrorSocketType is the error message type for daemon workspace sockets.
	WorkspaceErrorSocketType = "error"
)

// WorkspaceSocketMessage is the JSON envelope sent over workspace WebSocket connections.
type WorkspaceSocketMessage struct {
	Type    string          `json:"type"`
	ID      string          `json:"id,omitempty"`
	Payload json.RawMessage `json:"payload"`
}

// WorkspaceSocketHeartbeatPayload carries a workspace socket heartbeat.
type WorkspaceSocketHeartbeatPayload struct {
	WorkspaceID string    `json:"workspace_id"`
	TS          time.Time `json:"ts"`
}

// WorkspaceSocketOverflowPayload notifies a browser that workspace events were dropped.
type WorkspaceSocketOverflowPayload struct {
	WorkspaceID string    `json:"workspace_id"`
	Reason      string    `json:"reason"`
	TS          time.Time `json:"ts"`
}

func WorkspaceEventSocketMessage(event WorkspaceEvent) (WorkspaceSocketMessage, error) {
	return newWorkspaceSocketMessage(
		WorkspaceEventSocketType,
		fmt.Sprintf("%d", event.Seq),
		event,
	)
}

func WorkspaceHeartbeatSocketMessage(workspaceID string, now time.Time) (WorkspaceSocketMessage, error) {
	return newWorkspaceSocketMessage(
		WorkspaceHeartbeatSocketType,
		"",
		WorkspaceSocketHeartbeatPayload{
			WorkspaceID: strings.TrimSpace(workspaceID),
			TS:          now.UTC(),
		},
	)
}

func WorkspaceOverflowSocketMessage(
	workspaceID string,
	now time.Time,
	reason string,
) (WorkspaceSocketMessage, error) {
	return newWorkspaceSocketMessage(
		WorkspaceOverflowSocketType,
		"",
		WorkspaceSocketOverflowPayload{
			WorkspaceID: strings.TrimSpace(workspaceID),
			Reason:      strings.TrimSpace(reason),
			TS:          now.UTC(),
		},
	)
}

func WorkspaceErrorSocketMessage(payload TransportError) (WorkspaceSocketMessage, error) {
	return newWorkspaceSocketMessage(WorkspaceErrorSocketType, "", payload)
}

func newWorkspaceSocketMessage[T any](messageType string, id string, payload T) (WorkspaceSocketMessage, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return WorkspaceSocketMessage{}, fmt.Errorf("marshal workspace socket payload: %w", err)
	}
	if len(encoded) == 0 {
		encoded = []byte("null")
	}
	return WorkspaceSocketMessage{
		Type:    strings.TrimSpace(messageType),
		ID:      strings.TrimSpace(id),
		Payload: encoded,
	}, nil
}

func WriteWorkspaceSocketMessage(
	ctx context.Context,
	conn *websocket.Conn,
	message WorkspaceSocketMessage,
) error {
	if conn == nil {
		return errors.New("workspace websocket connection is required")
	}
	if strings.TrimSpace(message.Type) == "" {
		return errors.New("workspace websocket message type is required")
	}
	if len(message.Payload) == 0 {
		message.Payload = json.RawMessage("null")
	}
	if err := wsjson.Write(ctx, conn, message); err != nil {
		return fmt.Errorf("write workspace websocket message: %w", err)
	}
	return nil
}
