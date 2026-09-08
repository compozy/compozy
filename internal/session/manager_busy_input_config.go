package session

import (
	"strings"

	"github.com/compozy/compozy/internal/config"
)

// SetBusyInputDefaultMode applies the follow-up preference to subsequent admissions.
func (m *Manager) SetBusyInputDefaultMode(mode string) error {
	cfg := config.DefaultSessionBusyInputConfig()
	cfg.DefaultMode = mode
	if err := cfg.Validate(); err != nil {
		return err
	}
	m.busyInputMu.Lock()
	m.busyInput.DefaultMode = cfg.Normalize().DefaultMode
	m.busyInputMu.Unlock()
	return nil
}

func (m *Manager) busyInputDefaultMode() string {
	m.busyInputMu.RLock()
	mode := strings.TrimSpace(m.busyInput.DefaultMode)
	m.busyInputMu.RUnlock()
	if mode == "" {
		return config.DefaultSessionBusyInputConfig().DefaultMode
	}
	return mode
}
