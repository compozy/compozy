package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/store"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type approvalGrantEventContent struct {
	GrantID     string                         `json:"grant_id"`
	AgentName   string                         `json:"agent_name,omitempty"`
	ToolID      toolspkg.ToolID                `json:"tool_id"`
	InputDigest string                         `json:"input_digest,omitempty"`
	Decision    toolspkg.ApprovalGrantDecision `json:"decision"`
}

type toolApprovalGrantService struct {
	store  toolspkg.ApprovalGrantStore
	events store.EventSummaryStore
	logger *slog.Logger
	now    func() time.Time
}

var _ toolspkg.ApprovalGrantStore = (*toolApprovalGrantService)(nil)

func newToolApprovalGrantService(
	grantStore toolspkg.ApprovalGrantStore,
	eventStore store.EventSummaryStore,
	logger *slog.Logger,
	now func() time.Time,
) *toolApprovalGrantService {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &toolApprovalGrantService{store: grantStore, events: eventStore, logger: logger, now: now}
}

func (s *toolApprovalGrantService) LookupApprovalGrant(
	ctx context.Context,
	key toolspkg.ApprovalGrantKey,
) (toolspkg.ApprovalGrant, bool, error) {
	if s == nil || s.store == nil {
		return toolspkg.ApprovalGrant{}, false, errors.New("daemon: tool approval grant store is unavailable")
	}
	return s.store.LookupApprovalGrant(ctx, key)
}

func (s *toolApprovalGrantService) PutApprovalGrant(
	ctx context.Context,
	grant toolspkg.ApprovalGrant,
) (toolspkg.ApprovalGrant, error) {
	if s == nil || s.store == nil {
		return toolspkg.ApprovalGrant{}, errors.New("daemon: tool approval grant store is unavailable")
	}
	stored, err := s.store.PutApprovalGrant(ctx, grant)
	if err != nil {
		return toolspkg.ApprovalGrant{}, err
	}
	s.emitTransition(ctx, events.ToolApprovalGrantPut, stored)
	return stored, nil
}

func (s *toolApprovalGrantService) ListApprovalGrants(
	ctx context.Context,
	workspaceID string,
) ([]toolspkg.ApprovalGrant, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("daemon: tool approval grant store is unavailable")
	}
	return s.store.ListApprovalGrants(ctx, workspaceID)
}

func (s *toolApprovalGrantService) RevokeApprovalGrant(ctx context.Context, workspaceID, id string) error {
	if s == nil || s.store == nil {
		return errors.New("daemon: tool approval grant store is unavailable")
	}
	grants, err := s.store.ListApprovalGrants(ctx, workspaceID)
	if err != nil {
		return err
	}
	var revoked toolspkg.ApprovalGrant
	found := false
	for _, grant := range grants {
		if grant.ID == id {
			revoked = grant
			found = true
			break
		}
	}
	if !found {
		return toolspkg.ErrApprovalGrantNotFound
	}
	if err := s.store.RevokeApprovalGrant(ctx, workspaceID, id); err != nil {
		return err
	}
	s.emitTransition(ctx, events.ToolApprovalGrantRevoked, revoked)
	return nil
}

func (s *toolApprovalGrantService) emitTransition(
	ctx context.Context,
	eventType string,
	grant toolspkg.ApprovalGrant,
) {
	if s.events == nil {
		return
	}
	content, err := json.Marshal(approvalGrantEventContent{
		GrantID:     grant.ID,
		AgentName:   grant.AgentName,
		ToolID:      grant.ToolID,
		InputDigest: grant.InputDigest,
		Decision:    grant.Decision,
	})
	if err != nil {
		s.logger.Warn("daemon: marshal tool approval grant event failed", "type", eventType, "error", err)
		return
	}
	summary := fmt.Sprintf("approval grant for %s stored", grant.ToolID)
	if eventType == events.ToolApprovalGrantRevoked {
		summary = fmt.Sprintf("approval grant for %s revoked", grant.ToolID)
	}
	if err := s.events.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		WorkspaceID: grant.WorkspaceID,
		Type:        eventType,
		Outcome:     string(events.OutcomeFor(eventType)),
		Content:     content,
		Summary:     summary,
		Timestamp:   s.now().UTC(),
	}); err != nil {
		s.logger.Warn(
			"daemon: write tool approval grant event failed open",
			"type", eventType,
			"workspace_id", grant.WorkspaceID,
			"grant_id", grant.ID,
			"tool_id", grant.ToolID,
			"error", err,
		)
	}
}
