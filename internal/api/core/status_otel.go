package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/telemetry"
)

// TelemetryStatusSource reports OTLP export status; ok is false until telemetry has started.
type TelemetryStatusSource func() (status telemetry.Status, ok bool)

func (h *BaseHandlers) otelStatusPayload() *contract.OTelStatusPayload {
	if h.TelemetryStatus == nil {
		return nil
	}
	status, ok := h.TelemetryStatus()
	if !ok {
		return nil
	}
	return &contract.OTelStatusPayload{
		State:              string(status.State),
		Protocol:           status.Protocol,
		EndpointHost:       status.EndpointHost,
		SampleRatio:        status.SampleRatio,
		ForwardToProviders: status.ForwardToProviders,
		Traces:             otelSignalStatusPayload(status.Traces),
		Metrics:            otelSignalStatusPayload(status.Metrics),
		DroppedSpans:       status.DroppedSpans,
	}
}

func otelSignalStatusPayload(signal telemetry.SignalStatus) contract.OTelSignalStatusPayload {
	return contract.OTelSignalStatusPayload{
		Enabled:        signal.Enabled,
		LastExportAt:   signal.LastExportAt,
		LastErrorClass: signal.LastErrorClass,
		FailedExports:  signal.FailedExports,
	}
}
