package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

const (
	updateUpdateKey               = "update"
	updateOutcomeCanceled         = "canceled"
	updateOutcomeFailed           = "failed"
	updateReleaseField            = "release"
	updateMessageLabel            = "Message"
	defaultSettingsRestartTimeout = 45 * time.Second
	restartStatusReady            = "ready"
	restartStatusFailed           = "failed"
)

type updateManager interface {
	compozyupdate.CoordinatorManager
	CheckAll(context.Context, compozyupdate.CheckOptions) (compozyupdate.MultiState, *compozyupdate.Release, error)
	PlanOperation(
		context.Context,
		compozyupdate.Actor,
		[]compozyupdate.Target,
		compozyupdate.Holder,
	) (compozyupdate.OperationRequest, error)
	AcquireOperation(context.Context, compozyupdate.OperationRequest) (*compozyupdate.Operation, error)
	OperationStore() *compozyupdate.OperationStore
}

type updateRecord struct {
	Status    compozyupdate.Status            `json:"status"`
	Runtime   compozyupdate.RuntimeTrackState `json:"runtime"`
	App       *compozyupdate.AppTrackState    `json:"app,omitempty"`
	Operation *compozyupdate.OperationView    `json:"operation,omitempty"`
}

type updateCancelRecord struct {
	Status      compozyupdate.Status  `json:"status"`
	OperationID string                `json:"operation_id,omitempty"`
	Message     string                `json:"message"`
	Holder      *compozyupdate.Holder `json:"holder,omitempty"`
}

func newUpdateCommand(deps commandDeps) *cobra.Command {
	var checkOnly bool
	var cancel bool

	cmd := &cobra.Command{
		Use:   updateUpdateKey,
		Short: "Check for and apply CompozyOS runtime and app updates",
		Example: strings.TrimSpace(`
  compozy update
  compozy update --check
  compozy update --cancel
  compozy update -o json
		`),
		Args:    cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, _ []string) error { return rejectMachineProfileFlag(cmd) },
		RunE: func(cmd *cobra.Command, _ []string) error {
			if checkOnly && cancel {
				return errors.New("cli: --check and --cancel cannot be combined")
			}
			return runUpdateCommand(cmd, deps, checkOnly, cancel)
		},
	}
	configureMachineProfileFlag(cmd, false)
	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check without changing files")
	cmd.Flags().BoolVar(&cancel, "cancel", false, "Cancel a dormant update operation")
	return cmd
}

func runUpdateCommand(cmd *cobra.Command, deps commandDeps, checkOnly bool, cancel bool) error {
	manager, homePaths, err := resolveUpdateManager(deps)
	if err != nil {
		return err
	}
	if cancel {
		return cancelUpdateOperation(cmd, manager)
	}

	state, _, checkErr := manager.CheckAll(cmd.Context(), compozyupdate.CheckOptions{
		ForceRefresh: true,
	})
	record := updateRecordFromState(state)
	if checkErr != nil {
		return writeUpdateFailure(cmd, failedUpdateRecord(record, checkErr), checkErr)
	}
	targets := applicableUpdateTargets(state)
	if checkOnly || len(targets) == 0 {
		return writeCommandOutput(cmd, updateBundle(record))
	}

	holder, err := newUpdateHolder(compozyupdate.ActorCLI, deps.now())
	if err != nil {
		return writeUpdateFailure(cmd, failedUpdateRecord(record, err, targets...), err)
	}
	request, err := manager.PlanOperation(cmd.Context(), compozyupdate.ActorCLI, targets, holder)
	if err != nil {
		return writeUpdateFailure(cmd, failedUpdateRecord(record, err, targets...), err)
	}
	operation, err := manager.AcquireOperation(cmd.Context(), request)
	if err != nil {
		if blocked, ok := errors.AsType[*compozyupdate.BlockedError](err); ok {
			record = blockedUpdateRecord(record, &blocked.Operation, targets...)
		}
		return writeUpdateFailure(cmd, record, err)
	}

	runtime := newCLIUpdateRuntime(deps, homePaths)
	coordinator, err := compozyupdate.NewCoordinator(compozyupdate.CoordinatorConfig{
		Store: manager.OperationStore(), ReleaseManager: manager, BinaryManager: manager, Runtime: runtime,
	})
	if err != nil {
		return failAcquiredUpdate(cmd, manager, operation, record, err)
	}
	runErr := coordinator.Run(cmd.Context(), operation.ID, operation.Holder.ExecutorGeneration)
	var archived *compozyupdate.Operation
	var appRunErr error
	if runErr == nil && request.App != nil && record.App != nil && record.App.Running {
		archived, appRunErr = waitForAppUpdateCompletion(
			cmd.Context(), deps, manager.OperationStore(), operation.ID, request.Deadline,
		)
	}
	record, runErr = finalizeUpdateRecord(record, request, runErr, archived, appRunErr)
	if runErr != nil {
		return writeUpdateFailure(cmd, record, runErr)
	}
	return writeCommandOutput(cmd, updateBundle(record))
}

