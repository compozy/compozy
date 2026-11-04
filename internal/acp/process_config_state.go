package acp

import "strings"

func (p *AgentProcess) configOptionsSnapshot() ([]SessionConfigOption, uint64) {
	p.capsMu.RLock()
	defer p.capsMu.RUnlock()
	return CloneSessionConfigOptions(p.caps.ConfigOptions), p.configRevision
}

// configRevisionAfterResponse returns the revision that a returned control response supersedes.
// The ACP SDK processes every notification received before a response ahead of returning
// that response, so those notifications are older than the response and must not discard it.
// Read the revision only after the request returns; a notification handled after this point
// was sent after the response and stays authoritative.
func (p *AgentProcess) configRevisionAfterResponse() uint64 {
	p.capsMu.RLock()
	defer p.capsMu.RUnlock()
	return p.configRevision
}

// A notification processed after the response returned is newer than the response, so
// retain the notification snapshot on overlap and confirm against that snapshot.
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
