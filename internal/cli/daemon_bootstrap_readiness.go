package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
)

// bootstrapReadyPhase selects the daemon lifecycle phase this bootstrap reports as ready.
type bootstrapReadyPhase string

const (
	// bootstrapReadyPhaseReady waits for `status: running`, the answer every daemon gives.
	bootstrapReadyPhaseReady bootstrapReadyPhase = "ready"
	// bootstrapReadyPhaseServing also accepts a daemon that serves reads while it resumes sessions.
	bootstrapReadyPhaseServing bootstrapReadyPhase = "serving"
	// bootstrapWaitRuntimeMutation names a wait on another process's update, restart or start.
	bootstrapWaitRuntimeMutation = "runtime_mutation"
)

// bootstrapReadiness is the daemon lifecycle phase and step the bootstrap last observed.
type bootstrapReadiness struct {
	Phase string `json:"phase"`
	Step  string `json:"step,omitempty"`
}

// bootstrapWait says what the bootstrap is waiting on instead of spending a start attempt.
type bootstrapWait struct {
	Reason string `json:"reason"`
	// OwnerPID is the process holding the runtime mutation lock; omitted when it is not recorded.
	OwnerPID int `json:"owner_pid,omitzero"`
}

// bootstrapUnlaunchable marks a launch that fails the same way on every attempt.
type bootstrapUnlaunchable struct {
	cause error
}

func (e *bootstrapUnlaunchable) Error() string {
	return "cli: the CompozyOS runtime cannot be launched: " + e.cause.Error()
}

func (e *bootstrapUnlaunchable) Unwrap() error { return e.cause }

func parseBootstrapReadyPhase(value string) (bootstrapReadyPhase, error) {
	switch phase := bootstrapReadyPhase(strings.TrimSpace(value)); phase {
	case "", bootstrapReadyPhaseReady:
		return bootstrapReadyPhaseReady, nil
	case bootstrapReadyPhaseServing:
		return phase, nil
	default:
		return "", fmt.Errorf(
			"cli: --ready-phase %q is invalid; use %q or %q",
			value, bootstrapReadyPhaseReady, bootstrapReadyPhaseServing,
		)
	}
}

// bootstrapReadinessFromStatus projects the lifecycle a daemon reports; nil for older daemons.
func bootstrapReadinessFromStatus(status DaemonStatus) *bootstrapReadiness {
	if status.Readiness == nil || status.Readiness.Phase == "" {
		return nil
	}
	return &bootstrapReadiness{Phase: string(status.Readiness.Phase), Step: status.Readiness.Step}
}

// servingAs reports whether pid answered in the serving phase and this bootstrap accepts it. A
// daemon without lifecycle phases never matches, so it is awaited until `running` as before.
func (o bootstrapOptions) servingAs(status DaemonStatus, pid int) bool {
	return o.readyPhase == bootstrapReadyPhaseServing && pid > 0 && status.PID == pid &&
		status.Readiness != nil && status.Readiness.Phase == contract.DaemonReadinessServing
}

// bootstrapStartObserver sees each status the start wait reads and reports whether it is the
// readiness this bootstrap accepts before `running`.
type bootstrapStartObserver func(status DaemonStatus, childPID int) bool

// bootstrapObservedClient shows the start wait's own status reads to an observer, so readiness is
// reported without a second poller.
type bootstrapObservedClient struct {
	DaemonClient
	onStatus func(DaemonStatus)
}

func (c *bootstrapObservedClient) DaemonStatus(ctx context.Context) (DaemonStatus, error) {
	status, err := c.DaemonClient.DaemonStatus(ctx)
	if err == nil {
		c.onStatus(status)
	}
	return status, err
}

// waitForBootstrapDaemonStart is the daemon start wait with the observer attached. An accepted
// status ends the wait early; every other outcome is the start wait's own.
func waitForBootstrapDaemonStart(
	ctx context.Context,
	deps commandDeps,
	child daemonProcess,
	observe bootstrapStartObserver,
) (DaemonStatus, error) {
	if observe == nil {
		return waitForDaemonStart(ctx, deps, child)
	}
	waitCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var accepted *DaemonStatus
	newClient := deps.newClient
	deps.newClient = func(target ClientTarget) (DaemonClient, error) {
		client, err := newClient(target)
		if err != nil {
			return nil, err
		}
		return &bootstrapObservedClient{DaemonClient: client, onStatus: func(status DaemonStatus) {
			if accepted == nil && observe(status, child.PID()) {
				accepted = &status
				cancel()
			}
		}}, nil
	}
	status, err := waitForDaemonStart(waitCtx, deps, child)
	if accepted != nil {
		return *accepted, nil
	}
	return status, err
}

// observeStart reports each lifecycle phase or step change of the spawned daemon as a start event
// and accepts the serving phase when this bootstrap opted in.
func (e *bootstrapExecution) observeStart(resolution bootstrapResolution, attempt int) bootstrapStartObserver {
	var last bootstrapReadiness
	return func(status DaemonStatus, childPID int) bool {
		if status.PID != childPID {
			return false
		}
		if e.options.servingAs(status, childPID) {
			return true
		}
		readiness := bootstrapReadinessFromStatus(status)
		if readiness == nil || *readiness == last || daemonReadyAs(status, childPID) {
			return false
		}
		last = *readiness
		// Progress lines are advisory; readiness and failure still arrive as terminal events.
		if err := writeBootstrapEvent(e.cmd, bootstrapEvent{
			Type: bootstrapEventType, Phase: bootstrapPhaseStart, Status: bootstrapStatusStarted,
			Resolution: resolution, Attempt: attempt, Readiness: readiness,
			Message: "The CompozyOS runtime is starting.",
		}); err != nil {
			e.cmd.PrintErrf("cli: report runtime readiness: %v\n", err)
		}
		return false
	}
}

