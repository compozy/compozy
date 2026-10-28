package cli

import (
	"cmp"
	"context"
	"errors"
	"strings"

	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

func reportRecoveredUpdate(
	cmd *cobra.Command,
	deps commandDeps,
	store *compozyupdate.OperationStore,
	operation *compozyupdate.Operation,
	record updateRecord,
) error {
	archived, readErr := store.ReadArchived(cmd.Context(), operation.ID)
	if readErr != nil {
		return writeUpdateFailure(cmd, failedUpdateRecord(record, readErr, operation.Targets...), readErr)
	}
	if archived != nil {
		return writeRecoveredUpdate(cmd, record, archived)
	}
	timeout := operation.Deadline.Sub(deps.now())
	if timeout <= 0 {
		err := errors.New(
			"cli: update recovery is still unresolved; inspect `compozy update --check` before restarting the daemon",
		)
		return writeUpdateFailure(cmd, failedUpdateRecord(record, err, operation.Targets...), err)
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()
	recovered, err := pollUntil(
		ctx, timeout, deps.pollInterval, nil,
		"cli: update recovery has not completed; inspect `compozy update --check` before restarting the daemon",
		func(ctx context.Context, _ pollEvent) (*compozyupdate.Operation, bool, error) {
			archived, readErr := store.ReadArchived(ctx, operation.ID)
			if readErr != nil || archived != nil {
				return archived, true, readErr
			}
			live, readErr := store.Read(ctx)
			if readErr != nil {
				return nil, true, readErr
			}
			if live != nil && live.ID == operation.ID && live.Waiting == compozyupdate.WaitingForApp &&
				(record.App == nil || !record.App.Running) {
				return live, true, nil
			}
			return nil, false, nil
		},
	)
	if err != nil {
		return writeUpdateFailure(cmd, failedUpdateRecord(record, err, operation.Targets...), err)
	}
	return writeRecoveredUpdate(cmd, record, recovered)
}

func writeRecoveredUpdate(cmd *cobra.Command, record updateRecord, recovered *compozyupdate.Operation) error {
	record, err := recoveredUpdateRecord(record, recovered)
	if err != nil {
		return writeUpdateFailure(cmd, record, err)
	}
	return writeCommandOutput(cmd, updateBundle(record))
}

func recoveredUpdateRecord(record updateRecord, operation *compozyupdate.Operation) (updateRecord, error) {
	var outcomeErr error
	if runtime := operation.Runtime; runtime != nil {
		record.Runtime.LatestVersion = runtime.ToVersion
		record.Runtime.DaemonRestarted = runtime.DaemonRestarted
		switch runtime.Phase {
		case compozyupdate.PhaseFinalized:
			record.Runtime.Status = compozyupdate.StatusUpdated
			record.Runtime.CurrentVersion = runtime.ToVersion
			record.Runtime.LastError = ""
			record.Runtime.Message = "Update recovery completed; no manual daemon restart is needed."
		default:
			if runtime.Phase == compozyupdate.PhaseRolledBack {
				record.Runtime.RestoredVersion = runtime.FromVersion
				record.Runtime.CurrentVersion = runtime.FromVersion
			}
			message := cmp.Or(strings.TrimSpace(operation.LastError), "Runtime update recovery failed.")
			if !runtime.DaemonRestarted {
				message += " Run `compozy daemon stop && compozy daemon start` to restart the installed runtime."
			}
			record = failedUpdateRecord(record, errors.New(message), compozyupdate.TargetRuntime)
			outcomeErr = errors.New(message)
		}
	}
	record = applyArchivedAppOutcome(record, operation)
	if operation.App != nil && operation.App.Phase == compozyupdate.PhaseStaged && record.App != nil {
		record.App.Status = compozyupdate.StatusStaged
	}
	if operation.App != nil && operation.App.Phase == compozyupdate.PhaseFailed {
		outcomeErr = errors.Join(outcomeErr, errors.New(cmp.Or(operation.LastError, "App update recovery failed.")))
	}
	record.Operation = nil
	record.Status = compozyupdate.AggregateTerminalStatus(record.Runtime.Status, appUpdateStatus(record.App))
	return record, outcomeErr
}
