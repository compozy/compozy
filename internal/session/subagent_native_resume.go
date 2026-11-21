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
	updated, changed, err := s.store.ResumeNativeSubagent(ctx, row.ID, ev.ParentTurnID, ev.Task.RunID, at)
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
		if ev.Task.RunID != "" && ev.Task.RunID < c.runID {
			return
		}
		if ev.Task.Background != nil && *ev.Task.Background {
			c.background = true
		}
		if nativeTaskActive(ev.Task) {
			if !ev.At.IsZero() && !c.terminalAt.IsZero() && !ev.At.After(c.terminalAt) {
				return
			}
			ev.Task = retainNativeRunUsage(ev, c.tasks)
			c.tasks = c.tasks[:0]
			c.terminalAt = time.Time{}
		}
		if _, terminal := nativeTaskOutcome(ev.Task); terminal && ev.At.After(c.terminalAt) {
			c.terminalAt = ev.At
		}
		if _, terminal := nativeTaskOutcome(ev.Task); terminal || nativeTaskActive(ev.Task) {
			c.runID = max(c.runID, ev.Task.RunID)
			c.tasks = append(c.tasks, ev)
		}
		return
	}
	c.background = c.background || ev.Background
	if ev.Status == store.SubagentStatusCompleted || ev.Status == store.SubagentStatusFailed {
		c.result = &ev
	}
}

// Keep a run's usage when replay compacts its earlier lifecycle edges.
func retainNativeRunUsage(ev NativeSubagentEvent, earlier []NativeSubagentEvent) *NativeSubagentTask {
	task := *ev.Task
	var peak acp.NativeTaskUsage
	if task.Usage != nil {
		peak = *task.Usage
	}
	for _, old := range earlier {
		if old.Task.Usage == nil || old.Task.RunID != task.RunID ||
			(task.RunID == "" && old.ParentTurnID != ev.ParentTurnID) {
			continue
		}
		peak.TotalTokens = max(peak.TotalTokens, old.Task.Usage.TotalTokens)
		peak.ToolUses = max(peak.ToolUses, old.Task.Usage.ToolUses)
		task.Usage = &peak
	}
	return &task
}

func (s *subagentService) recordNativeUsage(
	ctx context.Context, row *store.SessionSubagent, ev *NativeSubagentEvent,
) error {
	usage := ev.Task.Usage
	if usage == nil {
		return nil
	}
	at := ev.At
	if at.IsZero() {
		at = s.now().UTC()
	}
	updated, changed, err := s.store.UpdateSubagentUsage(
		ctx, row.ID, ev.Task.RunID, store.SubagentUsage{TotalTokens: usage.TotalTokens, ToolUses: usage.ToolUses}, at,
	)
	if err != nil {
		return err
	}
	*row = updated
	if changed {
		s.publish(ctx, updated)
	}
	return nil
}
