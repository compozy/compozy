package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aghupdate "github.com/compozy/agh/internal/update"
	"github.com/spf13/cobra"
)

const (
	updateManagedValue      = "Managed"
	updateMessageValue      = "Message"
	updateStatusValue       = "Status"
	updateCurrentVersionKey = "current_version"
	updateLatestVersionKey  = "latest_version"
	updateManagedKey        = "managed"
	updateMessageKey        = "message"
	updateStatusKey         = "status"
	updateUpdateKey         = "update"
)

const (
	defaultSettingsRestartTimeout = 45 * time.Second
	restartStatusReady            = "ready"
	restartStatusFailed           = "failed"
)

type updateManager interface {
	Check(context.Context, aghupdate.CheckOptions) (aghupdate.State, *aghupdate.Release, error)
	ApplyRelease(context.Context, *aghupdate.Release) (aghupdate.AppliedBinary, error)
	Restore(aghupdate.AppliedBinary) error
	Finalize(aghupdate.AppliedBinary) error
}

type updateRecord struct {
	Status          string `json:"status"`
	InstallMethod   string `json:"install_method"`
	Managed         bool   `json:"managed"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version,omitempty"`
	ReleaseURL      string `json:"release_url,omitempty"`
	Recommendation  string `json:"recommendation,omitempty"`
	DaemonRestarted bool   `json:"daemon_restarted"`
	Message         string `json:"message"`
}

