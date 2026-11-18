package observe

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/telemetry"
	"github.com/compozy/compozy/internal/testutil"
)

// Suite: observe usage and activity metrics
// Invariant: the metrics sink sees exactly the usage observe committed and each counted activity once.
// Boundary IN: normalized ACP events through OnAgentEvent with a real global store.
// Boundary OUT: the EventMetrics sink (OTel instruments live in the daemon).
func TestObserverEventMetrics(t *testing.T) {
	t.Parallel()

	t.Run("Should report terminal usage once with the counts token_stats committed", func(t *testing.T) {
		t.Parallel()

		h, metrics := newMetricsHarness(t)
		sess := newSession("sess-metric-usage", session.StateActive, h.workspace, h.now)
		sess.Model = "claude-test"
		h.observeSessionCreated(t, sess)
		usage := &acp.TokenUsage{
			InputTokens:     new(int64(1200)),
			OutputTokens:    new(int64(340)),
			CacheReadTokens: new(int64(800)),
		}
		h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
			Type: acp.EventTypeUsage, TurnID: "turn-metric-usage", Usage: usage,
		})
		h.observer.OnAgentEvent(testutil.Context(t), sess.ID, acp.AgentEvent{
			Type: acp.EventTypeUsage, TurnID: "turn-metric-usage", Usage: usage,
		})
		h.recordUsage(t, sess.ID, usage)

		got := metrics.usageSnapshot()
		want := AcceptedUsage{
			Provider: "claude", Model: "claude-test", Agent: "coder",
			InputTokens: 1200, OutputTokens: 340, CacheReadTokens: 800,
			CostStatus: modelcatalog.CostStatusIncluded,
		}
		if len(got) != 1 || got[0] != want {
			t.Fatalf("RecordUsage calls = %+v, want once %+v", got, want)
		}
		stat := h.singleTokenStat(t, sess.ID)
		if *stat.InputTokens != got[0].InputTokens || *stat.OutputTokens != got[0].OutputTokens ||
			*stat.CacheReadTokens != got[0].CacheReadTokens {
			t.Fatalf("token_stats %+v differs from the reported usage %+v", stat, got[0])
		}
	})

	t.Run("Should not report usage that token_stats did not commit", func(t *testing.T) {
		t.Parallel()

		h, metrics := newMetricsHarness(t)
		h.observer.registry = tokenUsageFailingRegistry{Registry: h.observer.registry}
		sess := newSession("sess-metric-usage-failed", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)

		h.recordUsage(t, sess.ID, &acp.TokenUsage{OutputTokens: new(int64(5))})

		if got := metrics.usageSnapshot(); len(got) != 0 {
			t.Fatalf("RecordUsage calls = %+v, want none after a failed RecordTokenUsage", got)
		}
	})

	t.Run("Should carry each committed cost with its canonical status", func(t *testing.T) {
		t.Parallel()

		rate := 1.0
		for _, tc := range []struct {
			name       string
			authMode   compozyconfig.ProviderAuthMode
			usage      *acp.TokenUsage
			wantStatus modelcatalog.CostStatus
			wantAmount float64
			wantCurr   string
		}{
			{
				name: "Should carry an actual USD cost as actual", authMode: compozyconfig.ProviderAuthModeBoundSecret,
				usage:      &acp.TokenUsage{OutputTokens: new(int64(1)), CostAmount: new(0.25), CostCurrency: new("USD")},
				wantStatus: modelcatalog.CostStatusActual, wantAmount: 0.25, wantCurr: "USD",
			},
			{
				name: "Should carry an actual EUR cost as actual", authMode: compozyconfig.ProviderAuthModeBoundSecret,
				usage:      &acp.TokenUsage{OutputTokens: new(int64(1)), CostAmount: new(0.5), CostCurrency: new("EUR")},
				wantStatus: modelcatalog.CostStatusActual, wantAmount: 0.5, wantCurr: "EUR",
			},
			{
				name: "Should estimate a cost from the catalog rate", authMode: compozyconfig.ProviderAuthModeBoundSecret,
				usage:      &acp.TokenUsage{InputTokens: new(int64(1_000_000))},
				wantStatus: modelcatalog.CostStatusEstimated, wantAmount: 1, wantCurr: "USD",
			},
			{
				name: "Should mark native CLI usage as included", authMode: compozyconfig.ProviderAuthModeNativeCLI,
				usage:      &acp.TokenUsage{OutputTokens: new(int64(1))},
				wantStatus: modelcatalog.CostStatusIncluded,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				h, metrics := newMetricsHarness(t)
				h.observer.costCatalog = &stubCostCatalog{models: []modelcatalog.Model{{
					ProviderID: "claude", ModelID: "claude-test",
					CostInputPerMillion: &rate, CostInputSource: modelcatalog.SourceKindConfig,
				}}}
				h.observer.resolveProviderAuth = fixedProviderAuthMode(tc.authMode)
				sess := newSession("sess-metric-cost", session.StateActive, h.workspace, h.now)
				sess.Model = "claude-test"
				h.observeSessionCreated(t, sess)

				h.recordUsage(t, sess.ID, tc.usage)

				got := metrics.usageSnapshot()
				if len(got) != 1 || got[0].CostStatus != tc.wantStatus || got[0].CostAmount != tc.wantAmount ||
					got[0].CostCurrency != tc.wantCurr {
					t.Fatalf("RecordUsage calls = %+v, want status %s amount %v %s",
						got, tc.wantStatus, tc.wantAmount, tc.wantCurr)
				}
			})
		}
	})

	t.Run("Should count each tool call once per turn under its canonical tool id", func(t *testing.T) {
		t.Parallel()

		h, metrics := newMetricsHarness(t)
		sess := newSession("sess-metric-tools", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)
		ctx := testutil.Context(t)
		listed := acp.AgentEvent{Type: acp.EventTypeToolResult, TurnID: "turn-a", ToolCallID: "call-1"}.
			WithTool("compozy__session_list", nil, false)
		configured := acp.AgentEvent{Type: acp.EventTypeToolResult, TurnID: "turn-a", ToolCallID: "call-2"}.
			WithProviderToolMetadata("", "mcp__compozy-hosted-tools__compozy__config_get", "completed")

		h.observer.OnAgentEvent(ctx, sess.ID, listed)
		h.observer.OnAgentEvent(ctx, sess.ID, configured)
		h.observer.OnAgentEvent(ctx, sess.ID, listed)
		h.observer.OnAgentEvent(ctx, sess.ID, listed.WithToolResultUpdate())
		h.observer.OnAgentEvent(ctx, sess.ID, acp.AgentEvent{Type: acp.EventTypeDone, TurnID: "turn-a"})
		listed.TurnID = "turn-b"
		h.observer.OnAgentEvent(ctx, sess.ID, listed)

		want := []AcceptedEvent{
			toolEvent("compozy__session_list", "call-1", "turn-a"),
			toolEvent("compozy__config_get", "call-2", "turn-a"),
			toolEvent("compozy__session_list", "call-1", "turn-b"),
		}
		if got := metrics.eventSnapshot(); !slices.Equal(got, want) {
			t.Fatalf("RecordEvent calls = %+v, want %+v", got, want)
		}
	})

	t.Run("Should count resolved permissions and classified agent errors", func(t *testing.T) {
		t.Parallel()

		h, metrics := newMetricsHarness(t)
		sess := newSession("sess-metric-activity", session.StateActive, h.workspace, h.now)
		h.observeSessionCreated(t, sess)
		ctx := testutil.Context(t)
		for _, decision := range []acp.PermissionDecision{
			acp.PermissionDecisionPending,
			acp.PermissionDecisionAllowOnce,
			acp.PermissionDecisionRejectAlways,
			acp.PermissionDecisionCanceled,
		} {
			h.observer.OnAgentEvent(ctx, sess.ID, acp.AgentEvent{
				Type: acp.EventTypePermission, TurnID: "turn-p", Decision: string(decision),
			})
		}
		h.observer.OnAgentEvent(ctx, sess.ID, acp.AgentEvent{
			Type: acp.EventTypeError, TurnID: "turn-p", Error: "model backend crashed",
			Failure: &store.SessionFailure{Kind: store.FailurePrompt},
		})
		h.observer.OnAgentEvent(ctx, sess.ID, acp.AgentEvent{
			Type: acp.EventTypeError, TurnID: "turn-p", Failure: &store.SessionFailure{Kind: store.FailureTimeout},
		})

		want := []AcceptedEvent{
			{Kind: EventPermission, Provider: "claude", TurnID: "turn-p", Outcome: telemetry.OutcomeApproved},
			{Kind: EventPermission, Provider: "claude", TurnID: "turn-p", Outcome: telemetry.OutcomeDenied},
			{Kind: EventPermission, Provider: "claude", TurnID: "turn-p", Outcome: telemetry.OutcomeCancelled},
			{Kind: EventAgentError, Provider: "claude", TurnID: "turn-p", ErrorType: telemetry.ErrorProvider},
			{Kind: EventAgentError, Provider: "claude", TurnID: "turn-p", ErrorType: telemetry.ErrorDeadlineExceeded},
		}
		if got := metrics.eventSnapshot(); !slices.Equal(got, want) {
			t.Fatalf("RecordEvent calls = %+v, want %+v", got, want)
		}
	})

	t.Run("Should read activity gauges from live sessions without health probes", func(t *testing.T) {
		t.Parallel()

		h, _ := newMetricsHarness(t)
		h.observer.registry = listSessionsFailingRegistry{Registry: h.observer.registry}
		probes := 0
		h.observer.agentProbeSource = func(context.Context) ([]acp.ProbeTarget, error) {
			probes++
			return nil, nil
		}
		h.source.sessions = []*session.Info{
			{ID: "sess-1", AgentName: "coder", State: session.StateActive},
			{ID: "sess-2", AgentName: "coder", State: session.StateActive},
			{ID: "sess-3", AgentName: "reviewer", State: session.StateStopping},
			{ID: "sess-4", AgentName: "planner", State: session.StateStopped},
		}

		sessions, agents, err := h.observer.ActiveCounts(testutil.Context(t))
		if err != nil || sessions != 3 || agents != 2 {
			t.Fatalf("ActiveCounts() = %d, %d, %v; want 3 sessions and 2 agents", sessions, agents, err)
		}
		if uptime := h.observer.Uptime(); uptime.Hours() != 1 {
			t.Fatalf("Uptime() = %s, want 1h", uptime)
		}
		if probes != 0 {
			t.Fatalf("agent probes = %d, want none", probes)
		}
	})
}