func resolveUpdateManager(deps commandDeps) (updateManager, compozyconfig.HomePaths, error) {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return nil, compozyconfig.HomePaths{}, err
	}
	if deps.newUpdateManager == nil {
		return nil, compozyconfig.HomePaths{}, errors.New("cli: update manager factory is required")
	}
	manager, err := deps.newUpdateManager(homePaths)
	return manager, homePaths, err
}

func applicableUpdateTargets(state compozyupdate.MultiState) []compozyupdate.Target {
	targets := make([]compozyupdate.Target, 0, 2)
	if state.Runtime.Status == compozyupdate.StatusAvailable && !state.Runtime.Managed {
		targets = append(targets, compozyupdate.TargetRuntime)
	}
	if state.App != nil && state.App.Status == compozyupdate.StatusAvailable {
		targets = append(targets, compozyupdate.TargetApp)
	}
	return targets
}

func failAcquiredUpdate(
	cmd *cobra.Command,
	manager updateManager,
	operation *compozyupdate.Operation,
	record updateRecord,
	cause error,
) error {
	targets := []compozyupdate.Target(nil)
	if operation != nil && operation.Holder != nil {
		targets = append(targets, operation.ActiveTarget)
		_, transitionErr := manager.OperationStore().Transition(
			cmd.Context(), operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorCLI,
				Target: operation.ActiveTarget, Phase: compozyupdate.PhaseFailed,
				Percent: -1, LastError: cause.Error(), Outcome: updateOutcomeFailed,
			},
		)
		cause = errors.Join(cause, transitionErr)
	}
	return writeUpdateFailure(
		cmd,
		failedUpdateRecord(record, cause, targets...),
		cause,
	)
}

func cancelUpdateOperation(cmd *cobra.Command, manager updateManager) error {
	operation, err := manager.OperationStore().Read(cmd.Context())
	if err != nil {
		return err
	}
	if operation == nil {
		return writeCommandOutput(cmd, updateCancelBundle(updateCancelRecord{
			Status: compozyupdate.StatusCanceled, Message: "No update operation is active.",
		}))
	}
	canceled, err := manager.OperationStore().Transition(
		cmd.Context(), operation.ID, "", operation.Revision,
		compozyupdate.Transition{
			Kind: compozyupdate.TransitionCancel, Actor: compozyupdate.ActorCLI,
			Target: operation.ActiveTarget, Percent: -1, Outcome: updateOutcomeCanceled,
		},
	)
	if errors.Is(err, compozyupdate.ErrOperationNotCancelable) || errors.Is(err, compozyupdate.ErrExecutorFenced) {
		record := updateCancelRecord{
			Status: compozyupdate.StatusBlocked, OperationID: operation.ID, Holder: operation.Holder,
			Message: liveExecutorCancelMessage(operation),
		}
		return writeUpdateCancelFailure(cmd, record, err)
	}
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, updateCancelBundle(updateCancelRecord{
		Status: compozyupdate.StatusCanceled, OperationID: canceled.ID,
		Message: "Canceled dormant update operation; the update channel is free.",
	}))
}

func liveExecutorCancelMessage(operation *compozyupdate.Operation) string {
	if operation == nil || operation.Holder == nil {
		return "The update operation has a live executor and was not canceled."
	}
	return fmt.Sprintf(
		"Operation %s has a live executor (pid %d via %s). Not canceled.",
		operation.ID,
		operation.Holder.PID,
		operation.Holder.Surface,
	)
}

