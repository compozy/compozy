package session

import commandpkg "github.com/compozy/compozy/internal/command"

func (s *Session) setCurrentSkillInvocations(invocations []commandpkg.Invocation) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.currentSkillInvocations = cloneSkillInvocations(invocations)
	s.mu.Unlock()
}

func (s *Session) CurrentSkillInvocations() []commandpkg.Invocation {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSkillInvocations(s.currentSkillInvocations)
}

func cloneSkillInvocations(invocations []commandpkg.Invocation) []commandpkg.Invocation {
	if len(invocations) == 0 {
		return nil
	}
	return append([]commandpkg.Invocation(nil), invocations...)
}
