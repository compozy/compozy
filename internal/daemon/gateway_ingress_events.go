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

func (s gatewayIngressAuditSink) RecordIngressBound(ctx context.Context, event gateway.IngressMutationEvent) error {
	return s.record(
		ctx,
		eventspkg.GatewayIngressBound,
		eventspkg.OutcomeSuccess,
		"gateway ingress bound",
		event,
	)
}

func (s gatewayIngressAuditSink) RecordIngressUnbound(ctx context.Context, event gateway.IngressMutationEvent) error {
	return s.record(
		ctx,
		eventspkg.GatewayIngressUnbound,
		eventspkg.OutcomeWarning,
		"gateway ingress unbound",
		event,
	)
}

func (s gatewayIngressAuditSink) record(
	ctx context.Context,
	eventType string,
	outcome eventspkg.Outcome,
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
			maxGatewayAuditFieldBytes,
		),
		ScopeKind: string(binding.Scope),
		WorkspaceID: diagnostics.RedactAndBound(
			binding.WorkspaceID,
			maxGatewayAuditFieldBytes,
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
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayAuditWriteTimeout)
	defer cancel()
	if err := s.writer.WriteEventSummary(writeCtx, daemonEventSummary(store.EventSummary{
		ProfileID:   store.DefaultProfileID,
		WorkspaceID: binding.WorkspaceID,
		Type:        eventType, Outcome: string(outcome),
		Summary: summary, Timestamp: now().UTC(),
		EventCorrelation: store.EventCorrelation{
			ActorKind: event.ActorKind,
			ActorID: diagnostics.RedactAndBound(
				event.ActorID,
				maxGatewayAuditFieldBytes,
			),
		},
	}, payload)); err != nil {
		return fmt.Errorf("daemon: record gateway ingress event: %w", err)
	}
	return nil
}

var _ gateway.IngressEventSink = gatewayIngressAuditSink{}
