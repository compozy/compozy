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

type gatewayDeviceAuditSink struct {
	writer  store.EventSummaryStore
	now     func() time.Time
	timeout time.Duration
}

func (s gatewayDeviceAuditSink) RecordDeviceRemoved(
	ctx context.Context,
	event gateway.DeviceMutationEvent,
) error {
	if s.writer == nil {
		return errors.New("daemon: gateway device audit store is required")
	}
	if ctx == nil {
		return errors.New("daemon: gateway device audit context is required")
	}
	device := event.Device
	payload, err := json.Marshal(struct {
		DeviceID      string `json:"device_id"`
		Name          string `json:"name"`
		ActorKind     string `json:"actor_kind"`
		PairingOrigin string `json:"pairing_origin"`
		RemovedAt     string `json:"removed_at"`
	}{
		DeviceID:  diagnostics.RedactAndBound(device.ID, maxGatewayAuditFieldBytes),
		Name:      diagnostics.RedactAndBound(device.Name, maxGatewayAuditFieldBytes),
		ActorKind: string(device.ActorKind),
		PairingOrigin: diagnostics.RedactAndBound(
			device.PairingOrigin,
			maxGatewayAuditFieldBytes,
		),
		RemovedAt: event.RemovedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("daemon: encode gateway device removed event: %w", err)
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	timeout := s.timeout
	if timeout <= 0 {
		timeout = gatewayAuditWriteTimeout
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	if err := s.writer.WriteEventSummary(writeCtx, daemonEventSummary(store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventspkg.GatewayDeviceRemoved, Outcome: string(eventspkg.OutcomeWarning),
		Summary:   "gateway device record removed",
		ActorKind: event.ActorKind,
		ActorID:   diagnostics.RedactAndBound(event.ActorID, maxGatewayAuditFieldBytes),
		Timestamp: now().UTC(),
	}, payload)); err != nil {
		return fmt.Errorf("daemon: record gateway device removed event: %w", err)
	}
	return nil
}

var _ gateway.DeviceEventSink = gatewayDeviceAuditSink{}
