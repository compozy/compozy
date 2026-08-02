package acp

import "strings"

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
			p.caps.ConfigOptions[index].Current = value
			return
		}
	}
}
