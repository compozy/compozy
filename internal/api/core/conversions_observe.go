package core

import (
	"maps"

	"strings"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"

	"github.com/compozy/agh/internal/diagnostics"

	observepkg "github.com/compozy/agh/internal/observe"

	"github.com/compozy/agh/internal/store"
)

// ObserveHealthPayloadFromHealth converts the observer health snapshot into the shared payload.
func ObserveHealthPayloadFromHealth(health *observepkg.Health) contract.ObserveHealthPayload {
	if health == nil {
		return contract.ObserveHealthPayload{}
	}
	return contract.ObserveHealthPayload{
		Status:             health.Status,
		UptimeSeconds:      health.UptimeSeconds,
		ActiveSessions:     health.ActiveSessions,
		ActiveAgents:       health.ActiveAgents,
		GlobalDBSizeBytes:  health.GlobalDBSizeBytes,
		SessionDBSizeBytes: health.SessionDBSizeBytes,
		Persistence:        ObservePersistenceHealthPayloadFromHealth(health.Persistence),
		Retention:          ObserveRetentionHealthPayloadFromHealth(health.Retention),
		Failures:           ObserveFailureHealthPayloadFromHealth(health.Failures),
		AgentProbes:        AgentProbeHealthPayloadsFromACP(health.AgentProbes),
		Bridges:            BridgeAggregateHealthPayloadFromObserve(health.Bridges),
		Activities:         SessionActivityHealthPayloadsFromObserve(health.Activities),
		Version:            health.Version,
	}
}

// TaskHealthPayloadFromObserve converts observer task health into the shared status payload.
func TaskHealthPayloadFromObserve(health observepkg.TaskHealth) contract.TaskHealthPayload {
	return contract.TaskHealthPayload{
		Status:                     strings.TrimSpace(health.Status),
		QueueDepthTotal:            health.QueueDepthTotal,
		OldestQueuedAt:             optionalTime(health.OldestQueuedAt),
		OldestQueueAgeMilli:        health.OldestQueueAgeMilli,
		QueueDepth:                 TaskQueueDepthPayloadsFromObserve(health.QueueDepth),
		StuckRuns:                  StuckTaskRunPayloadsFromObserve(health.StuckRuns),
		ActiveOrphanRuns:           health.ActiveOrphanRuns,
		TaskTotals:                 TaskStatusTotalPayloadsFromObserve(health.TaskTotals),
		RunTotals:                  TaskRunTotalPayloadsFromObserve(health.RunTotals),
		OwnerTotals:                TaskOwnerTotalPayloadsFromObserve(health.OwnerTotals),
		ForcedStopsSinceStart:      health.ForcedStopsSinceStart,
		DuplicateIngressSinceStart: health.DuplicateIngressSinceStart,
		ChannelMismatchSinceStart:  health.ChannelMismatchSinceStart,
		RecoverySinceStart: contract.TaskRecoveryTotalsPayload{
			Requeued:      health.RecoverySinceStart.Requeued,
			MarkedRunning: health.RecoverySinceStart.MarkedRunning,
			Failed:        health.RecoverySinceStart.Failed,
		},
	}
}

// TaskQueueDepthPayloadsFromObserve converts task queue-depth rows.
func TaskQueueDepthPayloadsFromObserve(rows []observepkg.TaskQueueDepth) []contract.TaskQueueDepthPayload {
	if len(rows) == 0 {
		return nil
	}
	payloads := make([]contract.TaskQueueDepthPayload, 0, len(rows))
	for _, row := range rows {
		payloads = append(payloads, contract.TaskQueueDepthPayload{
			ChannelID:           strings.TrimSpace(row.ChannelID),
			Count:               row.Count,
			OldestQueuedAt:      optionalTime(row.OldestQueuedAt),
			OldestQueueAgeMilli: row.OldestQueueAgeMilli,
		})
	}
	return payloads
}

// StuckTaskRunPayloadsFromObserve converts stuck task-run diagnostics.
func StuckTaskRunPayloadsFromObserve(rows []observepkg.StuckTaskRun) []contract.StuckTaskRunPayload {
	if len(rows) == 0 {
		return nil
	}
	payloads := make([]contract.StuckTaskRunPayload, 0, len(rows))
	for _, row := range rows {
		payloads = append(payloads, contract.StuckTaskRunPayload{
			TaskID:     strings.TrimSpace(row.TaskID),
			RunID:      strings.TrimSpace(row.RunID),
			Status:     strings.TrimSpace(row.Status.String()),
			OriginKind: strings.TrimSpace(string(row.OriginKind)),
			ChannelID:  strings.TrimSpace(row.ChannelID),
			SessionID:  strings.TrimSpace(row.SessionID),
			AgeMillis:  row.AgeMillis,
		})
	}
	return payloads
}

