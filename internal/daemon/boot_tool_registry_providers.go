package daemon

import (
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (d *Daemon) bootToolProviders(
	state *bootState,
	registry func() toolspkg.Registry,
) ([]toolspkg.Provider, error) {
	var mcpAuth toolspkg.MCPAuthStatusProvider
	deps := d.nativeToolsDeps(state, registry)
	deps.MCPAuth = func() toolspkg.MCPAuthStatusProvider { return mcpAuth }
	nativeProvider, err := newDaemonNativeProvider(&deps)
	if err != nil {
		return nil, fmt.Errorf("daemon: create native tool provider: %w", err)
	}
	providers := []toolspkg.Provider{nativeProvider}

	extensionProvider, err := newDaemonExtensionToolProvider(state)
	if err != nil {
		return nil, fmt.Errorf("daemon: create extension tool provider: %w", err)
	}
	if extensionProvider != nil {
		providers = append(providers, extensionProvider)
	}

	mcpProvider, mcpAuthProvider, err := d.newDaemonMCPToolProvider(state)
	if err != nil {
		return nil, fmt.Errorf("daemon: create mcp tool provider: %w", err)
	}
	mcpAuth = mcpAuthProvider
	if mcpProvider != nil {
		state.mcpToolProvider = mcpProvider
		providers = append(providers, mcpProvider)
	}
	return providers, nil
}
