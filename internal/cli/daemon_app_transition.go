package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/spf13/cobra"
)

const (
	appTransitionActionAcquire        = "acquire"
	appTransitionActionRecover        = "recover"
	appTransitionActionRenew          = "renew"
	appTransitionActionPhase          = "phase"
	appTransitionActionProgress       = "progress"
	appTransitionActionFence          = "fence"
	appTransitionActionVerifyArtifact = "verify-artifact"
)

type daemonAppTransitionOptions struct {
	action             string
	operationID        string
	executorGeneration string
	expectedRevision   int64
	phase              string
	percent            int
	lastError          string
	outcome            string
	attemptID          string
	artifactPath       string
	watchdogDeadline   string
	incrementFailures  bool
	resetFailures      bool
}

func newDaemonAppTransitionCommand(deps commandDeps) *cobra.Command {
	options := daemonAppTransitionOptions{percent: -1}
	cmd := &cobra.Command{
		Use:    "app-update-transition",
		Short:  "Internal desktop app update transition",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemonAppTransition(cmd, deps, options)
		},
	}
	cmd.Flags().StringVar(&options.action, "action", "", "Transition action")
	cmd.Flags().StringVar(&options.operationID, "operation-id", "", "Update operation id")
	cmd.Flags().StringVar(&options.executorGeneration, "executor-generation", "", "Update executor generation")
	cmd.Flags().Int64Var(&options.expectedRevision, "expected-revision", 0, "Expected operation revision")
	cmd.Flags().StringVar(&options.phase, "phase", "", "App operation phase")
	cmd.Flags().IntVar(&options.percent, "percent", -1, "App operation progress")
	cmd.Flags().StringVar(&options.lastError, "last-error", "", "Safe terminal error")
	cmd.Flags().StringVar(&options.outcome, "outcome", "", "Terminal outcome")
	cmd.Flags().StringVar(&options.attemptID, "attempt-id", "", "App update attempt id")
	cmd.Flags().StringVar(&options.artifactPath, "artifact-path", "", "Downloaded app artifact path")
	cmd.Flags().StringVar(&options.watchdogDeadline, "watchdog-deadline", "", "App update watchdog deadline")
	cmd.Flags().BoolVar(&options.incrementFailures, "increment-failures", false, "Increment app update failures")
	cmd.Flags().BoolVar(&options.resetFailures, "reset-failures", false, "Reset app update failures")
	for _, name := range []string{"action", "operation-id", "expected-revision"} {
		mustMarkFlagRequired(cmd, name)
	}
	return cmd
}

func runDaemonAppTransition(
	cmd *cobra.Command,
	deps commandDeps,
	options daemonAppTransitionOptions,
) error {
	if cmd == nil {
		return errors.New("cli: app update transition command is required")
	}
	homePaths, err := deps.resolveHome()
	if err != nil {
		return err
	}
	store, err := compozyupdate.NewOperationStore(homePaths, nil)
	if err != nil {
		return err
	}
	operation, err := executeDaemonAppTransition(cmd.Context(), store, deps.now(), os.Getppid(), options)
	if err != nil {
		return err
	}
	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(operation); err != nil {
		return fmt.Errorf("cli: encode app update transition: %w", err)
	}
	return nil
}

func executeDaemonAppTransition(
	ctx context.Context,
	store *compozyupdate.OperationStore,
	now time.Time,
	parentPID int,
	options daemonAppTransitionOptions,
) (*compozyupdate.Operation, error) {
	if store == nil {
		return nil, errors.New("cli: app update operation store is required")
	}
	operationID := strings.TrimSpace(options.operationID)
	generation := strings.TrimSpace(options.executorGeneration)
	watchdogDeadline, err := parseOptionalAppWatchdog(options.watchdogDeadline)
	if err != nil {
		return nil, err
	}
	execution := daemonAppTransitionExecution{
		ctx: ctx, store: store, now: now, parentPID: parentPID, options: options,
		operationID: operationID, generation: generation, watchdogDeadline: watchdogDeadline,
	}
	switch strings.TrimSpace(options.action) {
	case appTransitionActionAcquire:
		return execution.acquire()
	case appTransitionActionRecover:
		return execution.recover()
	case appTransitionActionRenew:
		return execution.renew()
	case appTransitionActionPhase:
		return execution.phase()
	case appTransitionActionProgress:
		return execution.progress()
	case appTransitionActionFence:
		return execution.fence()
	case appTransitionActionVerifyArtifact:
		return execution.verifyArtifact()
	default:
		return nil, fmt.Errorf("cli: unsupported app update transition action %q", options.action)
	}
}

type daemonAppTransitionExecution struct {
	ctx              context.Context
	store            *compozyupdate.OperationStore
	now              time.Time
	parentPID        int
	options          daemonAppTransitionOptions
	operationID      string
	generation       string
	watchdogDeadline *time.Time
}

func (e daemonAppTransitionExecution) acquire() (*compozyupdate.Operation, error) {
	if e.generation == "" {
		return nil, errors.New("cli: app update executor generation is required")
	}
	operation, err := e.store.Read(e.ctx)
	if err != nil {
		return nil, err
	}
	if !appOperationReadyForShell(operation, e.operationID, e.options.expectedRevision) {
		return nil, compozyupdate.ErrExecutorFenced
	}
	holder, err := desktopAppUpdateHolder(e.parentPID, e.generation, e.now)
	if err != nil {
		return nil, err
	}
	return e.store.Transition(e.ctx, e.operationID, "", e.options.expectedRevision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionAcquireLease, Actor: compozyupdate.ActorShell,
		Target: compozyupdate.TargetApp, Percent: -1, Holder: holder,
	})
}

