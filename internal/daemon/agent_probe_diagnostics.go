package daemon

import (
	"fmt"
	"sync"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const maxAgentProbeDiagnosticKeys = 1024

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

type agentProbeDiagnostics struct {
	mu         sync.Mutex
	generation uint64
	seen       map[string]struct{}
}

func (d *agentProbeDiagnostics) shouldLog(generation uint64, kind string, target string, err error) bool {
	if d == nil {
		return true
	}
	key := kind + "\x00" + target + "\x00" + fmt.Sprintf("%T", err) + "\x00" + err.Error()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.generation != generation {
		d.generation = generation
		d.seen = make(map[string]struct{})
	}
	if _, ok := d.seen[key]; ok {
		return false
	}
	if len(d.seen) >= maxAgentProbeDiagnosticKeys {
		return false
	}
	d.seen[key] = struct{}{}
	return true
}
