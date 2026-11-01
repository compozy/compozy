//go:build integration

package observe

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestObserverIntegrationFullFlow(t *testing.T) {
	h := newHarness(t)
	sess := newSession("sess-integration", session.StateActive, h.workspace, h.now)
	sess.Model = "claude-test"
	inputRate := 1.0
	outputRate := 4.0
	h.observer.costCatalog = &stubCostCatalog{models: []modelcatalog.Model{{
		ProviderID:           "claude",
		ModelID:              sess.Model,
		CostInputPerMillion:  &inputRate,
		CostOutputPerMillion: &outputRate,
		CostInputSource:      modelcatalog.SourceKindConfig,
		CostOutputSource:     modelcatalog.SourceKindConfig,
	}}}
	h.observer.resolveProviderAuth = fixedProviderAuthMode(compozyconfig.ProviderAuthModeBoundSecret)

	h.observeSessionCreated(t, sess)
	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "agent_message",
		TurnID:    "turn-int-1",
		Timestamp: h.now.Add(time.Minute),
		Text:      "assistant reply",
	})

	inputTokens := int64(1_000_000)
	outputTokens := int64(500_000)
	totalTokens := inputTokens + outputTokens
	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "done",
		TurnID:    "turn-int-1",
		Timestamp: h.now.Add(2 * time.Minute),
		Usage: &acp.TokenUsage{
			TurnID:       "turn-int-1",
			InputTokens:  &inputTokens,
			OutputTokens: &outputTokens,
			TotalTokens:  &totalTokens,
			Timestamp:    h.now.Add(2 * time.Minute),
		},
	})

	h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
		Type:      "permission",
		TurnID:    "turn-int-2",
		Timestamp: h.now.Add(3 * time.Minute),
		Action:    "session/request_permission",
		Resource:  h.workspace,
		Decision:  "allow",
	})

	sess.State = session.StateStopped
	sess.UpdatedAt = h.now.Add(4 * time.Minute)
	h.observer.OnSessionStopped(testutil.Context(t), sess)

	events, err := h.observer.QueryEvents(
		testutil.Context(t),
		store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}, SessionID: sess.ID},
	)
	if err != nil {
		t.Fatalf("QueryEvents() error = %v", err)
	}
	if got, want := len(events), 3; got != want {
		t.Fatalf("len(events) = %d, want %d", got, want)
	}

	stats, err := h.observer.QueryTokenStats(testutil.Context(t), store.TokenStatsQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryTokenStats() error = %v", err)
	}
	if got, want := len(stats), 1; got != want {
		t.Fatalf("len(stats) = %d, want %d", got, want)
	}
	if stats[0].TotalTokens == nil || *stats[0].TotalTokens != totalTokens {
		t.Fatalf("stats[0].TotalTokens = %#v, want %d", stats[0].TotalTokens, totalTokens)
	}
	if stats[0].TotalCost == nil || *stats[0].TotalCost != 3 ||
		stats[0].CostCurrency == nil || *stats[0].CostCurrency != "USD" ||
		stats[0].CostStatus != "estimated" || stats[0].CostSource != "catalog_config" {
		t.Fatalf("stats[0] cost = %#v, want estimated catalog_config USD 3", stats[0])
	}

	permissions, err := h.observer.QueryPermissionLog(testutil.Context(t), store.PermissionLogQuery{SessionID: sess.ID})
	if err != nil {
		t.Fatalf("QueryPermissionLog() error = %v", err)
	}
	if got, want := len(permissions), 1; got != want {
		t.Fatalf("len(permissions) = %d, want %d", got, want)
	}
	if permissions[0].Decision != "allow" {
		t.Fatalf("permissions[0].Decision = %q, want allow", permissions[0].Decision)
	}
}

func TestObserverIntegrationUnattendedPermissions(t *testing.T) {
	t.Parallel()
	t.Run("Should return only unattended initiators through observe", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		for _, kind := range []string{"", "human", "app", "agent_session", "automation", "daemon", "extension"} {
			sess := newSession("sess-origin-"+kind, session.StateActive, h.workspace, h.now)
			sess.InitiatorKind, sess.InitiatorID = kind, "origin-"+kind
			h.observeSessionCreated(t, sess)
			h.observer.OnAgentEvent(t.Context(), sess.ID, acp.AgentEvent{
				Type: "permission", TurnID: "turn-origin", Timestamp: h.now, Action: "session/request_permission", Resource: h.workspace, Decision: "allow",
			})
		}
		rows, err := h.observer.QueryPermissionLog(t.Context(), store.PermissionLogQuery{Unattended: true, Limit: 3})
		if err != nil || len(rows) != 3 {
			t.Fatalf("unattended permissions = %#v, %v", rows, err)
		}
		rollups, err := h.observer.QueryPermissionDailyRollups(t.Context(), store.PermissionRollupQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Unattended: true,
			Since: h.now.Add(-time.Hour), Until: h.now.Add(time.Hour),
		})
		if err != nil || len(rollups) != 1 || rollups[0].Count != 3 {
			t.Fatalf("unattended daily rollups = %#v, %v", rollups, err)
		}
		seen := make(map[string]bool)
		for _, row := range rows {
			seen[row.InitiatorKind] = true
		}
		for _, kind := range []string{"automation", "daemon", "extension"} {
			if !seen[kind] {
				t.Fatalf("missing %s initiator: %#v", kind, rows)
			}
		}
	})
}
