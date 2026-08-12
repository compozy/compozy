package daemon

import (
	"fmt"
	"sync"
)

const maxAgentProbeDiagnosticKeys = 1024

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
