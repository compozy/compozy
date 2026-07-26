package core

import (
	"sort"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/doctor"
	"github.com/compozy/agh/internal/subprocess"
)

func (h *BaseHandlers) subprocessHealthAggregate(
	workspaceID string,
) contract.SubprocessHealthAggregatePayload {
	payload := contract.SubprocessHealthAggregatePayload{Status: statusStateOK}
	source, ok := h.Sessions.(doctor.SubprocessHealthSnapshotSource)
	if !ok {
		return payload
	}

	workspaceID = strings.TrimSpace(workspaceID)
	for _, snapshot := range source.SubprocessHealthSnapshots() {
		if workspaceID != "" && strings.TrimSpace(snapshot.WorkspaceID) != workspaceID {
			continue
		}
		payload.Monitored++
		if snapshot.Health.Healthy {
			payload.Healthy++
			continue
		}
		if !subprocess.HealthFailureDetected(snapshot.Health) {
			continue
		}
		payload.Status = statusStateDegraded
		payload.Unhealthy++
		payload.Sessions = append(payload.Sessions, contract.SubprocessHealthSessionPayload{
			SessionID:           strings.TrimSpace(snapshot.SessionID),
			WorkspaceID:         strings.TrimSpace(snapshot.WorkspaceID),
			AgentName:           strings.TrimSpace(snapshot.AgentName),
			ConsecutiveFailures: snapshot.Health.ConsecutiveFailures,
			LastCheckedAt:       subprocessHealthStatusTimestamp(snapshot.Health.LastCheckedAt),
			Reason: diagnostics.RedactAndBound(
				subprocess.HealthFailureReason(snapshot.Health),
				maxDiagnosticPayloadBytes,
			),
		})
	}
	sort.Slice(payload.Sessions, func(i int, j int) bool {
		return payload.Sessions[i].SessionID < payload.Sessions[j].SessionID
	})
	return payload
}

func subprocessHealthStatusTimestamp(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	timestamp := value.UTC()
	return &timestamp
}
