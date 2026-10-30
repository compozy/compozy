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
	members, valid := scannedSignalMembers(snapshot, target, lineage)
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
		members = []ScannedProcess{target}
	}
	// A process can join a group after any snapshot. Signal the verified members
	// individually so a late joiner never inherits this operation's authority.
	if err := signalScannedMembers(members, syscall.SIGTERM); err != nil {
		return ProcessTermination{Outcome: terminationFailed, Err: err}
	}
	return awaitGracefulScannedExit(ctx, members, grace)
}

func scannedSignalMembers(snapshot ProcessSnapshot, target ScannedProcess, lineage bool) ([]ScannedProcess, bool) {
	attributed := false
	for _, p := range snapshot.Processes {
		if p.PID == target.PID && p.StartedAt.Equal(target.StartedAt) && p.SessionID == target.SessionID &&
			p.Home == target.Home {
			attributed = true
			break
		}
	}
	if !attributed && !lineage {
		return nil, false
	}
	members := []ScannedProcess{target}
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
		}
	}
	return members, true
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
			if err := signalScannedMembers(members, syscall.SIGKILL); err != nil {
				return ProcessTermination{Outcome: terminationUnproven, Err: err}
			}

			return awaitScannedExit(ctx, members)
		}
	}
}

func signalScannedMembers(members []ScannedProcess, signal syscall.Signal) error {
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
		if p.PID <= 1 || p.PID == os.Getpid() || current.PGID == syscall.Getpgrp() {
			return errors.New("process moved into protected group")
		}
		if e := syscall.Kill(p.PID, signal); e != nil && !errors.Is(e, syscall.ESRCH) {
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
