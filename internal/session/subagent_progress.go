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
	progress = subagentFirstLine(progress, 280)
	if progress == "" {
		return
	}
	s.mu.Lock()
	if pending := s.progress[row.ID]; pending != nil {
		pending.text = progress
		pending.count++
		s.mu.Unlock()
		return
	}
	pending := &subagentProgress{text: progress, count: 1}
	s.progress[row.ID] = pending
	s.mu.Unlock()
	s.launch(func() {
		timer := time.NewTimer(time.Second)
		defer timer.Stop()
		select {
		case <-s.ctx.Done():
			s.mu.Lock()
			delete(s.progress, row.ID)
			s.mu.Unlock()
			return
		case <-timer.C:
		}
		unlock := s.lock(row.ParentSessionID)
		defer unlock()
		s.mu.Lock()
		text, count := pending.text, pending.count
		delete(s.progress, row.ID)
		s.mu.Unlock()
		latest, err := s.store.GetSubagent(s.ctx, row.WorkspaceID, row.ID)
		if err != nil {
			s.logError(s.ctx, "progress", err)
			return
		}
		if store.IsSubagentStatusTerminal(latest.Status) {
			return
		}
		if err := s.store.UpdateProgress(s.ctx, row.ID, text, s.now().UTC()); err != nil {
			s.logError(s.ctx, "progress", err)
			return
		}
		s.logError(s.ctx, "progress_publish", s.publishID(s.ctx, row.WorkspaceID, row.ID))
		if count > 1 {
			s.logger.DebugContext(s.ctx, "subagent.progress_coalesced", "subagent_id", row.ID, "count", count)
		}
	})
}
