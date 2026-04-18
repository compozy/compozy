package extensions

import (
	"errors"
	"log/slog"
	"strings"

	"github.com/compozy/compozy/internal/core/provider"
)

// ResolveReviewProviderBridge returns the run-local extension bridge for one
// declared review provider when this runtime owns it.
func (m *Manager) ResolveReviewProviderBridge(name string) (provider.ExtensionBridge, bool) {
	if m == nil {
		return nil, false
	}

	key := reviewProviderKey(name)
	if key == "" {
		return nil, false
	}

	m.mu.RLock()
	bridge := m.reviewBridges[key]
	m.mu.RUnlock()
	if bridge != nil {
		return bridge, true
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if bridge := m.reviewBridges[key]; bridge != nil {
		return bridge, true
	}

	entry, ok := m.reviewProviders[key]
	if !ok {
		return nil, false
	}

	bridge, err := NewReviewProviderBridge(entry, m.workspaceRoot, m.invokingCommand)
	if err != nil {
		slog.Warn(
			"build runtime review provider bridge",
			"component", "extension.runtime",
			"provider", strings.TrimSpace(name),
			"err", err,
		)
		return nil, false
	}

	m.reviewBridges[key] = bridge
	return bridge, true
}

func (m *Manager) closeReviewProviderBridges() error {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	bridges := make([]*ReviewProviderBridge, 0, len(m.reviewBridges))
	for key, bridge := range m.reviewBridges {
		if bridge == nil {
			continue
		}
		bridges = append(bridges, bridge)
		delete(m.reviewBridges, key)
	}
	m.mu.Unlock()

	var closeErr error
	for _, bridge := range bridges {
		if err := bridge.Close(); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	return closeErr
}

func reviewProviderKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
