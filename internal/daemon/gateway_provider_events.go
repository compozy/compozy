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

type gatewayProviderAuditSink struct {
	writer  store.EventSummaryStore
	now     func() time.Time
	timeout time.Duration
}

func (s gatewayProviderAuditSink) RecordEndpointVerified(
	ctx context.Context,
	activation gateway.ProviderActivation,
	endpoint gateway.AdvertisedEndpoint,
) error {
	if err := endpoint.Validate(); err != nil {
		return fmt.Errorf("daemon: validate verified gateway endpoint: %w", err)
	}
	safeProvider := diagnostics.RedactAndBound(activation.ProviderName, maxGatewayAuditFieldBytes)
	payload, err := json.Marshal(struct {
		Provider   string `json:"provider"`
		Tier       string `json:"tier"`
		Generation uint64 `json:"generation"`
		URL        string `json:"url"`
		Scheme     string `json:"scheme"`
		Stability  string `json:"stability"`
	}{
		Provider: safeProvider, Tier: string(activation.Tier),
		Generation: activation.Generation,
		URL:        diagnostics.RedactAndBound(endpoint.URL, maxGatewayAuditFieldBytes),
		Scheme:     endpoint.Scheme, Stability: string(endpoint.Stability),
	})
	if err != nil {
		return fmt.Errorf("daemon: encode verified gateway endpoint: %w", err)
	}
	return s.record(ctx, daemonEventSummary(store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventspkg.GatewayEndpointVerified, Outcome: string(eventspkg.OutcomeSuccess),
		Provider: safeProvider,
		Summary:  "gateway provider endpoint verified",
	}, payload), "verified gateway endpoint")
}

func (s gatewayProviderAuditSink) RecordProviderStateChanged(
	ctx context.Context,
	activation gateway.ProviderActivation,
	state gateway.ProviderObservedState,
	cause string,
) error {
	safeProvider := diagnostics.RedactAndBound(activation.ProviderName, maxGatewayAuditFieldBytes)
	safeCause := diagnostics.RedactAndBound(cause, maxGatewayAuditFieldBytes)
	payload, err := json.Marshal(struct {
		Provider   string `json:"provider"`
		Tier       string `json:"tier"`
		Generation uint64 `json:"generation"`
		State      string `json:"state"`
		Cause      string `json:"cause,omitempty"`
	}{
		Provider: safeProvider, Tier: string(activation.Tier),
		Generation: activation.Generation, State: string(state), Cause: safeCause,
	})
	if err != nil {
		return fmt.Errorf("daemon: encode gateway provider state change: %w", err)
	}
	outcome := eventspkg.OutcomeInfo
	switch state {
	case gateway.ProviderUp, gateway.ProviderDown:
		outcome = eventspkg.OutcomeSuccess
	case gateway.ProviderDegraded:
		outcome = eventspkg.OutcomeFailure
	}
	return s.record(ctx, daemonEventSummary(store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventspkg.GatewayProviderStateChanged, Outcome: string(outcome),
		Provider: safeProvider,
		Summary:  "gateway provider state changed",
	}, payload), "gateway provider state change")
}

func (s gatewayProviderAuditSink) RecordProviderRefusal(
	ctx context.Context,
	activation gateway.ProviderActivation,
	kind gateway.ProviderRefusalKind,
	cause error,
) error {
	safeProvider := diagnostics.RedactAndBound(activation.ProviderName, maxGatewayAuditFieldBytes)
	reason := "provider contract refused"
	if cause != nil {
		reason = diagnostics.RedactAndBound(cause.Error(), maxGatewayAuditFieldBytes)
	}
	payload, err := json.Marshal(struct {
		Provider   string `json:"provider"`
		Tier       string `json:"tier"`
		Generation uint64 `json:"generation"`
		Kind       string `json:"kind"`
		Reason     string `json:"reason"`
	}{
		Provider: safeProvider, Tier: string(activation.Tier),
		Generation: activation.Generation, Kind: string(kind), Reason: reason,
	})
	if err != nil {
		return fmt.Errorf("daemon: encode gateway provider refusal: %w", err)
	}
	eventType := eventspkg.GatewayProviderStateChanged
	summary := "gateway provider authority refused"
	if kind == gateway.ProviderRefusalEndpoint {
		eventType = eventspkg.GatewayEndpointRejected
		summary = "gateway provider endpoint rejected"
	}
	return s.record(ctx, daemonEventSummary(store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventType, Outcome: string(eventspkg.OutcomeFailure),
		Provider: safeProvider,
		Summary:  summary,
	}, payload), "gateway provider refusal")
}

func (s gatewayProviderAuditSink) record(
	ctx context.Context,
	event store.EventSummary,
	action string,
) error {
	if s.writer == nil {
		return errors.New("daemon: gateway audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: gateway audit context is required")
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	event.Timestamp = now().UTC()
	writeCtx, cancel := s.writeContext(ctx)
	defer cancel()
	if err := s.writer.WriteEventSummary(writeCtx, event); err != nil {
		return fmt.Errorf("daemon: record %s: %w", action, err)
	}
	return nil
}

func (s gatewayProviderAuditSink) writeContext(ctx context.Context) (context.Context, context.CancelFunc) {
	timeout := s.timeout
	if timeout <= 0 {
		timeout = gatewayAuditWriteTimeout
	}
	return context.WithTimeout(context.WithoutCancel(ctx), timeout)
}

var _ gateway.ProviderAuditSink = gatewayProviderAuditSink{}
