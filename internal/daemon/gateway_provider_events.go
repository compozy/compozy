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

const maxGatewayAuditReasonBytes = 2048

type gatewayProviderAuditSink struct {
	writer store.EventSummaryStore
	now    func() time.Time
}

func (s gatewayProviderAuditSink) RecordEndpointVerified(
	ctx context.Context,
	activation gateway.ProviderActivation,
	endpoint gateway.AdvertisedEndpoint,
) error {
	if s.writer == nil {
		return errors.New("daemon: gateway audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: gateway audit context is required")
	}
	if err := endpoint.Validate(); err != nil {
		return fmt.Errorf("daemon: validate verified gateway endpoint: %w", err)
	}
	safeProvider := diagnostics.RedactAndBound(activation.ProviderName, maxGatewayAuditReasonBytes)
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
		URL:        diagnostics.RedactAndBound(endpoint.URL, maxGatewayAuditReasonBytes),
		Scheme:     endpoint.Scheme, Stability: string(endpoint.Stability),
	})
	if err != nil {
		return fmt.Errorf("daemon: encode verified gateway endpoint: %w", err)
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	if err := s.writer.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		Type: eventspkg.GatewayEndpointVerified, Outcome: string(eventspkg.OutcomeSuccess),
		Provider: safeProvider, Content: payload,
		Summary: "gateway provider endpoint verified", Timestamp: now().UTC(),
	}); err != nil {
		return fmt.Errorf("daemon: record verified gateway endpoint: %w", err)
	}
	return nil
}

func (s gatewayProviderAuditSink) RecordProviderStateChanged(
	ctx context.Context,
	activation gateway.ProviderActivation,
	state gateway.ProviderObservedState,
	cause string,
) error {
	if s.writer == nil {
		return errors.New("daemon: gateway audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: gateway audit context is required")
	}
	safeProvider := diagnostics.RedactAndBound(activation.ProviderName, maxGatewayAuditReasonBytes)
	safeCause := diagnostics.RedactAndBound(cause, maxGatewayAuditReasonBytes)
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
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	outcome := eventspkg.OutcomeInfo
	switch state {
	case gateway.ProviderUp, gateway.ProviderDown:
		outcome = eventspkg.OutcomeSuccess
	case gateway.ProviderDegraded:
		outcome = eventspkg.OutcomeFailure
	}
	if err := s.writer.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		Type: eventspkg.GatewayProviderStateChanged, Outcome: string(outcome),
		Provider: safeProvider, Content: payload,
		Summary: "gateway provider state changed", Timestamp: now().UTC(),
	}); err != nil {
		return fmt.Errorf("daemon: record gateway provider state change: %w", err)
	}
	return nil
}

func (s gatewayProviderAuditSink) RecordProviderRefusal(
	ctx context.Context,
	activation gateway.ProviderActivation,
	kind gateway.ProviderRefusalKind,
	cause error,
) error {
	if s.writer == nil {
		return errors.New("daemon: gateway audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: gateway audit context is required")
	}
	safeProvider := diagnostics.RedactAndBound(activation.ProviderName, maxGatewayAuditReasonBytes)
	reason := "provider contract refused"
	if cause != nil {
		reason = diagnostics.RedactAndBound(cause.Error(), maxGatewayAuditReasonBytes)
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
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	eventType := eventspkg.GatewayProviderStateChanged
	summary := "gateway provider authority refused"
	if kind == gateway.ProviderRefusalEndpoint {
		eventType = eventspkg.GatewayEndpointRejected
		summary = "gateway provider endpoint rejected"
	}
	if err := s.writer.WriteEventSummary(context.WithoutCancel(ctx), store.EventSummary{
		Type: eventType, Outcome: string(eventspkg.OutcomeFailure),
		Provider: safeProvider, Content: payload,
		Summary: summary, Timestamp: now().UTC(),
	}); err != nil {
		return fmt.Errorf("daemon: record gateway provider refusal: %w", err)
	}
	return nil
}

var _ gateway.ProviderAuditSink = gatewayProviderAuditSink{}
