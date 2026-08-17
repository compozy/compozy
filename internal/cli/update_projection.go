package cli

import (
	"errors"
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

func finalizeUpdateRecord(
	record updateRecord,
	request compozyupdate.OperationRequest,
	runErr error,
	archived *compozyupdate.Operation,
	appRunErr error,
) (updateRecord, error) {
	record = completedUpdateRecord(record, request, runErr)
	record = applyArchivedAppOutcome(record, archived)
	if appRunErr != nil && archived == nil {
		record = failedUpdateRecord(record, appRunErr, compozyupdate.TargetApp)
	}
	return record, errors.Join(runErr, appRunErr)
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
	target := firstUpdateTarget(targets)
	if errorTarget, ok := compozyupdate.ErrorTarget(cause); ok {
		target = errorTarget
	}
	switch target {
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
	target := blockedOperationTarget(operation, targets)
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

func blockedOperationTarget(
	operation *compozyupdate.Operation,
	targets []compozyupdate.Target,
) compozyupdate.Target {
	if operation != nil {
		if operation.ActiveTarget != "" {
			return operation.ActiveTarget
		}
		if operation.Waiting == compozyupdate.WaitingForApp {
			return compozyupdate.TargetApp
		}
	}
	return firstUpdateTarget(targets)
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
