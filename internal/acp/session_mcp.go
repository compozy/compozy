package acp

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/mcpbridge"
	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/store"
)

// prepareSessionMCPServers resolves the MCP servers sent in session/new or
// session/load. Providers that accept only remote MCP servers get each stdio
// server through a loopback HTTP bridge owned by the agent process.
func (d *Driver) prepareSessionMCPServers(ctx context.Context, process *AgentProcess, opts StartOpts) error {
	if opts.ProviderConfig == nil || !opts.ProviderConfig.SessionMCPOverHTTP() {
		return process.setSessionMCPServers(toSDKMCPServers(opts.MCPServers), nil)
	}
	servers := make([]acpsdk.McpServer, 0, len(opts.MCPServers))
	bridges := make([]*mcpbridge.Bridge, 0, len(opts.MCPServers))
	for _, server := range opts.MCPServers {
		if server.EffectiveTransport() != compozyconfig.MCPServerTransportStdio ||
			strings.TrimSpace(server.Command) == "" {
			continue
		}
		bridge, err := mcpbridge.Start(ctx, mcpbridge.Spec{
			Name:    server.Name,
			Command: server.Command,
			Args:    append([]string(nil), server.Args...),
			Env:     bridgedServerEnv(opts.Env, server.Env),
			Dir:     opts.Cwd,
		}, d.logger)
		if err != nil {
			closeErr := closeMCPBridges(bridges)
			return WrapFailure(store.FailureStartup, "session MCP bridge startup failed", errors.Join(
				fmt.Errorf("acp: bridge MCP server %q over HTTP: %w", server.Name, err),
				closeErr,
			))
		}
		bridges = append(bridges, bridge)
		servers = append(servers, acpsdk.McpServer{Http: &acpsdk.McpServerHttpInline{
			Name:    bridge.Name(),
			Type:    "http",
			Url:     bridge.URL(),
			Headers: []acpsdk.HttpHeader{{Name: "Authorization", Value: bridge.AuthorizationHeader()}},
		}})
	}
	if err := process.setSessionMCPServers(servers, bridges); err != nil {
		return WrapFailure(store.FailureStartup, "session MCP bridge startup failed", err)
	}
	return nil
}

// bridgedServerEnv gives a bridged child the environment the agent would have
// handed it: the agent's launch environment plus the server's own variables.
func bridgedServerEnv(agentEnv []string, serverEnv map[string]string) []string {
	base := append([]string(nil), agentEnv...)
	if len(base) == 0 {
		base = procutil.FilteredDaemonEnv(os.Environ())
	}
	for _, key := range slices.Sorted(maps.Keys(serverEnv)) {
		base = append(base, key+"="+serverEnv[key])
	}
	return base
}

// errAgentExitedBeforeMCPBridge reports bridges that finished starting after
// their agent process exited; they are closed instead of being registered.
var errAgentExitedBeforeMCPBridge = errors.New("acp: agent process exited before its MCP bridges were ready")

// setSessionMCPServers registers the session MCP payload and the bridges the
// process owns. After the process has exited and closed its bridges, late
// bridges are closed here instead of leaking their child and listener.
func (p *AgentProcess) setSessionMCPServers(servers []acpsdk.McpServer, bridges []*mcpbridge.Bridge) error {
	p.sessionMCPMu.Lock()
	// Only bridges need refusing; an exited process without bridges keeps its own failure.
	if p.mcpBridgesClosed && len(bridges) > 0 {
		p.sessionMCPMu.Unlock()
		return errors.Join(errAgentExitedBeforeMCPBridge, closeMCPBridges(bridges))
	}
	defer p.sessionMCPMu.Unlock()
	p.sessionMCP = servers
	p.mcpBridges = append(p.mcpBridges, bridges...)
	return nil
}

func (p *AgentProcess) sessionMCPServers() []acpsdk.McpServer {
	p.sessionMCPMu.Lock()
	defer p.sessionMCPMu.Unlock()
	if p.sessionMCP == nil {
		return []acpsdk.McpServer{}
	}
	return slices.Clone(p.sessionMCP)
}

// closeMCPBridges tears down the bridges this process owns when it exits.
func (p *AgentProcess) closeMCPBridges() error {
	p.sessionMCPMu.Lock()
	bridges := p.mcpBridges
	p.mcpBridges = nil
	p.mcpBridgesClosed = true
	p.sessionMCPMu.Unlock()
	return closeMCPBridges(bridges)
}

func closeMCPBridges(bridges []*mcpbridge.Bridge) error {
	errs := make([]error, 0, len(bridges))
	for _, bridge := range bridges {
		errs = append(errs, bridge.Close())
	}
	return errors.Join(errs...)
}
