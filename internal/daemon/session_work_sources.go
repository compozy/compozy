package daemon

import (
	"context"
	"errors"
	"time"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/toolruntime"
)

type sessionLoopWorkReader interface {
	ListSessionLoopWork(context.Context, store.ReadScope, string, string, looppkg.RunID) ([]looppkg.SessionWork, error)
}

type sessionWorkSources struct {
	state   *bootState
	manager *session.Manager
	now     func() time.Time
}

func (s sessionWorkSources) tools(ctx context.Context, id string) ([]session.WorkSignal, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.state.processRegistry == nil {
		return nil, errors.New("tool process registry is unavailable")
	}
	result := make([]session.WorkSignal, 0)
	for _, observation := range s.state.processRegistry.Observations(id) {
		until := observation.VerifiedAt.Add(2 * toolruntime.SweepInterval)
		result = append(result, session.WorkSignal{
			Kind: session.WorkSignalToolRunning, Since: observation.Record.CreatedAt,
			Ref: observation.Record.ID, ValidUntil: until, StaleAttention: true,
		})
	}
	return result, nil
}

func (s sessionWorkSources) children(ctx context.Context, id string) ([]session.WorkSignal, error) {
	parent, err := s.manager.Status(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.state.registry.ListSessions(ctx, store.SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: parent.ProfileID}, WorkspaceID: parent.WorkspaceID,
		ParentSessionID: id,
	})
	if err != nil {
		return nil, err
	}
	result := make([]session.WorkSignal, 0)
	for _, row := range rows {
		info, err := s.manager.Status(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		until := s.now().Add(2 * toolruntime.SweepInterval)
		switch info.State {
		case session.StateStarting, session.StateActive:
		case session.StateStopping:
			child, ok := s.manager.Get(info.ID)
			if !ok {
				return nil, errors.New("stopping child has no live stop episode")
			}
			until = child.StoppingDeadline(s.state.cfg.Session.Stop.CooperativeGrace)
			if until.IsZero() {
				return nil, errors.New("stopping child has no stop deadline")
			}
		default:
			continue
		}
		result = append(result, session.WorkSignal{Kind: session.WorkSignalActiveChild,
			Since: info.CreatedAt, Ref: info.ID, ValidUntil: until, StaleAttention: info.State == session.StateStopping,
		})
	}
	return result, nil
}

func (s sessionWorkSources) leases(ctx context.Context, id string) ([]session.WorkSignal, error) {
	if s.state.tasks == nil {
		return nil, errors.New("task store is unavailable")
	}
	info, err := s.manager.Status(ctx, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.state.tasks.store.ListTaskRuns(ctx, taskpkg.RunQuery{
		ReadScope: store.ReadScope{ProfileID: info.ProfileID}, SessionID: id,
	})
	if err != nil {
		return nil, err
	}
	result := make([]session.WorkSignal, 0)
	for _, row := range rows {
		switch row.Status {
		case taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning:
			if row.LeaseUntil.IsZero() {
				continue
			}
			result = append(result, session.WorkSignal{Kind: session.WorkSignalTaskLease,
				Since: row.ClaimedAt, Ref: row.ID, ValidUntil: row.LeaseUntil,
			})
		}
	}
	return result, nil
}

func (s sessionWorkSources) loopWork(ctx context.Context, id string) ([]looppkg.SessionWork, error) {
	reader, ok := s.state.registry.(sessionLoopWorkReader)
	if !ok {
		return nil, errors.New("loop work store is unavailable")
	}
	info, err := s.manager.Status(ctx, id)
	if err != nil {
		return nil, err
	}
	var ownerRunID looppkg.RunID
	if info.NetworkOwnerKey != "" {
		owner, err := participation.OwnerRefFromKey(info.WorkspaceID, info.NetworkOwnerKey)
		if err != nil {
			return nil, err
		}
		if owner.Kind == participation.OwnerKindLoopRun {
			ownerRunID = looppkg.RunID(owner.ID)
		}
	}
	return reader.ListSessionLoopWork(ctx, store.ReadScope{ProfileID: info.ProfileID}, info.WorkspaceID, id, ownerRunID)
}

func (s sessionWorkSources) loops(ctx context.Context, id string) ([]session.WorkSignal, error) {
	rows, err := s.loopWork(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]session.WorkSignal, 0, len(rows))
	for _, row := range rows {
		if (row.Status == looppkg.StatusPaused || row.Status == looppkg.StatusWatching) && !row.NeedsAttention &&
			len(row.Waits) == 0 {
			continue
		}
		result = append(result, sessionLoopWorkSignal(row))
	}
	return result, nil
}

// Durable leases and admitted wait deadlines bound activity; reading a run never renews it.
func sessionLoopWorkSignal(row looppkg.SessionWork) session.WorkSignal {
	until := row.ReconciledAt.Add(2 * defaultMechanicalSchedulerInterval)
	if initial := row.Since.Add(2 * defaultMechanicalSchedulerInterval); initial.After(until) {
		until = initial
	}
	if row.LeaseUntil.After(until) {
		until = row.LeaseUntil
	}
	attention := ""
	if row.NeedsAttention {
		attention = "Loop work requires attention; inspect the run's node failures and quarantine."
	}
	if row.Status == looppkg.StatusNeedsApproval {
		attention = "The Goal or Loop is waiting for approval; inspect the run's pending request."
	}
	for _, wait := range row.Waits {
		if wait.ClaimState == looppkg.WaitClaimInterventionRequired {
			attention = "A Loop wait requires intervention; inspect the run's pending request."
			continue
		}
		deadline := wait.ResumeAt
		if wait.NextEscalationAt != nil && (deadline == nil || wait.NextEscalationAt.Before(*deadline)) {
			deadline = wait.NextEscalationAt
		}
		if deadline != nil && deadline.Add(2*defaultMechanicalSchedulerInterval).After(until) {
			until = deadline.Add(2 * defaultMechanicalSchedulerInterval)
		}
	}
	return session.WorkSignal{
		Kind: session.WorkSignalLoopRun, Since: row.Since, Ref: string(row.RunID), ValidUntil: until,
		StaleAttention: attention == "", AttentionReason: attention,
	}
}

func (s sessionWorkSources) waits(ctx context.Context, id string) ([]session.WorkSignal, error) {
	rows, err := s.loopWork(ctx, id)
	if err != nil {
		return nil, err
	}
	result := make([]session.WorkSignal, 0)
	for _, row := range rows {
		for _, wait := range row.Waits {
			if wait.ClaimState != looppkg.WaitClaimWaiting && wait.ClaimState != looppkg.WaitClaimClaimed {
				continue
			}
			deadline := wait.ResumeAt
			if wait.NextEscalationAt != nil && (deadline == nil || wait.NextEscalationAt.Before(*deadline)) {
				deadline = wait.NextEscalationAt
			}
			if deadline == nil {
				return nil, errors.New("scheduled wait has no admission deadline")
			}
			result = append(result, session.WorkSignal{Kind: session.WorkSignalScheduledWait,
				Since: wait.CreatedAt, Ref: string(row.RunID) + ":" + string(wait.NodeID), ValidUntil: *deadline,
			})
		}
	}
	return result, nil
}