func newUpdateCommand(deps commandDeps) *cobra.Command {
	var checkOnly bool

	cmd := &cobra.Command{
		Use:   updateUpdateKey,
		Short: "Check for and apply the latest stable AGH release",
		Long: strings.TrimSpace(`
Check GitHub Releases for the latest stable AGH build and apply it when this install supports
self-update. Managed installs return the exact package-manager upgrade path instead of mutating
files directly.
		`),
		Example: strings.TrimSpace(`
  agh update
  agh update --check
  agh update -o json
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runUpdateCommand(cmd, deps, checkOnly)
		},
	}

	cmd.Flags().BoolVar(&checkOnly, "check", false, "Check for a newer stable release without changing files")
	return cmd
}

func runUpdateCommand(cmd *cobra.Command, deps commandDeps, checkOnly bool) error {
	manager, err := resolveUpdateManager(deps)
	if err != nil {
		return err
	}

	state, release, err := manager.Check(cmd.Context(), aghupdate.CheckOptions{
		ForceRefresh:         true,
		AllowCachedOnFailure: false,
	})
	record := updateRecordFromState(state)
	if err != nil {
		return writeUpdateFailure(cmd, record, err)
	}
	if checkOnly || state.Status != aghupdate.StatusAvailable {
		return writeCommandOutput(cmd, updateBundle(record))
	}

	return applyAvailableUpdate(cmd, deps, manager, release, record)
}

func resolveUpdateManager(deps commandDeps) (updateManager, error) {
	homePaths, err := deps.resolveHome()
	if err != nil {
		return nil, err
	}
	if deps.newUpdateManager == nil {
		return nil, errors.New("cli: update manager factory is required")
	}
	return deps.newUpdateManager(homePaths)
}

func applyAvailableUpdate(
	cmd *cobra.Command,
	deps commandDeps,
	manager updateManager,
	release *aghupdate.Release,
	record updateRecord,
) error {
	runtime, running, err := resolveUpdateRuntime(deps)
	if err != nil {
		return err
	}

	applied, err := manager.ApplyRelease(cmd.Context(), release)
	if err != nil {
		record.Status = string(aghupdate.StatusFailed)
		record.Message = err.Error()
		return writeUpdateFailure(cmd, record, err)
	}
	if !running {
		return finishLocalUpdate(cmd, manager, applied, release, record)
	}
	return restartDaemonAfterUpdate(cmd, deps, manager, runtime, applied, release, record)
}

func resolveUpdateRuntime(deps commandDeps) (*runtimeContext, bool, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return nil, false, err
	}
	_, running, err := daemonInfo(runtime.HomePaths, deps)
	if err != nil {
		return nil, false, err
	}
	return runtime, running, nil
}

func finishLocalUpdate(
	cmd *cobra.Command,
	manager updateManager,
	applied aghupdate.AppliedBinary,
	release *aghupdate.Release,
	record updateRecord,
) error {
	if err := manager.Finalize(applied); err != nil {
		record.Status = string(aghupdate.StatusFailed)
		record.Message = err.Error()
		return writeUpdateFailure(cmd, record, err)
	}
	record.Status = string(aghupdate.StatusUpdated)
	record.CurrentVersion = strings.TrimSpace(release.Version)
	record.Message = "Updated AGH to " + strings.TrimSpace(release.Version) + "."
	return writeCommandOutput(cmd, updateBundle(record))
}

func restartDaemonAfterUpdate(
	cmd *cobra.Command,
	deps commandDeps,
	manager updateManager,
	runtime *runtimeContext,
	applied aghupdate.AppliedBinary,
	release *aghupdate.Release,
	record updateRecord,
) error {
	client, err := clientFromDeps(deps)
	if err != nil {
		return failAppliedUpdate(
			cmd,
			manager,
			applied,
			deps,
			runtime,
			record,
			"Updated the binary on disk, but failed to prepare daemon restart.",
			err,
			false,
		)
	}

	restartAction, err := client.TriggerSettingsRestart(cmd.Context())
	if err != nil {
		return failAppliedUpdate(
			cmd,
			manager,
			applied,
			deps,
			runtime,
			record,
			"Updated the binary on disk, but failed to trigger daemon restart.",
			err,
			false,
		)
	}

	restartStatus, err := waitForSettingsRestart(cmd.Context(), deps, client, restartAction.OperationID)
	if err != nil {
		return failAppliedUpdate(
			cmd,
			manager,
			applied,
			deps,
			runtime,
			record,
			"Updated the binary on disk, but the daemon restart did not complete successfully.",
			err,
			true,
		)
	}
	if strings.TrimSpace(string(restartStatus.Status)) != restartStatusReady {
		restartErr := fmt.Errorf("cli: daemon restart finished in unexpected state %q", restartStatus.Status)
		return failAppliedUpdate(
			cmd,
			manager,
			applied,
			deps,
			runtime,
			record,
			"Updated the binary on disk, but the daemon restart did not become ready.",
			restartErr,
			true,
		)
	}

	if err := manager.Finalize(applied); err != nil {
		record.Status = string(aghupdate.StatusFailed)
		record.Message = err.Error()
		return writeUpdateFailure(cmd, record, err)
	}
	record.Status = string(aghupdate.StatusUpdated)
	record.CurrentVersion = strings.TrimSpace(release.Version)
	record.DaemonRestarted = true
	record.Message = "Updated AGH to " + strings.TrimSpace(release.Version) + " and restarted the daemon."
	return writeCommandOutput(cmd, updateBundle(record))
}

func failAppliedUpdate(
	cmd *cobra.Command,
	manager updateManager,
	applied aghupdate.AppliedBinary,
	deps commandDeps,
	runtime *runtimeContext,
	record updateRecord,
	prefix string,
	cause error,
	attemptRecoveryStart bool,
) error {
	rollbackErr := rollbackAppliedUpdate(
		cmd.Context(),
		manager,
		applied,
		deps,
		runtime,
		attemptRecoveryStart,
	)
	record.Status = string(aghupdate.StatusFailed)
	record.Message = combineUpdateErrors(prefix, cause, rollbackErr)
	return writeUpdateFailure(cmd, record, errors.Join(cause, rollbackErr))
}

func updateRecordFromState(state aghupdate.State) updateRecord {
	return updateRecord{
		Status:         strings.TrimSpace(string(state.Status)),
		InstallMethod:  strings.TrimSpace(state.InstallMethod),
		Managed:        state.Managed,
		CurrentVersion: strings.TrimSpace(state.CurrentVersion),
		LatestVersion:  strings.TrimSpace(state.LatestVersion),
		ReleaseURL:     strings.TrimSpace(state.ReleaseURL),
		Recommendation: strings.TrimSpace(state.Recommendation),
		Message:        strings.TrimSpace(state.Message),
	}
}

func updateBundle(record updateRecord) outputBundle {
	rows := []keyValue{
		{Label: updateStatusValue, Value: stringOrDash(record.Status)},
		{Label: "Install Method", Value: stringOrDash(record.InstallMethod)},
		{Label: updateManagedValue, Value: fmt.Sprintf("%t", record.Managed)},
		{Label: "Current Version", Value: stringOrDash(record.CurrentVersion)},
		{Label: "Latest Version", Value: stringOrDash(record.LatestVersion)},
		{Label: "Release", Value: stringOrDash(record.ReleaseURL)},
		{Label: updateMessageValue, Value: stringOrDash(record.Message)},
		{Label: "Daemon Restarted", Value: fmt.Sprintf("%t", record.DaemonRestarted)},
	}
	if record.Recommendation != "" {
		rows = append(rows, keyValue{Label: "Recommendation", Value: record.Recommendation})
	}

	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			return renderHumanSection("Update", rows), nil
		},
		toon: func() (string, error) {
			order := []string{
				updateStatusKey,
				"install_method",
				updateManagedKey,
				updateCurrentVersionKey,
				updateLatestVersionKey,
				"release_url",
				"daemon_restarted",
				updateMessageKey,
			}
			values := []string{
				record.Status,
				record.InstallMethod,
				fmt.Sprintf("%t", record.Managed),
				record.CurrentVersion,
				record.LatestVersion,
				record.ReleaseURL,
				fmt.Sprintf("%t", record.DaemonRestarted),
				record.Message,
			}
			if record.Recommendation != "" {
				order = append(order, "recommendation")
				values = append(values, record.Recommendation)
			}
			return renderToonObject(
				updateUpdateKey,
				order,
				values,
			), nil
		},
	}
}

func writeUpdateFailure(cmd *cobra.Command, record updateRecord, cause error) error {
	if writeErr := writeCommandOutput(cmd, updateBundle(record)); writeErr != nil {
		return writeErr
	}
	if cause == nil {
		return errors.New(strings.TrimSpace(record.Message))
	}
	return cause
}

func waitForSettingsRestart(
	ctx context.Context,
	deps commandDeps,
	client DaemonClient,
	operationID string,
) (SettingsRestartStatusRecord, error) {
	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	if _, hasDeadline := waitCtx.Deadline(); !hasDeadline {
		timeout := settingsRestartTimeout(deps)
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(waitCtx, timeout)
		defer cancel()
	}

	ticker := time.NewTicker(deps.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return SettingsRestartStatusRecord{}, errors.New("cli: daemon restart did not complete before timeout")
		case <-ticker.C:
			status, err := client.GetSettingsRestartStatus(waitCtx, operationID)
			if err != nil {
				continue
			}
			switch strings.TrimSpace(string(status.Status)) {
			case restartStatusReady:
				return status, nil
			case restartStatusFailed:
				reason := strings.TrimSpace(status.FailureReason)
				if reason == "" {
					reason = "daemon restart failed"
				}
				return status, errors.New("cli: " + reason)
			}
		}
	}
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

func rollbackAppliedUpdate(
	ctx context.Context,
	manager updateManager,
	applied aghupdate.AppliedBinary,
	deps commandDeps,
	runtime *runtimeContext,
	attemptRecoveryStart bool,
) error {
	var combined error
	if err := manager.Restore(applied); err != nil {
		combined = errors.Join(combined, err)
	}
	if !attemptRecoveryStart || runtime == nil {
		return combined
	}
	if _, running, err := daemonInfo(runtime.HomePaths, deps); err == nil && !running {
		if _, startErr := runDaemonDetached(ctx, deps); startErr != nil {
			combined = errors.Join(combined, startErr)
		}
	}
	return combined
}

func combineUpdateErrors(prefix string, primary error, secondary error) string {
	parts := []string{strings.TrimSpace(prefix)}
	if primary != nil {
		parts = append(parts, strings.TrimSpace(primary.Error()))
	}
	if secondary != nil {
		parts = append(parts, strings.TrimSpace(secondary.Error()))
	}
	return strings.Join(parts, " ")
}
