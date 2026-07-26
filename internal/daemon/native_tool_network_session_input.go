package daemon

import (
	"encoding/json"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/agh/internal/network"

	"github.com/compozy/agh/internal/store"

	toolspkg "github.com/compozy/agh/internal/tools"
)

type toolListInput struct {
	Limit int `json:"limit,omitempty"`
}

type toolSearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type toolInfoInput struct {
	ToolID string `json:"tool_id"`
}

type skillListInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type skillSearchInput struct {
	Query       string `json:"query"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type skillViewInput struct {
	Name        string `json:"name"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	File        string `json:"file,omitempty"`
}

type networkPeersInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel,omitempty"`
}

type networkChannelsInput struct {
	WorkspaceID string `json:"workspace_id"`
}

type networkInboxInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id,omitempty"`
}

type networkSendInput struct {
	WorkspaceID string               `json:"workspace_id"`
	SessionID   string               `json:"session_id,omitempty"`
	Channel     string               `json:"channel"`
	Surface     string               `json:"surface,omitempty"`
	ThreadID    string               `json:"thread_id,omitempty"`
	DirectID    string               `json:"direct_id,omitempty"`
	Kind        string               `json:"kind"`
	To          string               `json:"to,omitempty"`
	Mentions    []string             `json:"mentions,omitempty"`
	Body        json.RawMessage      `json:"body"`
	WorkID      string               `json:"work_id,omitempty"`
	ReplyTo     string               `json:"reply_to,omitempty"`
	TraceID     string               `json:"trace_id,omitempty"`
	CausationID string               `json:"causation_id,omitempty"`
	ExpiresAt   *int64               `json:"expires_at,omitempty"`
	ID          string               `json:"id,omitempty"`
	Ext         network.ExtensionMap `json:"ext,omitempty"`
}

type networkDirectResolveInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id,omitempty"`
	Channel     string `json:"channel"`
	PeerID      string `json:"peer_id"`
}

type networkWorkInput struct {
	WorkspaceID string `json:"workspace_id"`
	WorkID      string `json:"work_id"`
}

type networkSubscriptionsInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	ThreadID    string `json:"thread_id,omitempty"`
	SessionID   string `json:"session_id,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type networkSubscriptionInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	ThreadID    string `json:"thread_id,omitempty"`
	SessionID   string `json:"session_id"`
}

type networkSubscriptionDeleteInput struct {
	WorkspaceID string `json:"workspace_id"`
	Channel     string `json:"channel"`
	ThreadID    string `json:"thread_id,omitempty"`
	SessionID   string `json:"session_id"`
}

type sessionIDInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
}

type sessionEventQueryInput struct {
	WorkspaceID   string `json:"workspace_id"`
	SessionID     string `json:"session_id"`
	Type          string `json:"type,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
	TurnID        string `json:"turn_id,omitempty"`
	AfterSequence int64  `json:"after_sequence,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Since         string `json:"since,omitempty"`
}

type agentHeartbeatStatusInput struct {
	WorkspaceID             string `json:"workspace_id"`
	AgentName               string `json:"agent_name"`
	SessionID               string `json:"session_id,omitempty"`
	IncludeSessionHealth    bool   `json:"include_session_health,omitempty"`
	IncludeRecentWakeEvents bool   `json:"include_recent_wake_events,omitempty"`
}

type agentHeartbeatWakeInput struct {
	WorkspaceID string `json:"workspace_id"`
	AgentName   string `json:"agent_name"`
	SessionID   string `json:"session_id"`
	Source      string `json:"source,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

func (i sessionEventQueryInput) eventQuery(id toolspkg.ToolID) (store.EventQuery, error) {
	query := store.EventQuery{
		Type:          strings.TrimSpace(i.Type),
		AgentName:     strings.TrimSpace(i.AgentName),
		TurnID:        strings.TrimSpace(i.TurnID),
		AfterSequence: i.AfterSequence,
		Limit:         i.Limit,
	}
	if rawSince := strings.TrimSpace(i.Since); rawSince != "" {
		since, err := time.Parse(time.RFC3339, rawSince)
		if err != nil {
			return store.EventQuery{}, toolspkg.NewToolError(
				toolspkg.ErrorCodeInvalidInput,
				id,
				"session event since must be an RFC3339 timestamp",
				fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
				toolspkg.ReasonSchemaInvalid,
			)
		}
		query.Since = since
	}
	if err := query.Validate(); err != nil {
		return store.EventQuery{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"session event query is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return query, nil
}

type workspaceRefInput struct {
	Workspace string `json:"workspace"`
}
