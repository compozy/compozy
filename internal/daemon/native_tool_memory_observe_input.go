package daemon

import (
	"fmt"

	"strings"

	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/store"

	toolspkg "github.com/compozy/agh/internal/tools"
)

type memoryShowInput struct {
	Filename  string `json:"filename"`
	Scope     string `json:"scope,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	AgentTier string `json:"agent_tier,omitempty"`
}

type memorySearchInput struct {
	Query     string `json:"query,omitempty"`
	Q         string `json:"q,omitempty"`
	Scope     string `json:"scope,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	AgentName string `json:"agent_name,omitempty"`
	AgentTier string `json:"agent_tier,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

type memoryNoteInput struct {
	Content   string   `json:"content"`
	Slug      string   `json:"slug,omitempty"`
	Scope     string   `json:"scope,omitempty"`
	Workspace string   `json:"workspace,omitempty"`
	AgentName string   `json:"agent_name,omitempty"`
	AgentTier string   `json:"agent_tier,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

type memoryToolLocation struct {
	Store       *memorypkg.Store
	Scope       memcontract.Scope
	Workspace   string
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	Filename    string
}

type memoryToolSelector struct {
	Scope     string
	Workspace string
	AgentName string
	AgentTier string
}

type nativeMemoryRecallEntry struct {
	Key     string  `json:"key"`
	Content string  `json:"content"`
	Score   float64 `json:"score"`
}

type logQueryInput struct {
	WorkspaceID   string `json:"workspace_id"`
	SessionID     string `json:"session_id,omitempty"`
	AgentName     string `json:"agent_name,omitempty"`
	Type          string `json:"type,omitempty"`
	RunID         string `json:"run,omitempty"`
	ActorKind     string `json:"actor_kind,omitempty"`
	ActorID       string `json:"actor_id,omitempty"`
	Provider      string `json:"provider,omitempty"`
	Outcome       string `json:"outcome,omitempty"`
	Component     string `json:"component,omitempty"`
	ErrorOnly     bool   `json:"error_only,omitempty"`
	AfterSequence int64  `json:"after_seq,omitempty"`
	Since         string `json:"since,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

func (i logQueryInput) eventSummaryQuery(id toolspkg.ToolID) (store.EventSummaryQuery, error) {
	since, err := parseNativeOptionalRFC3339(id, "since", i.Since)
	if err != nil {
		return store.EventSummaryQuery{}, err
	}
	query := store.EventSummaryQuery{
		WorkspaceID:   strings.TrimSpace(i.WorkspaceID),
		SessionID:     strings.TrimSpace(i.SessionID),
		AgentName:     strings.TrimSpace(i.AgentName),
		Type:          strings.TrimSpace(i.Type),
		RunID:         strings.TrimSpace(i.RunID),
		ActorKind:     strings.TrimSpace(i.ActorKind),
		ActorID:       strings.TrimSpace(i.ActorID),
		Provider:      strings.TrimSpace(i.Provider),
		Outcome:       strings.TrimSpace(i.Outcome),
		Component:     strings.TrimSpace(i.Component),
		ErrorOnly:     i.ErrorOnly,
		AfterSequence: i.AfterSequence,
		Since:         since,
		Limit:         i.Limit,
	}
	if err := query.Validate(); err != nil {
		return store.EventSummaryQuery{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"logs query is invalid",
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return query, nil
}

type observeSearchInput struct {
	Query string `json:"query"`
	logQueryInput
}

type taskReadInput struct {
	TaskID string `json:"task_id"`
}
