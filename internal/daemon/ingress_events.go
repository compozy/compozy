package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/store"
)

const (
	maxIngressAuditFieldBytes = 2048
	ingressAuditWriteTimeout  = 5 * time.Second
)

type ingressAuditSink struct {
	writer store.EventSummaryStore
	now    func() time.Time
}

func (s ingressAuditSink) RecordIngressBound(ctx context.Context, event ingress.MutationEvent) error {
	return s.record(
		ctx,
		eventspkg.IngressBound,
		eventspkg.OutcomeSuccess,
		"ingress bound",
		event,
	)
}

func (s ingressAuditSink) RecordIngressUnbound(ctx context.Context, event ingress.MutationEvent) error {
	return s.record(
		ctx,
		eventspkg.IngressUnbound,
		eventspkg.OutcomeWarning,
		"ingress unbound",
		event,
	)
}

func (s ingressAuditSink) record(
	ctx context.Context,
	eventType string,
	outcome eventspkg.Outcome,
	summary string,
	event ingress.MutationEvent,
) error {
	if s.writer == nil {
		return errors.New("daemon: ingress audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: ingress audit context is required")
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
			maxIngressAuditFieldBytes,
		),
		ScopeKind: string(binding.Scope),
		WorkspaceID: diagnostics.RedactAndBound(
			binding.WorkspaceID,
			maxIngressAuditFieldBytes,
		),
		EndpointGeneration: binding.EndpointGeneration,
	})
	if err != nil {
		return fmt.Errorf("daemon: encode ingress event: %w", err)
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ingressAuditWriteTimeout)
	defer cancel()
	if err := s.writer.WriteEventSummary(writeCtx, daemonEventSummary(store.EventSummary{
		ProfileID:   store.DefaultProfileID,
		WorkspaceID: binding.WorkspaceID,
		Type:        eventType, Outcome: string(outcome),
		Summary: summary, Timestamp: now().UTC(),
		ActorKind: event.ActorKind,
		ActorID: diagnostics.RedactAndBound(
			event.ActorID,
			maxIngressAuditFieldBytes,
		),
	}, payload)); err != nil {
		return fmt.Errorf("daemon: record ingress event: %w", err)
	}
	return nil
}

var _ ingress.EventSink = ingressAuditSink{}
