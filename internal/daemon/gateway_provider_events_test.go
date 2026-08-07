package daemon

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestGatewayProviderAuditSink(t *testing.T) {
	t.Parallel()

	t.Run("Should durably record a redacted provider contract refusal [UT-076]", func(t *testing.T) {
		t.Parallel()
		writer := &gatewayAuditWriter{}
		sink := gatewayProviderAuditSink{
			writer: writer,
			now:    func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) },
		}
		if err := sink.RecordProviderRefusal(testutil.Context(t), gateway.ProviderActivation{
			ProviderName: "connectivity-test", Tier: gateway.TierPrivate, Generation: 7,
		}, gateway.ProviderRefusalAuthority, errors.New("api_key=sk-gateway-audit-secret")); err != nil {
			t.Fatalf("RecordProviderRefusal() error = %v", err)
		}
		if len(writer.summaries) != 1 {
			t.Fatalf("event count = %d, want 1", len(writer.summaries))
		}
		summary := writer.summaries[0]
		if summary.Type != eventspkg.GatewayProviderStateChanged ||
			summary.Outcome != string(eventspkg.OutcomeFailure) ||
			summary.Provider != "connectivity-test" {
			t.Fatalf("event summary = %#v", summary)
		}
		if strings.Contains(string(summary.Content), "sk-gateway-audit-secret") {
			t.Fatalf("event content leaked provider secret: %s", summary.Content)
		}
	})

	t.Run("Should durably record a verified provider endpoint before publication", func(t *testing.T) {
		t.Parallel()
		writer := &gatewayAuditWriter{}
		sink := gatewayProviderAuditSink{
			writer: writer,
			now:    func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) },
		}
		if err := sink.RecordEndpointVerified(testutil.Context(t), gateway.ProviderActivation{
			ProviderName: "connectivity-test", Tier: gateway.TierPublic, Generation: 9,
		}, gateway.AdvertisedEndpoint{
			URL: "https://public.example.test", Scheme: "https", Stability: gateway.EndpointStable,
		}); err != nil {
			t.Fatalf("RecordEndpointVerified() error = %v", err)
		}
		if len(writer.summaries) != 1 {
			t.Fatalf("event count = %d, want 1", len(writer.summaries))
		}
		summary := writer.summaries[0]
		if summary.Type != eventspkg.GatewayEndpointVerified ||
			summary.Outcome != string(eventspkg.OutcomeSuccess) ||
			summary.Provider != "connectivity-test" ||
			!strings.Contains(string(summary.Content), "https://public.example.test") {
			t.Fatalf("event summary = %#v", summary)
		}
	})

	t.Run("Should durably record provider lifecycle state with a redacted cause", func(t *testing.T) {
		t.Parallel()
		writer := &gatewayAuditWriter{}
		sink := gatewayProviderAuditSink{writer: writer}
		if err := sink.RecordProviderStateChanged(testutil.Context(t), gateway.ProviderActivation{
			ProviderName: "connectivity-test", Tier: gateway.TierPrivate, Generation: 11,
		}, gateway.ProviderDegraded, "api_key=sk-gateway-state-secret"); err != nil {
			t.Fatalf("RecordProviderStateChanged() error = %v", err)
		}
		if len(writer.summaries) != 1 {
			t.Fatalf("event count = %d, want 1", len(writer.summaries))
		}
		summary := writer.summaries[0]
		if summary.Type != eventspkg.GatewayProviderStateChanged ||
			summary.Outcome != string(eventspkg.OutcomeFailure) ||
			!strings.Contains(string(summary.Content), `"state":"degraded"`) {
			t.Fatalf("event summary = %#v", summary)
		}
		if strings.Contains(string(summary.Content), "sk-gateway-state-secret") {
			t.Fatalf("event content leaked provider credential: %s", summary.Content)
		}
	})
}

func TestGatewayIngressAuditSink(t *testing.T) {
	t.Parallel()

	t.Run("Should record redacted bind and warning unbind lifecycle events", func(t *testing.T) {
		t.Parallel()
		writer := &gatewayAuditWriter{}
		sink := gatewayIngressAuditSink{
			writer: writer,
			now:    func() time.Time { return time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC) },
		}
		binding := gateway.IngressBinding{
			Subject: gateway.IngressSubjectRef{
				Kind: gateway.IngressSubjectBridgeInstance,
				ID:   "api_key=sk-ingress-audit-secret",
			},
			Scope: gateway.IngressScopeWorkspace, WorkspaceID: "ws-1", EndpointGeneration: 7,
		}
		event := gateway.IngressMutationEvent{
			Binding: binding, ActorKind: string(gateway.ActorKindOperatorDevice), ActorID: "device-1",
		}
		if err := sink.RecordIngressBound(testutil.Context(t), event); err != nil {
			t.Fatalf("RecordIngressBound() error = %v", err)
		}
		if err := sink.RecordIngressUnbound(testutil.Context(t), event); err != nil {
			t.Fatalf("RecordIngressUnbound() error = %v", err)
		}
		if len(writer.summaries) != 2 {
			t.Fatalf("event count = %d, want 2", len(writer.summaries))
		}
		bound, unbound := writer.summaries[0], writer.summaries[1]
		if bound.Type != eventspkg.GatewayIngressBound ||
			bound.Outcome != string(eventspkg.OutcomeSuccess) || bound.WorkspaceID != "ws-1" {
			t.Fatalf("bound summary = %#v", bound)
		}
		if unbound.Type != eventspkg.GatewayIngressUnbound ||
			unbound.Outcome != string(eventspkg.OutcomeWarning) || unbound.WorkspaceID != "ws-1" {
			t.Fatalf("unbound summary = %#v", unbound)
		}
		if bound.ActorKind != string(gateway.ActorKindOperatorDevice) || bound.ActorID != "device-1" ||
			unbound.ActorKind != bound.ActorKind || unbound.ActorID != bound.ActorID {
			t.Fatalf(
				"ingress event actors = bound:%s/%s unbound:%s/%s",
				bound.ActorKind,
				bound.ActorID,
				unbound.ActorKind,
				unbound.ActorID,
			)
		}
		for _, summary := range writer.summaries {
			if strings.Contains(string(summary.Content), "sk-ingress-audit-secret") {
				t.Fatalf("event content leaked ingress secret: %s", summary.Content)
			}
		}
	})
}

type gatewayAuditWriter struct {
	summaries []store.EventSummary
}

func (w *gatewayAuditWriter) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	w.summaries = append(w.summaries, summary)
	return nil
}

func (w *gatewayAuditWriter) ListEventSummaries(
	context.Context,
	store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return append([]store.EventSummary(nil), w.summaries...), nil
}
