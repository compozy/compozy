package session

import (
	"context"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/store"
)

type subagentProgress struct {
	text  string
	count int
}

func (s *subagentService) OnChildActivity(ctx context.Context, child, progress string) {
	row, err := s.store.GetSubagentByChild(ctx, child)
	if errors.Is(err, store.ErrSubagentNotFound) {
		return
	}
	if err != nil {
		s.logError(ctx, "progress", err)
		return
	}
	if store.IsSubagentStatusTerminal(row.Status) {
		return
	}
	s.queueProgress(row.WorkspaceID, row.ParentSessionID, row.ID, progress)
}

// queueProgress coalesces a subagent's progress lines into one write (and one
// published row, which bumps updated_at) per second. Terminal rows are skipped.
func (s *subagentService) queueProgress(workspaceID, parentID, id, progress string) {
	progress = subagentFirstLine(progress, 280)
	if progress == "" {
		return
	}
	s.mu.Lock()
	if pending := s.progress[id]; pending != nil {
		pending.text = progress
		pending.count++
		s.mu.Unlock()
		return
	}
	pending := &subagentProgress{text: progress, count: 1}
	s.progress[id] = pending
	s.mu.Unlock()
	s.launch(func() {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-s.ctx.Done():
			s.mu.Lock()
			delete(s.progress, id)
			s.mu.Unlock()
			return
		case <-timer.C:
		}
		unlock := s.lock(parentID)
		defer unlock()
		s.mu.Lock()
		text, count := pending.text, pending.count
		delete(s.progress, id)
		s.mu.Unlock()
		latest, err := s.store.GetSubagent(s.ctx, workspaceID, id)
		if errors.Is(err, store.ErrSubagentNotFound) {
			return
		}
		if err != nil {
			s.logError(s.ctx, "progress", err)
			return
		}
		if store.IsSubagentStatusTerminal(latest.Status) {
			return
		}
		if err := s.store.UpdateProgress(s.ctx, id, text, s.now().UTC()); err != nil {
			s.logError(s.ctx, "progress", err)
			return
		}
		s.logError(s.ctx, "progress_publish", s.publishID(s.ctx, workspaceID, id))
		if count > 1 {
			s.logger.DebugContext(s.ctx, "subagent.progress_coalesced", "subagent_id", id, "count", count)
		}
	})
}
