package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

type workspaceAccessAuditEmitter struct {
	store store.EventSummaryStore
	now   func() time.Time
}

type workspaceAccessAuditPayload struct {
	TargetWorkspaceID string                         `json:"target_workspace_id"`
	Seam              workspaceaccess.Seam           `json:"seam"`
	Source            workspaceaccess.DecisionSource `json:"source"`
	Mode              workspaceaccess.Mode           `json:"mode"`
	Error             string                         `json:"error"`
}

var _ workspaceaccess.AuditEmitter = (*workspaceAccessAuditEmitter)(nil)

func newWorkspaceAccessAuditEmitter(
	summaryStore store.EventSummaryStore,
	now func() time.Time,
) (*workspaceAccessAuditEmitter, error) {
	if summaryStore == nil {
		return nil, errors.New("daemon: workspace access event summary store is required")
	}
	if now == nil {
		return nil, errors.New("daemon: workspace access clock is required")
	}
	return &workspaceAccessAuditEmitter{store: summaryStore, now: now}, nil
}

func (e *workspaceAccessAuditEmitter) EmitWorkspaceAccess(
	ctx context.Context,
	record workspaceaccess.AccessRecord,
) error {
	if ctx == nil {
		return errors.New("daemon: workspace access audit context is required")
	}
	eventType := eventspkg.WorkspaceAccessDenied
	summary := "workspace access denied"
	if record.Allowed {
		eventType = eventspkg.WorkspaceAccessGranted
		summary = "workspace access granted"
	}
	content, err := json.Marshal(workspaceAccessAuditPayload{
		TargetWorkspaceID: strings.TrimSpace(record.TargetID),
		Seam:              record.Seam,
		Source:            record.Source,
		Mode:              record.Mode,
		Error:             strings.TrimSpace(record.Err),
	})
	if err != nil {
		return fmt.Errorf("daemon: encode workspace access audit payload: %w", err)
	}
	if err := e.store.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		SessionID:   strings.TrimSpace(record.Actor.SessionID),
		WorkspaceID: strings.TrimSpace(record.Actor.WorkspaceID),
		Type:        eventType,
		AgentName:   strings.TrimSpace(record.Actor.AgentName),
		Outcome:     string(eventspkg.OutcomeFor(eventType)),
		Content:     content,
		EventCorrelation: store.EventCorrelation{
			ActorKind: string(record.Actor.Kind),
			ActorID:   strings.TrimSpace(record.Actor.SessionID),
		},
		Summary:   summary,
		Timestamp: e.now().UTC(),
	}); err != nil {
		return fmt.Errorf("daemon: write workspace access audit event: %w", err)
	}
	return nil
}
