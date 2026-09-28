package daemon

import (
	"errors"

	"fmt"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

type toolListInput struct {
	Offset int `json:"offset,omitempty"`
	Limit  int `json:"limit,omitempty"`
}

type toolSearchInput struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

type toolInfoInput struct {
	ToolID string `json:"tool_id"`
}

type skillListInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type skillSearchInput struct {
	Query       string `json:"query"`
	WorkspaceID string `json:"workspace,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type skillViewInput struct {
	Name        string `json:"name"`
	CommandID   string `json:"command_id,omitempty"`
	WorkspaceID string `json:"workspace,omitempty"`
	File        string `json:"file,omitempty"`
}

type sessionIDInput struct {
	WorkspaceID string `json:"workspace"`
	SessionID   string `json:"session_id"`
}

type sessionEventQueryInput struct {
	WorkspaceID   string `json:"workspace"`
	SessionID     string `json:"session_id"`
	Type          string `json:"type,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
	TurnID        string `json:"turn_id,omitempty"`
	AfterSequence *int64 `json:"after_sequence,omitempty"`
	Limit         int    `json:"limit,omitempty"`
	Since         string `json:"since,omitempty"`
	Archive       string `json:"archive,omitempty"`
}

type agentHeartbeatStatusInput struct {
	WorkspaceID             string `json:"workspace"`
	AgentName               string `json:"agent_name"`
	SessionID               string `json:"session_id,omitempty"`
	IncludeSessionHealth    bool   `json:"include_session_health,omitempty"`
	IncludeRecentWakeEvents bool   `json:"include_recent_wake_events,omitempty"`
}

type agentHeartbeatWakeInput struct {
	WorkspaceID string `json:"workspace"`
	AgentName   string `json:"agent_name"`
	SessionID   string `json:"session_id"`
	Source      string `json:"source,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
}

func (i sessionEventQueryInput) eventQuery(id toolspkg.ToolID) (store.EventQuery, error) {
	archive := store.EventArchiveUnarchived
	switch strings.TrimSpace(i.Archive) {
	case "", nativeFilterActiveValue:
	case "archived":
		archive = store.EventArchiveArchived
	case nativeFilterAllValue:
		archive = store.EventArchiveAll
	default:
		return store.EventQuery{}, nativeInputError(id, errors.New("archive must be active, archived, or all"))
	}
	query := store.EventQuery{
		Type:      strings.TrimSpace(i.Type),
		AgentName: strings.TrimSpace(i.AgentName),
		TurnID:    strings.TrimSpace(i.TurnID),
		Forward:   i.AfterSequence != nil,
		Limit:     i.Limit,
		Archive:   archive,
	}
	if i.AfterSequence != nil {
		query.AfterSequence = *i.AfterSequence
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
