package core

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/doctor"
	"github.com/compozy/agh/internal/settings"
	"github.com/compozy/agh/internal/store"
)

// StatusSnapshot returns the same runtime status payload used by /api/status.
func (h *BaseHandlers) StatusSnapshot(ctx context.Context) (contract.StatusPayload, error) {
	return h.statusPayload(ctx, "")
}

// DoctorSnapshot returns the same diagnostic payload used by /api/doctor with all probes enabled.
func (h *BaseHandlers) DoctorSnapshot(ctx context.Context) (contract.DoctorPayload, error) {
	return h.doctorPayload(ctx, doctor.RunOptions{
		Quiet: true,
		Env: doctor.ProbeEnv{
			Now: h.nowUTC,
		},
	})
}

// ProviderListSnapshot returns the same provider inventory payload used by /api/providers.
func (h *BaseHandlers) ProviderListSnapshot(ctx context.Context) (contract.ProviderListResponse, error) {
	return h.providerListResponse(ctx)
}

// ConfigApplyRecordsSnapshot returns recent config apply records for support bundles.
func (h *BaseHandlers) ConfigApplyRecordsSnapshot(
	ctx context.Context,
) (contract.ConfigApplyRecordsResponse, error) {
	if h.Settings == nil {
		return contract.ConfigApplyRecordsResponse{}, errors.New("api: settings service is required")
	}
	records, err := h.Settings.ListApplyRecords(ctx, settings.ApplyRecordFilter{Limit: 100})
	if err != nil {
		return contract.ConfigApplyRecordsResponse{}, fmt.Errorf("api: list config apply records: %w", err)
	}
	return ConfigApplyRecordsResponseFromRecords(records), nil
}

// EventSummariesSnapshot returns recent event summaries for support bundles.
func (h *BaseHandlers) EventSummariesSnapshot(ctx context.Context) (contract.LogsListResponse, error) {
	if h.Observer == nil {
		return contract.LogsListResponse{}, errors.New("api: observer is required")
	}
	events, err := h.Observer.QueryEvents(ctx, store.EventSummaryQuery{Limit: 500})
	if err != nil {
		return contract.LogsListResponse{}, fmt.Errorf("api: query event summaries: %w", err)
	}
	payload := make([]contract.LogEventPayload, 0, len(events))
	for _, event := range events {
		payload = append(payload, LogEventPayloadFromSummary(event))
	}
	return contract.LogsListResponse{Events: payload}, nil
}

// SessionsSnapshot returns all known session payloads for support bundles.
func (h *BaseHandlers) SessionsSnapshot(ctx context.Context) (contract.SessionsResponse, error) {
	if h.Sessions == nil {
		return contract.SessionsResponse{}, errors.New("api: session manager is required")
	}
	infos, err := h.Sessions.ListAll(ctx)
	if err != nil {
		return contract.SessionsResponse{}, fmt.Errorf("api: list sessions: %w", err)
	}
	payloads := make([]contract.SessionPayload, 0, len(infos))
	for _, info := range infos {
		payloads = append(payloads, SessionPayloadFromInfo(info))
	}
	return contract.SessionsResponse{Sessions: payloads}, nil
}