// TaskStatusTotalPayloadsFromObserve converts task status buckets.
func TaskStatusTotalPayloadsFromObserve(rows []observepkg.TaskStatusTotal) []contract.TaskStatusTotalPayload {
	if len(rows) == 0 {
		return nil
	}
	payloads := make([]contract.TaskStatusTotalPayload, 0, len(rows))
	for _, row := range rows {
		payloads = append(payloads, contract.TaskStatusTotalPayload{
			Scope:     strings.TrimSpace(string(row.Scope)),
			Status:    strings.TrimSpace(string(row.Status)),
			ChannelID: strings.TrimSpace(row.ChannelID),
			Count:     row.Count,
		})
	}
	return payloads
}

// TaskRunTotalPayloadsFromObserve converts task-run status buckets.
func TaskRunTotalPayloadsFromObserve(rows []observepkg.TaskRunTotal) []contract.TaskRunTotalPayload {
	if len(rows) == 0 {
		return nil
	}
	payloads := make([]contract.TaskRunTotalPayload, 0, len(rows))
	for _, row := range rows {
		payloads = append(payloads, contract.TaskRunTotalPayload{
			Status:     strings.TrimSpace(row.Status.String()),
			OriginKind: strings.TrimSpace(string(row.OriginKind)),
			ChannelID:  strings.TrimSpace(row.ChannelID),
			Count:      row.Count,
		})
	}
	return payloads
}

// TaskOwnerTotalPayloadsFromObserve converts task ownership buckets.
func TaskOwnerTotalPayloadsFromObserve(rows []observepkg.TaskOwnerTotal) []contract.TaskOwnerTotalPayload {
	if len(rows) == 0 {
		return nil
	}
	payloads := make([]contract.TaskOwnerTotalPayload, 0, len(rows))
	for _, row := range rows {
		payloads = append(payloads, contract.TaskOwnerTotalPayload{
			OwnerKind: strings.TrimSpace(string(row.OwnerKind)),
			OwnerRef:  strings.TrimSpace(row.OwnerRef),
			Count:     row.Count,
		})
	}
	return payloads
}

// ObservePersistenceHealthPayloadFromHealth converts persistence health into the shared payload.
func ObservePersistenceHealthPayloadFromHealth(
	health observepkg.PersistenceHealth,
) contract.ObservePersistenceHealthPayload {
	return contract.ObservePersistenceHealthPayload{
		Status:             strings.TrimSpace(health.Status),
		GlobalDBSizeBytes:  health.GlobalDBSizeBytes,
		SessionDBSizeBytes: health.SessionDBSizeBytes,
	}
}

// ObserveRetentionHealthPayloadFromHealth converts retention health into the shared payload.
func ObserveRetentionHealthPayloadFromHealth(
	health observepkg.RetentionHealth,
) contract.ObserveRetentionHealthPayload {
	return contract.ObserveRetentionHealthPayload{
		Enabled:                  health.Enabled,
		RetentionDays:            health.RetentionDays,
		SweepIntervalSeconds:     health.SweepIntervalSeconds,
		LastSweepStatus:          strings.TrimSpace(health.LastSweepStatus),
		LastSweepAt:              cloneTimePtr(health.LastSweepAt),
		LastCutoffAt:             cloneTimePtr(health.LastCutoffAt),
		LastSweepError:           strings.TrimSpace(health.LastSweepError),
		DeletedEventSummaries:    health.DeletedEventSummaries,
		DeletedTokenStats:        health.DeletedTokenStats,
		DeletedTokenUsageDaily:   health.DeletedTokenUsageDaily,
		DeletedPermissionLogRows: health.DeletedPermissionLogRows,
	}
}

// ObserveFailureHealthPayloadFromHealth converts lifecycle failure health into
// the shared payload.
func ObserveFailureHealthPayloadFromHealth(
	health observepkg.FailureHealth,
) contract.ObserveFailureHealthPayload {
	payload := contract.ObserveFailureHealthPayload{
		Status: strings.TrimSpace(health.Status),
		Total:  health.Total,
	}
	if len(health.ByKind) > 0 {
		payload.ByKind = make(map[store.FailureKind]int, len(health.ByKind))
		maps.Copy(payload.ByKind, health.ByKind)
	}
	if len(health.Recent) > 0 {
		payload.Recent = make([]contract.SessionFailureHealthPayload, 0, len(health.Recent))
		for _, failure := range health.Recent {
			payload.Recent = append(payload.Recent, contract.SessionFailureHealthPayload{
				SessionID:       strings.TrimSpace(failure.SessionID),
				AgentName:       strings.TrimSpace(failure.AgentName),
				Provider:        strings.TrimSpace(failure.Provider),
				WorkspaceID:     strings.TrimSpace(failure.WorkspaceID),
				State:           strings.TrimSpace(failure.State),
				FailureKind:     failure.FailureKind,
				Summary:         diagnostics.RedactAndBound(failure.Summary, maxDiagnosticPayloadBytes),
				CrashBundlePath: diagnostics.RedactAndBound(failure.CrashBundlePath, maxDiagnosticPayloadBytes),
				UpdatedAt:       failure.UpdatedAt,
			})
		}
	}
	return payload
}

