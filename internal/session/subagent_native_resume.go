package session

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func nativeTaskActive(task *NativeSubagentTask) bool {
	return task != nil && (task.Kind == acp.NativeTaskKindSpawn ||
		task.Kind == acp.NativeTaskKindProgress ||
		task.Kind == acp.NativeTaskKindState && task.Status == acp.NativeTaskStatusRunning)
}

func (s *subagentService) resumeNativeTask(
	ctx context.Context,
	row store.SessionSubagent,
	ev NativeSubagentEvent,
) (store.SessionSubagent, bool, error) {
	if !nativeTaskActive(ev.Task) || !store.IsSubagentStatusTerminal(row.Status) {
		return row, false, nil
	}
	if row.ParentTurnID != ev.ParentTurnID {
		snap, err := s.runtime.Snapshot(ctx, row.ParentSessionID)
		if err != nil {
			return row, false, err
		}
		if !snap.Active || snap.TurnID != ev.ParentTurnID {
			return row, false, nil
		}
	}
	at := ev.At
	if at.IsZero() {
		at = s.now().UTC()
	}
	updated, changed, err := s.store.ResumeNativeSubagent(ctx, row.ID, ev.ParentTurnID, at)
	if err != nil {
		return row, false, err
	}
	if changed {
		s.publish(ctx, updated)
		s.runtime.PublishParent(ctx, row.ParentSessionID)
	}
	return updated, changed, nil
}

func (c *nativeReplayCall) observe(ev NativeSubagentEvent) {
	if ev.Task != nil {
		if ev.Task.Background != nil && *ev.Task.Background {
			c.background = true
		}
		if nativeTaskActive(ev.Task) {
			if !ev.At.IsZero() && !c.terminalAt.IsZero() && !ev.At.After(c.terminalAt) {
				return
			}
			c.tasks = c.tasks[:0]
			c.terminalAt = time.Time{}
		}
		if _, terminal := nativeTaskOutcome(ev.Task); terminal && ev.At.After(c.terminalAt) {
			c.terminalAt = ev.At
		}
		if _, terminal := nativeTaskOutcome(ev.Task); terminal || nativeTaskActive(ev.Task) {
			c.tasks = append(c.tasks, ev)
		}
		return
	}
	c.background = c.background || ev.Background
	if ev.Status == store.SubagentStatusCompleted || ev.Status == store.SubagentStatusFailed {
		c.result = &ev
	}
}
