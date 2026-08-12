package daemon

import (
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type agentProbeConfigState struct {
	mu         sync.RWMutex
	config     compozyconfig.Config
	generation uint64
}

func newAgentProbeConfigState(config *compozyconfig.Config) *agentProbeConfigState {
	return &agentProbeConfigState{
		config:     compozyconfig.CloneConfig(config),
		generation: 1,
	}
}

func (s *agentProbeConfigState) Snapshot() (compozyconfig.Config, uint64) {
	if s == nil {
		return compozyconfig.Config{}, 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return compozyconfig.CloneConfig(&s.config), s.generation
}

func (s *agentProbeConfigState) Update(config *compozyconfig.Config) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.config = compozyconfig.CloneConfig(config)
	s.generation++
	s.mu.Unlock()
}