func (e daemonAppTransitionExecution) recover() (*compozyupdate.Operation, error) {
	if e.generation == "" {
		return nil, errors.New("cli: app update executor generation is required")
	}
	operation, err := e.store.Read(e.ctx)
	if err != nil {
		return nil, err
	}
	if !appOperationRecoverableByShell(e.store, operation, e.operationID, e.options.expectedRevision) {
		return nil, compozyupdate.ErrExecutorFenced
	}
	holder, err := desktopAppUpdateHolder(e.parentPID, e.generation, e.now)
	if err != nil {
		return nil, err
	}
	return e.store.Transition(e.ctx, e.operationID, "", e.options.expectedRevision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionRecover, Actor: compozyupdate.ActorShell,
		Target: compozyupdate.TargetApp, Percent: operation.Percent, Holder: holder,
	})
}

func (e daemonAppTransitionExecution) renew() (*compozyupdate.Operation, error) {
	operation, err := e.store.Read(e.ctx)
	if err != nil {
		return nil, err
	}
	if operation == nil || operation.Holder == nil {
		return nil, compozyupdate.ErrOperationNotFound
	}
	holder := *operation.Holder
	holder.LeaseExpiresAt = e.now.UTC().Add(compozyupdate.DefaultLeaseDuration)
	return e.store.Transition(e.ctx, e.operationID, e.generation, e.options.expectedRevision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionRenewLease, Actor: compozyupdate.ActorShell,
		Target: compozyupdate.TargetApp, Percent: operation.Percent, Holder: &holder,
	})
}

func (e daemonAppTransitionExecution) phase() (*compozyupdate.Operation, error) {
	return e.store.Transition(e.ctx, e.operationID, e.generation, e.options.expectedRevision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorShell,
		Target: compozyupdate.TargetApp, Phase: compozyupdate.OperationPhase(strings.TrimSpace(e.options.phase)),
		Percent: e.options.percent, LastError: e.options.lastError, Outcome: e.options.outcome,
		AppWatchdogDeadline:  e.watchdogDeadline,
		IncrementAppFailures: e.options.incrementFailures,
		ResetAppFailures:     e.options.resetFailures,
	})
}

func (e daemonAppTransitionExecution) progress() (*compozyupdate.Operation, error) {
	return e.store.Transition(e.ctx, e.operationID, e.generation, e.options.expectedRevision, compozyupdate.Transition{
		Kind: compozyupdate.TransitionProgress, Actor: compozyupdate.ActorShell,
		Target: compozyupdate.TargetApp, Percent: e.options.percent,
	})
}

func (e daemonAppTransitionExecution) fence() (*compozyupdate.Operation, error) {
	if err := e.store.Fence(e.ctx, e.operationID, e.generation, e.options.expectedRevision); err != nil {
		return nil, err
	}
	return e.store.Read(e.ctx)
}

func (e daemonAppTransitionExecution) verifyArtifact() (*compozyupdate.Operation, error) {
	if err := e.store.VerifyAppArtifact(
		e.ctx,
		e.operationID,
		e.generation,
		e.options.expectedRevision,
		strings.TrimSpace(e.options.attemptID),
		strings.TrimSpace(e.options.artifactPath),
	); err != nil {
		return nil, err
	}
	return e.store.Read(e.ctx)
}

func parseOptionalAppWatchdog(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if value == "clear" {
		deadline := time.Time{}
		return &deadline, nil
	}
	deadline, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, fmt.Errorf("cli: parse app update watchdog deadline: %w", err)
	}
	deadline = deadline.UTC()
	return &deadline, nil
}

func desktopAppUpdateHolder(parentPID int, generation string, now time.Time) (*compozyupdate.Holder, error) {
	startedAt, err := procutil.StartedAt(parentPID)
	if err != nil {
		return nil, fmt.Errorf("cli: resolve desktop app process identity: %w", err)
	}
	return &compozyupdate.Holder{
		PID: parentPID, PIDStartTime: startedAt, Surface: compozyupdate.ActorShell,
		ExecutorGeneration: generation, LeaseExpiresAt: now.UTC().Add(compozyupdate.DefaultLeaseDuration),
	}, nil
}

func appOperationReadyForShell(operation *compozyupdate.Operation, operationID string, expectedRevision int64) bool {
	return operation != nil &&
		operation.ID == operationID &&
		operation.Revision == expectedRevision &&
		operation.Waiting == compozyupdate.WaitingForApp &&
		operation.ActiveTarget == "" &&
		operation.Holder == nil &&
		operation.App != nil &&
		operation.App.Phase == compozyupdate.PhaseStaged
}

func appOperationRecoverableByShell(
	store *compozyupdate.OperationStore,
	operation *compozyupdate.Operation,
	operationID string,
	expectedRevision int64,
) bool {
	if operation == nil || operation.ID != operationID || operation.Revision != expectedRevision ||
		operation.ActiveTarget != compozyupdate.TargetApp || operation.App == nil ||
		(operation.App.Phase != compozyupdate.PhaseStaged && operation.App.Phase != compozyupdate.PhaseApplying &&
			operation.App.Phase != compozyupdate.PhaseInstallerHandoff) {
		return false
	}
	return operation.Holder == nil || !store.HolderLive(operation.Holder)
}
