package acp

import "strings"

func (p *AgentProcess) configOptionsSnapshot() ([]SessionConfigOption, uint64) {
	p.capsMu.RLock()
	defer p.capsMu.RUnlock()
	return CloneSessionConfigOptions(p.caps.ConfigOptions), p.configRevision
}

// A notification processed while a request is outstanding must not be overwritten
// by its response waiter. The SDK does not expose the response's receive sequence,
// so retain the notification snapshot on overlap and confirm against that snapshot.
func (p *AgentProcess) setConfigOptionsResponse(options []SessionConfigOption, revision uint64) []SessionConfigOption {
	p.capsMu.Lock()
	defer p.capsMu.Unlock()
	if p.configRevision == revision {
		p.setConfigOptionsLocked(options)
	}
	return CloneSessionConfigOptions(p.caps.ConfigOptions)
}

func (p *AgentProcess) setConfigOptionCurrent(optionID string, current string) {
	if p == nil {
		return
	}
	id := strings.TrimSpace(optionID)
	value := strings.TrimSpace(current)
	if id == "" || value == "" {
		return
	}

	p.capsMu.Lock()
	defer p.capsMu.Unlock()
	for index := range p.caps.ConfigOptions {
		if strings.TrimSpace(p.caps.ConfigOptions[index].ID) == id {
			p.caps.ConfigOptions[index].CurrentValueID = value
			p.configRevision++
			return
		}
	}
}
