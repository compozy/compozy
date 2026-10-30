//go:build darwin || linux

package procutil

import (
	"context"
	"errors"
	"os"
	"syscall"
	"time"
)

const (
	terminationFailed   = "failed"
	terminationUnproven = "unproven"
)

type ProcessTermination struct {
	Outcome string
	Err     error
}

// TerminateScannedProcess never uses the launch-time skew tolerance for user-selected process identities.
func TerminateScannedProcess(ctx context.Context, target ScannedProcess, grace time.Duration) ProcessTermination {
	return terminateObservedProcess(ctx, target, grace, false)
}

// TerminateObservedProcess accepts a durable lineage identity verified by the session process service.
// Lineage authority permits only an individual PID signal, never an inferred group signal.
func TerminateObservedProcess(ctx context.Context, target ScannedProcess, grace time.Duration) ProcessTermination {
	return terminateObservedProcess(ctx, target, grace, true)
}

func terminateObservedProcess(
	ctx context.Context,
	target ScannedProcess,
	grace time.Duration,
	lineage bool,
) ProcessTermination {
	if target.PID <= 1 || target.PID == os.Getpid() || target.PGID == syscall.Getpgrp() || target.SessionID == "" {
		return ProcessTermination{Outcome: terminationFailed, Err: errors.New("protected or unattributed process")}
	}
	observed, err := scanProcessIdentity(target.PID)
	if errors.Is(err, ErrProcessIdentityGone) || (err == nil && observed.Zombie) {
		return ProcessTermination{Outcome: "already_exited"}
	}
	if err != nil {
		return ProcessTermination{Outcome: terminationUnproven, Err: err}
	}
	if !observed.StartedAt.Equal(target.StartedAt) {
		return ProcessTermination{Outcome: "identity_mismatch"}
	}
	snapshot, err := ScanProcesses(ctx)
	if err != nil {
		return ProcessTermination{Outcome: terminationUnproven, Err: err}
	}
	members, signalPID, valid := scannedSignalTarget(snapshot, target, lineage)
	if !valid {
		return ProcessTermination{Outcome: terminationUnproven, Err: errors.New("process attribution changed")}
	}
	// Recheck the anchor after snapshot acquisition, immediately before sending a signal.
	observed, err = scanProcessIdentity(target.PID)
	if errors.Is(err, ErrProcessIdentityGone) {
		return ProcessTermination{Outcome: "already_exited"}
	}
	if err != nil {
		return ProcessTermination{Outcome: terminationUnproven, Err: err}
	}
	if !observed.StartedAt.Equal(target.StartedAt) {
		return ProcessTermination{Outcome: "identity_mismatch"}
	}
	if observed.PGID == syscall.Getpgrp() {
		return ProcessTermination{Outcome: terminationFailed, Err: errors.New("process moved into protected group")}
	}
	if observed.PGID != target.PGID {
		members, signalPID = []ScannedProcess{target}, target.PID
	}
	if err := syscall.Kill(signalPID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return ProcessTermination{Outcome: terminationFailed, Err: err}
	}
	return awaitGracefulScannedExit(ctx, members, grace)
}

func scannedSignalTarget(snapshot ProcessSnapshot, target ScannedProcess, lineage bool) ([]ScannedProcess, int, bool) {
	attributed := false
	for _, p := range snapshot.Processes {
		if p.PID == target.PID && p.StartedAt.Equal(target.StartedAt) && p.SessionID == target.SessionID &&
			p.Home == target.Home {
			attributed = true
			break
		}
	}
	if !attributed && !lineage {
		return nil, 0, false
	}
	members := []ScannedProcess{target}
	signalPID := target.PID
	// A group leader is an anchor; unreadable or foreign members force a PID-only signal.
	if !lineage && target.PID == target.PGID && target.PGID > 1 {
		safe := true
		group := []ScannedProcess{}
		for _, p := range snapshot.Processes {
			if p.PGID != target.PGID || p.Zombie {
				continue
			}
			if p.SessionID != target.SessionID || p.Home != target.Home || p.PID == os.Getpid() {
				safe = false
				break
			}
			group = append(group, p)
		}
		if safe && len(group) > 0 {
			members = group
			signalPID = -target.PGID
		}
	}
	return members, signalPID, true
}

func awaitGracefulScannedExit(ctx context.Context, members []ScannedProcess, grace time.Duration) ProcessTermination {
	deadline := time.NewTimer(grace)
	defer deadline.Stop()
	poll := time.NewTicker(50 * time.Millisecond)
	defer poll.Stop()
	for {
		done, err := scannedMembersGone(members)
		if err != nil {
			return ProcessTermination{Outcome: terminationUnproven, Err: err}
		}
		if done {
			return ProcessTermination{Outcome: "terminated"}
		}
		select {
		case <-ctx.Done():
			return ProcessTermination{Outcome: terminationUnproven, Err: ctx.Err()}
		case <-poll.C:
			continue
		case <-deadline.C:
			if err := killScannedMembers(members); err != nil {
				return ProcessTermination{Outcome: terminationUnproven, Err: err}
			}

			return awaitScannedExit(ctx, members)
		}
	}
}

func killScannedMembers(members []ScannedProcess) error {
	for _, p := range members {
		current, e := scanProcessIdentity(p.PID)
		if errors.Is(e, ErrProcessIdentityGone) {
			continue
		}
		if e != nil {
			return e
		}
		if !current.StartedAt.Equal(p.StartedAt) || current.Zombie {
			continue
		}
		if e := syscall.Kill(p.PID, syscall.SIGKILL); e != nil && !errors.Is(e, syscall.ESRCH) {
			return e
		}
	}
	return nil
}

func scannedMembersGone(members []ScannedProcess) (bool, error) {
	gone := true
	for _, p := range members {
		observed, err := scanProcessIdentity(p.PID)
		if errors.Is(err, ErrProcessIdentityGone) {
			continue
		}
		if err != nil {
			return false, err
		}
		if !observed.Zombie && observed.StartedAt.Equal(p.StartedAt) {
			gone = false
		}
	}
	return gone, nil
}

func awaitScannedExit(ctx context.Context, members []ScannedProcess) ProcessTermination {
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	poll := time.NewTicker(20 * time.Millisecond)
	defer poll.Stop()
	for {
		gone, err := scannedMembersGone(members)
		if err != nil {
			return ProcessTermination{Outcome: terminationUnproven, Err: err}
		}
		if gone {
			return ProcessTermination{Outcome: "terminated"}
		}
		select {
		case <-ctx.Done():
			return ProcessTermination{Outcome: terminationUnproven, Err: ctx.Err()}
		case <-deadline.C:
			return ProcessTermination{Outcome: terminationUnproven}
		case <-poll.C:
		}
	}
}
