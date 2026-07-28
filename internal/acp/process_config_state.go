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
	for index := range p.Caps.ConfigOptions {
		if strings.TrimSpace(p.Caps.ConfigOptions[index].ID) == id {
			p.Caps.ConfigOptions[index].Current = value
			return
		}
	}
}
