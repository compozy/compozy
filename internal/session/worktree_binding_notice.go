package session

import (
	"fmt"
	"strings"
)

func (s *Session) worktreeBindingNotice(message string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	transition := s.worktreeTransition
	if transition == nil || transition.Pending || !transition.NoticePending {
		return message
	}
	return fmt.Sprintf("<compozy-environment-change>\n"+
		"The working directory changed from %q to %q. Continue this same conversation in the current directory. "+
		"Files were not moved; previous absolute paths may refer to the old checkout. "+
		"Current worktree ID: %q (empty means workspace root).\n</compozy-environment-change>\n\n%s",
		transition.Previous.CWD, transition.Current.CWD, transition.Current.WorktreeID, strings.TrimSpace(message))
}

func (m *Manager) consumeWorktreeBindingNotice(target *Session) {
	target.persistMu.Lock()
	defer target.persistMu.Unlock()
	target.mu.Lock()
	transition := target.worktreeTransition
	if transition == nil || transition.Pending || !transition.NoticePending {
		target.mu.Unlock()
		return
	}
	transition.NoticePending = false
	target.mu.Unlock()
	if err := m.writeMeta(target); err != nil {
		m.sessionLogger(target).Error("session.worktree.notice_ack_failed", "error", err)
	}
}