// AgentProbeHealthPayloadsFromACP converts downstream ACP probe results into
// the shared health payload.
func AgentProbeHealthPayloadsFromACP(probes []acp.ProbeResult) []contract.AgentProbeHealthPayload {
	if len(probes) == 0 {
		return nil
	}
	payloads := make([]contract.AgentProbeHealthPayload, 0, len(probes))
	for _, probe := range probes {
		payloads = append(payloads, contract.AgentProbeHealthPayload{
			AgentName:  strings.TrimSpace(probe.AgentName),
			Provider:   strings.TrimSpace(probe.Provider),
			Command:    diagnostics.RedactAndBound(probe.Command, maxDiagnosticPayloadBytes),
			Executable: strings.TrimSpace(probe.Executable),
			Status:     strings.TrimSpace(probe.Status),
			Error:      diagnostics.RedactAndBound(probe.Error, maxDiagnosticPayloadBytes),
			CheckedAt:  probe.CheckedAt,
			DurationMS: probe.DurationMS,
		})
	}
	return payloads
}

// SessionActivityHealthPayloadsFromObserve converts observer activity health
// rows into the shared health response payload.
func SessionActivityHealthPayloadsFromObserve(
	activities []observepkg.SessionActivityHealth,
) []contract.SessionActivityHealthPayload {
	if len(activities) == 0 {
		return nil
	}
	payloads := make([]contract.SessionActivityHealthPayload, 0, len(activities))
	for _, activity := range activities {
		payloads = append(payloads, contract.SessionActivityHealthPayload{
			SessionID:          strings.TrimSpace(activity.SessionID),
			TurnID:             strings.TrimSpace(activity.TurnID),
			TurnSource:         strings.TrimSpace(activity.TurnSource),
			TurnStartedAt:      cloneTimePtr(activity.TurnStartedAt),
			LastActivityAt:     cloneTimePtr(activity.LastActivityAt),
			LastActivityKind:   strings.TrimSpace(activity.LastActivityKind),
			LastActivityDetail: strings.TrimSpace(activity.LastActivityDetail),
			CurrentTool:        strings.TrimSpace(activity.CurrentTool),
			ToolCallID:         strings.TrimSpace(activity.ToolCallID),
			LastProgressAt:     cloneTimePtr(activity.LastProgressAt),
			IterationCurrent:   activity.IterationCurrent,
			IterationMax:       activity.IterationMax,
			IdleSeconds:        activity.IdleSeconds,
			ElapsedSeconds:     activity.ElapsedSeconds,
			Status:             strings.TrimSpace(activity.Status),
			StallState:         strings.TrimSpace(activity.StallState),
			StallReason:        strings.TrimSpace(activity.StallReason),
		})
	}
	return payloads
}

// AutomationHealthPayloadFromStatus converts manager status into the shared
// additive automation health block.
func AutomationHealthPayloadFromStatus(
	enabled bool,
	status automationpkg.ManagerStatus,
) contract.AutomationHealthPayload {
	return contract.AutomationHealthPayload{
		Enabled: enabled,
		Jobs: contract.AutomationResourceStatusPayload{
			Total:   status.Jobs.Total,
			Enabled: status.Jobs.Enabled,
		},
		Triggers: contract.AutomationResourceStatusPayload{
			Total:   status.Triggers.Total,
			Enabled: status.Triggers.Enabled,
		},
		SchedulerRunning: status.SchedulerRunning,
		NextFire:         status.NextFire,
		ScheduledJobs:    AutomationSchedulerStatePayloadsFromStates(status.ScheduledJobs),
	}
}

// WebhookDeliveryPayloadFromResult converts a webhook trigger dispatch result
// into the shared response payload.
func WebhookDeliveryPayloadFromResult(result automationpkg.TriggerResult) contract.WebhookDeliveryPayload {
	return contract.WebhookDeliveryPayload{
		Matched: result.Matched,
		Runs:    RunPayloadsFromRuns(result.Runs),
	}
}
