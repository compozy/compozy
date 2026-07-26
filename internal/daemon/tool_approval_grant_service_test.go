package daemon

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/events"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestToolApprovalGrantServiceEmitsCanonicalTransitions(t *testing.T) {
	t.Parallel()

	grantStore := &recordingApprovalGrantStore{}
	eventStore := &recordingToolEventSummaryStore{}
	now := time.Date(2026, time.July, 15, 15, 0, 0, 0, time.UTC)
	service := newToolApprovalGrantService(grantStore, eventStore, nil, func() time.Time { return now })

	t.Run("Should emit one put event after durable storage", func(t *testing.T) {
		stored, err := service.PutApprovalGrant(t.Context(), toolspkg.ApprovalGrant{
			ApprovalGrantKey: toolspkg.ApprovalGrantKey{
				WorkspaceID: "ws-1",
				AgentName:   "codex",
				ToolID:      "agh__approval_probe",
				InputDigest: "sha256:abc",
			},
			Decision: toolspkg.ApprovalGrantAllow,
		})
		if err != nil {
			t.Fatalf("PutApprovalGrant() error = %v", err)
		}
		if len(eventStore.summaries) != 1 {
			t.Fatalf("event summaries = %d, want 1", len(eventStore.summaries))
		}
		summary := eventStore.summaries[0]
		if summary.Type != events.ToolApprovalGrantPut || summary.WorkspaceID != "ws-1" ||
			summary.Outcome != string(events.OutcomeSuccess) || !summary.Timestamp.Equal(now) {
			t.Fatalf("put event summary = %#v", summary)
		}
		var content approvalGrantEventContent
		if err := json.Unmarshal(summary.Content, &content); err != nil {
			t.Fatalf("Unmarshal(summary.Content) error = %v", err)
		}
		if content.GrantID != stored.ID || content.ToolID != stored.ToolID ||
			content.Decision != toolspkg.ApprovalGrantAllow {
			t.Fatalf("put event content = %#v, want stored grant identity", content)
		}
	})

	t.Run("Should emit one revoke event and no event for a repeated revoke", func(t *testing.T) {
		grants, err := service.ListApprovalGrants(t.Context(), "ws-1")
		if err != nil || len(grants) != 1 {
			t.Fatalf("ListApprovalGrants() = %#v, %v, want one grant", grants, err)
		}
		if err := service.RevokeApprovalGrant(t.Context(), "ws-1", grants[0].ID); err != nil {
			t.Fatalf("RevokeApprovalGrant() error = %v", err)
		}
		if len(eventStore.summaries) != 2 || eventStore.summaries[1].Type != events.ToolApprovalGrantRevoked {
			t.Fatalf("event summaries = %#v, want one put and one revoke", eventStore.summaries)
		}
		err = service.RevokeApprovalGrant(t.Context(), "ws-1", grants[0].ID)
		if !errors.Is(err, toolspkg.ErrApprovalGrantNotFound) {
			t.Fatalf("repeated RevokeApprovalGrant() error = %v, want ErrApprovalGrantNotFound", err)
		}
		if len(eventStore.summaries) != 2 {
			t.Fatalf("event summaries after repeated revoke = %d, want 2", len(eventStore.summaries))
		}
	})
}
