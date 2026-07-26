package core

import (
	"context"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/store"
)

// SchemaStreamStatusReader reads the daemon-global migration streams exposed by status.
type SchemaStreamStatusReader interface {
	SchemaStreamStatuses(context.Context) ([]store.StreamStatus, error)
}

func (h *BaseHandlers) schemaStreamStatusPayloads(ctx context.Context) ([]contract.SchemaStreamStatus, error) {
	payloads := make([]contract.SchemaStreamStatus, 0)
	if h.SchemaStreams == nil {
		return payloads, nil
	}

	statuses, err := h.SchemaStreams.SchemaStreamStatuses(ctx)
	if err != nil {
		return nil, err
	}
	payloads = make([]contract.SchemaStreamStatus, 0, len(statuses))
	for _, status := range statuses {
		payloads = append(payloads, contract.SchemaStreamStatus{
			Stream:       status.Stream,
			Version:      status.Version,
			AppliedCount: status.AppliedCount,
			SumDigest:    status.SumDigest,
		})
	}
	return payloads, nil
}

func (h *BaseHandlers) daemonStatusPayload(
	health *observe.Health,
	totalSessions int,
	networkStatus *contract.NetworkStatusPayload,
	schemaStreams []contract.SchemaStreamStatus,
) contract.DaemonStatusPayload {
	activeSessions := 0
	version := ""
	if health != nil {
		activeSessions = health.ActiveSessions
		version = health.Version
	}
	httpPort := h.HTTPPortValue()
	if httpPort <= 0 {
		httpPort = h.Config.HTTP.Port
	}
	daemonStatus := statusStateRunning
	if h.DrainController != nil && h.DrainController.DrainState() == contract.DrainStateDraining {
		daemonStatus = string(contract.DrainStateDraining)
	}
	return contract.DaemonStatusPayload{
		Status:         daemonStatus,
		PID:            h.PID(),
		StartedAt:      h.StartedAt,
		Socket:         h.Config.Daemon.Socket,
		HTTPHost:       h.Config.HTTP.Host,
		HTTPPort:       httpPort,
		UserHomeDir:    h.daemonUserHomeDir(),
		ActiveSessions: activeSessions,
		TotalSessions:  totalSessions,
		Version:        version,
		Network:        networkStatus,
		SchemaStreams:  schemaStreams,
	}
}
