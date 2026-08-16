package cli

import (
	"fmt"
	"strings"

	compozyupdate "github.com/compozy/compozy/internal/update"
)

func updateRecordFromState(state compozyupdate.MultiState) updateRecord {
	return updateRecord{
		Status: state.Aggregate, Runtime: state.Runtime, App: state.App, Operation: state.Operation,
	}
}

func completedUpdateRecord(
	record updateRecord,
	request compozyupdate.OperationRequest,
	runErr error,
) updateRecord {
	if runErr != nil {
		return failedUpdateRecord(record, runErr, request.Targets...)
	}
	if request.Runtime != nil {
		record.Runtime.Status = compozyupdate.StatusUpdated
		record.Runtime.CurrentVersion = request.Runtime.ToVersion
		record.Runtime.LatestVersion = request.Runtime.ToVersion
		record.Runtime.Recommendation = ""
		record.Runtime.DaemonRestarted = true
		record.Runtime.LastError = ""
		record.Runtime.Message = "Updated CompozyOS runtime to " + request.Runtime.ToVersion + " and restarted the daemon."
	}
	if request.App != nil && record.App != nil {
		record.App.Status = compozyupdate.StatusStaged
		record.App.LatestVersion = request.App.ToVersion
		record.App.AttemptID = request.App.AttemptID
		record.App.LastError = ""
		record.App.Message = "CompozyOS app update staged; applies on next launch."
	}
	record.Operation = nil
	record.Status = compozyupdate.AggregateTerminalStatus(record.Runtime.Status, appUpdateStatus(record.App))
	return record
}

func applyArchivedAppOutcome(record updateRecord, operation *compozyupdate.Operation) updateRecord {
	if operation == nil || operation.App == nil || record.App == nil {
		return record
	}
	switch operation.App.Phase {
	case compozyupdate.PhaseVerified:
		record.App.Status = compozyupdate.StatusUpdated
		record.App.CurrentVersion = operation.App.ToVersion
		record.App.LatestVersion = operation.App.ToVersion
		record.App.AttemptID = operation.App.AttemptID
		record.App.LastError = ""
		record.App.Message = "CompozyOS app is restarting on " + operation.App.ToVersion + "."
	case compozyupdate.PhaseFailed:
		record.App.Status = compozyupdate.StatusFailed
		record.App.LastError = operation.LastError
		record.App.Message = operation.LastError
	}
	record.Status = compozyupdate.AggregateTerminalStatus(record.Runtime.Status, record.App.Status)
	return record
}

func failedUpdateRecord(
	record updateRecord,
	cause error,
	targets ...compozyupdate.Target,
) updateRecord {
	message := "Update failed."
	if cause != nil {
		message = strings.TrimSpace(cause.Error())
	}
	record.Status = compozyupdate.StatusFailed
	record.Operation = nil
	switch firstUpdateTarget(targets) {
	case compozyupdate.TargetApp:
		if record.App != nil {
			record.App.Status = compozyupdate.StatusFailed
			record.App.LastError = message
			record.App.Message = message
			return record
		}
		fallthrough
	default:
		record.Runtime.Status = compozyupdate.StatusFailed
		record.Runtime.LastError = message
		record.Runtime.Message = message
	}
	return record
}

func blockedUpdateRecord(
	record updateRecord,
	operation *compozyupdate.Operation,
	targets ...compozyupdate.Target,
) updateRecord {
	record.Status = compozyupdate.StatusBlocked
	record.Operation = nil
	message := "An update operation is already in progress. Retry after it completes."
	target := firstUpdateTarget(targets)
	if operation != nil && operation.Holder != nil {
		if target == compozyupdate.TargetApp {
			message = fmt.Sprintf(
				"An update operation is already in progress (holder pid %d). Retry after it completes.",
				operation.Holder.PID,
			)
		} else {
			message = fmt.Sprintf(
				"A runtime update is already in progress (holder pid %d). Retry after it completes.",
				operation.Holder.PID,
			)
		}
	}
	if target == compozyupdate.TargetApp && record.App != nil {
		record.App.Status = compozyupdate.StatusBlocked
		record.App.Message = message
		return record
	}
	record.Runtime.Status = compozyupdate.StatusBlocked
	record.Runtime.Message = message
	return record
}

func firstUpdateTarget(targets []compozyupdate.Target) compozyupdate.Target {
	if len(targets) == 0 {
		return compozyupdate.TargetRuntime
	}
	return targets[0]
}

func appUpdateStatus(app *compozyupdate.AppTrackState) compozyupdate.Status {
	if app == nil {
		return compozyupdate.StatusUpToDate
	}
	return app.Status
}