func toolEvent(toolID, callID, turnID string) AcceptedEvent {
	return AcceptedEvent{
		Kind: EventToolCallTerminal, Provider: "claude", ToolID: toolID, ToolCallID: callID, TurnID: turnID,
		Outcome: telemetry.OutcomeOK,
	}
}

func newMetricsHarness(t *testing.T) (*harness, *recordingEventMetrics) {
	t.Helper()
	h := newHarness(t)
	metrics := &recordingEventMetrics{}
	h.observer.eventMetrics = metrics
	return h, metrics
}

type recordingEventMetrics struct {
	mu     sync.Mutex
	usage  []AcceptedUsage
	events []AcceptedEvent
}

func (r *recordingEventMetrics) RecordUsage(_ context.Context, usage AcceptedUsage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.usage = append(r.usage, usage)
}

func (r *recordingEventMetrics) RecordEvent(_ context.Context, event AcceptedEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

func (r *recordingEventMetrics) usageSnapshot() []AcceptedUsage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.usage)
}

func (r *recordingEventMetrics) eventSnapshot() []AcceptedEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.events)
}

type tokenUsageFailingRegistry struct {
	Registry
}

func (tokenUsageFailingRegistry) RecordTokenUsage(
	context.Context,
	store.TokenStatsUpdate,
	store.TokenUsageDailyUpdate,
) error {
	return errors.New("token stats unavailable")
}
