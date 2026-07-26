package session

import aghconfig "github.com/compozy/agh/internal/config"

// AgentDefinition returns the concrete definition snapshot that owns this
// session. The snapshot is immutable from callers' perspective.
func (s *Session) AgentDefinition() aghconfig.AgentDef {
	if s == nil {
		return aghconfig.AgentDef{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent := aghconfig.CloneAgentDef(s.agentDef)
	if agent.Name == "" {
		agent.Name = s.AgentName
	}
	return agent
}

func (s *Session) setAgentDefinition(agent aghconfig.AgentDef) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.agentDef = aghconfig.CloneAgentDef(agent)
	s.mu.Unlock()
}

// SessionAgentDefinition returns the concrete definition for one live session.
func (m *Manager) SessionAgentDefinition(id string) (aghconfig.AgentDef, bool) {
	if m == nil {
		return aghconfig.AgentDef{}, false
	}
	session, ok := m.Get(id)
	if !ok || session == nil {
		return aghconfig.AgentDef{}, false
	}
	agent := session.AgentDefinition()
	return agent, agent.Name != ""
}
