package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/gateway"
	"github.com/compozy/compozy/internal/store"
)

type gatewayIngressAuditSink struct {
	writer store.EventSummaryStore
	now    func() time.Time
}

const gatewayIngressAuditWriteTimeout = 5 * time.Second

func (s gatewayIngressAuditSink) RecordIngressBound(ctx context.Context, event gateway.IngressMutationEvent) error {
	return s.record(ctx, eventspkg.GatewayIngressBound, "gateway ingress bound", event)
}

func (s gatewayIngressAuditSink) RecordIngressUnbound(ctx context.Context, event gateway.IngressMutationEvent) error {
	return s.record(ctx, eventspkg.GatewayIngressUnbound, "gateway ingress unbound", event)
}

func (s gatewayIngressAuditSink) record(
	ctx context.Context,
	eventType string,
	summary string,
	event gateway.IngressMutationEvent,
) error {
	if s.writer == nil {
		return errors.New("daemon: gateway ingress audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: gateway ingress audit context is required")
	}
	binding := event.Binding
	payload, err := json.Marshal(struct {
		SubjectKind        string `json:"subject_kind"`
		SubjectID          string `json:"subject_id"`
		ScopeKind          string `json:"scope_kind"`
		WorkspaceID        string `json:"workspace_id,omitempty"`
		EndpointGeneration uint64 `json:"endpoint_generation"`
	}{
		SubjectKind: string(binding.Subject.Kind),
		SubjectID: diagnostics.RedactAndBound(
			binding.Subject.ID,
			maxGatewayAuditReasonBytes,
		),
		ScopeKind: string(binding.Scope),
		WorkspaceID: diagnostics.RedactAndBound(
			binding.WorkspaceID,
			maxGatewayAuditReasonBytes,
		),
		EndpointGeneration: binding.EndpointGeneration,
	})
	if err != nil {
		return fmt.Errorf("daemon: encode gateway ingress event: %w", err)
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	outcome := eventspkg.OutcomeSuccess
	if eventType == eventspkg.GatewayIngressUnbound {
		outcome = eventspkg.OutcomeWarning
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayIngressAuditWriteTimeout)
	defer cancel()
	if err := s.writer.WriteEventSummary(writeCtx, store.EventSummary{
		WorkspaceID: binding.WorkspaceID,
		Type:        eventType, Outcome: string(outcome), Content: payload,
		Summary: summary, Timestamp: now().UTC(),
		EventCorrelation: store.EventCorrelation{
			ActorKind: event.ActorKind,
			ActorID:   event.ActorID,
		},
	}); err != nil {
		return fmt.Errorf("daemon: record gateway ingress event: %w", err)
	}
	return nil
}

var _ gateway.IngressEventSink = gatewayIngressAuditSink{}
