package session

import (
	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

// AgentDefinition returns the concrete definition snapshot that owns this
// session. The snapshot is immutable from callers' perspective.
func (s *Session) AgentDefinition() compozyconfig.AgentDef {
	if s == nil {
		return compozyconfig.AgentDef{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	agent := compozyconfig.CloneAgentDef(s.agentDef)
	if agent.Name == "" {
		agent.Name = s.AgentName
	}
	return agent
}

func (s *Session) setAgentDefinition(agent compozyconfig.AgentDef, manifest acp.StartupManifest) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.agentDef = compozyconfig.CloneAgentDef(agent)
	s.startupManifest = acp.CloneStartupManifest(manifest)
	s.mu.Unlock()
}

// SessionAgentDefinition returns the concrete definition for one live session.
func (m *Manager) SessionAgentDefinition(id string) (compozyconfig.AgentDef, bool) {
	if m == nil {
		return compozyconfig.AgentDef{}, false
	}
	session, ok := m.Get(id)
	if !ok || session == nil {
		return compozyconfig.AgentDef{}, false
	}
	agent := session.AgentDefinition()
	return agent, agent.Name != ""
}

func (s *Session) startupDefinition() (compozyconfig.AgentDef, acp.StartupManifest) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return compozyconfig.CloneAgentDef(s.agentDef), acp.CloneStartupManifest(s.startupManifest)
}