func updateBundle(record updateRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, record) },
		human: func() (string, error) {
			rows := []keyValue{
				{Label: automationStatusValue, Value: string(record.Status)},
				{
					Label: "Runtime",
					Value: updateTrackSummary(
						record.Runtime.CurrentVersion,
						record.Runtime.LatestVersion,
						record.Runtime.InstallMethod,
					),
				},
			}
			if record.App != nil {
				rows = append(
					rows,
					keyValue{
						Label: "App",
						Value: updateTrackSummary(
							record.App.CurrentVersion,
							record.App.LatestVersion,
							appRunningLabel(record.App.Running),
						),
					},
				)
			}
			if record.Runtime.ReleaseURL != "" {
				rows = append(rows, keyValue{Label: cliReleaseValue, Value: record.Runtime.ReleaseURL})
			}
			return renderHumanSection("Update", rows), nil
		},
		toon: func() (string, error) {
			fields := []string{automationStatusKey, loopRuntimeKey, updateReleaseField}
			values := []string{
				string(record.Status),
				updateTrackSummary(
					record.Runtime.CurrentVersion,
					record.Runtime.LatestVersion,
					record.Runtime.InstallMethod,
				),
				record.Runtime.ReleaseURL,
			}
			if record.App != nil {
				fields = slices.Insert(fields, 2, "app")
				values = slices.Insert(values, 2, updateTrackSummary(
					record.App.CurrentVersion,
					record.App.LatestVersion,
					appRunningLabel(record.App.Running),
				))
			}
			return renderToonObject(updateUpdateKey, fields, values), nil
		},
	}
}

func updateCancelBundle(record updateCancelRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, record) },
		human: func() (string, error) {
			return renderHumanSection("Update", []keyValue{
				{Label: automationStatusValue, Value: string(record.Status)},
				{Label: "Operation", Value: stringOrDash(record.OperationID)},
				{Label: updateMessageLabel, Value: record.Message},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				updateUpdateKey,
				[]string{automationStatusKey, "operation_id", bridgeMessageKey},
				[]string{
					string(record.Status), record.OperationID, record.Message,
				},
			), nil
		},
	}
}

func updateTrackSummary(current string, latest string, suffix string) string {
	value := stringOrDash(current)
	if latest != "" && latest != current {
		value += " → " + latest
	}
	if suffix != "" {
		value += " (" + suffix + ")"
	}
	return value
}

func appRunningLabel(running bool) string {
	if running {
		return daemonRunningStatus
	}
	return "closed"
}

func writeUpdateFailure(cmd *cobra.Command, record updateRecord, cause error) error {
	if writeErr := writeCommandOutput(cmd, updateBundle(record)); writeErr != nil {
		return writeErr
	}
	if cause == nil {
		return errors.New(record.Runtime.Message)
	}
	return cause
}

func writeUpdateCancelFailure(cmd *cobra.Command, record updateCancelRecord, cause error) error {
	if writeErr := writeCommandOutput(cmd, updateCancelBundle(record)); writeErr != nil {
		return writeErr
	}
	return cause
}

func waitForSettingsRestart(
	ctx context.Context,
	deps commandDeps,
	client settingsRestartStatusClient,
	operationID string,
) (SettingsRestartStatusRecord, error) {
	if err := requirePollingContext(ctx); err != nil {
		return SettingsRestartStatusRecord{}, err
	}
	return pollUntil(
		ctx,
		settingsRestartTimeout(deps),
		deps.pollInterval,
		nil,
		"cli: daemon restart did not complete before timeout",
		func(pollCtx context.Context, _ pollEvent) (SettingsRestartStatusRecord, bool, error) {
			status, err := client.GetSettingsRestartStatus(pollCtx, operationID)
			if err != nil {
				return SettingsRestartStatusRecord{}, false, nil
			}
			switch strings.TrimSpace(string(status.Status)) {
			case restartStatusReady:
				return status, true, nil
			case restartStatusFailed:
				reason := strings.TrimSpace(status.FailureReason)
				if reason == "" {
					reason = "daemon restart failed"
				}
				return status, true, errors.New("cli: " + reason)
			default:
				return SettingsRestartStatusRecord{}, false, nil
			}
		},
	)
}

type settingsRestartStatusClient interface {
	GetSettingsRestartStatus(context.Context, string) (SettingsRestartStatusRecord, error)
}

func settingsRestartTimeout(deps commandDeps) time.Duration {
	timeout := defaultSettingsRestartTimeout
	if deps.startTimeout > 0 && deps.stopTimeout > 0 {
		calculated := deps.startTimeout + deps.stopTimeout + (5 * time.Second)
		if calculated > timeout {
			timeout = calculated
		}
	}
	return timeout
}