// bootstrapServingProbe asks the daemon directly for its lifecycle phase. daemon.json is published
// only at full readiness, so the discovery probe cannot see a daemon that already serves.
type bootstrapServingProbe struct {
	execution *bootstrapExecution
	client    DaemonClient
}

// status returns the serving daemon's status. A zero pid accepts any live daemon.
func (p *bootstrapServingProbe) status(ctx context.Context, pid int) (DaemonStatus, bool) {
	e := p.execution
	if e.options.readyPhase != bootstrapReadyPhaseServing {
		return DaemonStatus{}, false
	}
	if p.client == nil {
		client, err := clientFromDeps(e.deps)
		if err != nil {
			return DaemonStatus{}, false
		}
		p.client = client
	}
	status, err := p.client.DaemonStatus(ctx)
	if err != nil {
		return DaemonStatus{}, false
	}
	if pid == 0 {
		pid = status.PID
	}
	if !e.options.servingAs(status, pid) || !e.deps.processAlive(pid) {
		return DaemonStatus{}, false
	}
	return status, true
}

// launch starts the runtime. While another process holds the runtime mutation lock (an update, a
// restart or a concurrent start) it waits for that mutation instead of spending start attempts on
// a lock this process cannot take, bounded like the wait for a booting lock owner.
func (e *bootstrapExecution) launch(
	resolution bootstrapResolution,
	attempt int,
	runtimePath string,
) (DaemonStatus, error) {
	ctx := e.cmd.Context()
	observe := e.observeStart(resolution, attempt)
	status, err := runBootstrapDaemonDetached(ctx, e.deps, e.homePaths, runtimePath, observe)
	if !errors.Is(err, compozydaemon.ErrRuntimeMutationInProgress) {
		return status, err
	}
	ownerPID := bootstrapMutationOwnerPID(daemonStartUpdateLockPath(e.homePaths))
	message := "Another CompozyOS update or restart is in progress; waiting for it to finish."
	if ownerPID > 0 {
		message = fmt.Sprintf(
			"Another CompozyOS update or restart is in progress (pid %d); waiting for it to finish.", ownerPID,
		)
	}
	if writeErr := writeBootstrapEvent(e.cmd, bootstrapEvent{
		Type: bootstrapEventType, Phase: bootstrapPhaseStart, Status: bootstrapStatusStarted,
		Resolution: resolution, Attempt: attempt, Classification: bootstrapProbeMutationBusy,
		Wait:    &bootstrapWait{Reason: bootstrapWaitRuntimeMutation, OwnerPID: ownerPID},
		Message: message,
	}); writeErr != nil {
		return DaemonStatus{}, writeErr
	}
	timeout := e.deps.bootstrapAttachTimeout
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(e.deps.pollInterval)
	defer ticker.Stop()
	serving := bootstrapServingProbe{execution: e}
	for {
		select {
		case <-ctx.Done():
			return DaemonStatus{}, fmt.Errorf("cli: stopped waiting for the runtime mutation: %w", ctx.Err())
		case <-deadline.C:
			return DaemonStatus{}, fmt.Errorf(
				"cli: another CompozyOS update or restart did not finish within %s; "+
					"inspect the runtime log before retrying: %w",
				timeout, err,
			)
		case <-ticker.C:
		}
		if servingStatus, ok := serving.status(ctx, 0); ok {
			return servingStatus, nil
		}
		status, err = runBootstrapDaemonDetached(ctx, e.deps, e.homePaths, runtimePath, observe)
		if !errors.Is(err, compozydaemon.ErrRuntimeMutationInProgress) {
			return status, err
		}
	}
}

// bootstrapMutationOwnerPID reads the PID the runtime mutation lock holder recorded, or 0 when it
// is unknown. The payload lives in the lock file, or beside it on Windows where the lock is exclusive.
func bootstrapMutationOwnerPID(lockPath string) int {
	if runtime.GOOS == platformWindows {
		lockPath += ".owner"
	}
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return 0
	}
	var owner compozydaemon.UpdateLockOwner
	if err := json.Unmarshal(data, &owner); err != nil || owner.PID <= 0 {
		return 0
	}
	return owner.PID
}

// terminalStartFailure classifies a failed launch that no retry can change: a config file the
// runtime cannot load, a runtime mutation that outlasted the wait, or a runtime binary that is
// missing or not permitted to run. Every other failure may be transient and returns nil.
func (e *bootstrapExecution) terminalStartFailure(startErr error) (bootstrapProbeClass, error) {
	if configErr := e.configFailureAfterStart(startErr); configErr != nil {
		return bootstrapProbeConfigInvalid, configErr
	}
	if errors.Is(startErr, compozydaemon.ErrRuntimeMutationInProgress) {
		return bootstrapProbeMutationBusy, startErr
	}
	if unlaunchable, ok := errors.AsType[*bootstrapUnlaunchable](startErr); ok {
		return bootstrapProbeUnlaunchable, unlaunchable
	}
	return "", nil
}

// classifyBootstrapSpawnFailure marks a spawn that fails because the runtime binary is missing or
// not permitted to run. Only the spawn itself is classified: later failures can carry the same
// causes for transient reasons, such as a socket that does not exist yet.
func classifyBootstrapSpawnFailure(spawnErr error) error {
	if errors.Is(spawnErr, fs.ErrNotExist) || errors.Is(spawnErr, fs.ErrPermission) ||
		errors.Is(spawnErr, exec.ErrNotFound) {
		return &bootstrapUnlaunchable{cause: spawnErr}
	}
	return spawnErr
}
