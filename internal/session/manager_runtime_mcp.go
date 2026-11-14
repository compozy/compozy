package session

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/runtimemcp"
	shellquote "github.com/kballard/go-shellquote"
)

// RuntimeMCPInventory supplies native launch inputs for the session execution context.
type RuntimeMCPInventory interface {
	List(context.Context, runtimemcp.Request) (runtimemcp.Inventory, error)
}

// WithRuntimeMCPInventory shares the daemon inventory cache with session admission.
func WithRuntimeMCPInventory(inventory RuntimeMCPInventory) Option {
	return func(m *Manager) { m.runtimeMCPInventory = inventory }
}

func runtimeMCPDescriptor(agent compozyconfig.ResolvedAgent) (runtimemcp.Descriptor, bool) {
	runtimeID := cmp.Or(agent.RuntimeProvider, agent.Provider)
	if agent.Harness == compozyconfig.ProviderHarnessPiACP {
		runtimeID = "pi"
	}
	if runtimeID == "qwen" {
		runtimeID = "qwen-code"
	}
	for _, descriptor := range runtimemcp.Descriptors() {
		if descriptor.Runtime == runtimeID {
			return descriptor, true
		}
	}
	return runtimemcp.Descriptor{}, false
}

func runtimeMCPNeedsDigest(descriptor runtimemcp.Descriptor, mode string) bool {
	return !descriptor.NativeHotReload || mode == "off" &&
		(descriptor.Runtime == "copilot" || descriptor.Runtime == "qwen-code")
}

func (m *Manager) readRuntimeMCP(ctx context.Context, request runtimemcp.Request) (runtimemcp.Snapshot, error) {
	var inventory runtimemcp.Inventory
	var err error
	if m.runtimeMCPInventory != nil {
		inventory, err = m.runtimeMCPInventory.List(ctx, request)
	} else {
		inventory, _, err = (runtimemcp.Reader{}).List(ctx, request)
	}
	if err != nil {
		return runtimemcp.Snapshot{}, err
	}
	for _, snapshot := range inventory.Runtimes {
		if snapshot.Runtime == request.Runtime {
			return snapshot, nil
		}
	}
	canDisable, displayName := false, ""
	for _, descriptor := range runtimemcp.Descriptors() {
		if descriptor.Runtime == request.Runtime {
			canDisable, displayName = descriptor.CanDisableRuntimeMCP, descriptor.DisplayName
			break
		}
	}
	return runtimemcp.Snapshot{
		Runtime: request.Runtime, DisplayName: displayName, ReadState: "ok",
		RuntimeMCP:       runtimemcp.RuntimeMode{CanDisable: canDisable},
		ActivationDigest: activationFingerprint("", request.Runtime, ""),
	}, nil
}

func (m *Manager) prepareRuntimeMCPStartup(
	ctx context.Context, spec *sessionStartSpec, runtime *sessionStartRuntime, opts acp.StartOpts,
) (acp.StartOpts, error) {
	runtime.activationMode = spec.workspace.Config.Providers[runtime.agent.Provider].EffectiveRuntimeMCP()
	runtime.activationDigest, runtime.activationRequest = "", nil
	descriptor, ok := runtimeMCPDescriptor(runtime.agent)
	if !ok {
		return opts, nil
	}
	env := slices.Clone(opts.Env)
	command, err := compozyconfig.ParseLaunchCommand(opts.Command)
	if err != nil {
		return opts, err
	}
	for _, assignment := range command.Environment {
		key, value, _ := strings.Cut(assignment, "=")
		env = setSessionStartEnvValue(env, key, value)
	}
	readContext, err := runtimemcp.ProviderContext(m.homePaths, runtime.agent.Provider,
		providerConfigFromResolvedAgent(runtime.agent), env, opts.Cwd, spec.worktreeRoot)
	if err != nil {
		return opts, fmt.Errorf("session: resolve runtime MCP context: %w", err)
	}
	if home := providerHomeIdentity(env); home != "" {
		readContext.Home = home
	}
	readContext.Env = runtimemcp.Environment(env)
	readContext.Profile, readContext.WorkspaceRoot = spec.profileID, spec.workspace.RootDir
	request := runtimemcp.Request{Context: readContext, Runtime: descriptor.Runtime}
	snapshot, err := m.readRuntimeMCP(ctx, request)
	if err != nil {
		return opts, fmt.Errorf("session: read runtime MCP inventory: %w", err)
	}
	runtime.activationRequest = &request
	if runtimeMCPNeedsDigest(descriptor, runtime.activationMode) {
		runtime.activationDigest = snapshot.ActivationDigest
	}
	if descriptor.Runtime == "claude" {
		opts.ManagedProviderMCP = !snapshot.RuntimeMCP.CanDisable
		if runtime.activationMode == "off" && !opts.ManagedProviderMCP {
			opts.IsolateProviderMCP = true
		}
		return opts, nil
	}
	if runtime.activationMode != "off" || !descriptor.CanDisableRuntimeMCP {
		return opts, nil
	}
	if snapshot.ReadState != "ok" {
		return opts, fmt.Errorf("session: cannot disable runtime MCP servers: %s", snapshot.ReadError)
	}
	args := []string{}
	if descriptor.Runtime == "copilot" {
		args = append(args, "--disable-builtin-mcps")
	}
	for _, server := range snapshot.Servers {
		args = append(args, "--disable-mcp-server", server.Name)
	}
	if len(args) > 0 {
		opts.Command += " " + shellquote.Join(args...)
	}
	return opts, nil
}

func (m *Manager) currentRuntimeMCPDigest(ctx context.Context, state activationState, mode string) string {
	if state.runtimeRequest == nil {
		return state.runtimeDigest
	}
	for _, descriptor := range runtimemcp.Descriptors() {
		if descriptor.Runtime != state.runtimeRequest.Runtime {
			continue
		}
		if !runtimeMCPNeedsDigest(descriptor, mode) {
			return ""
		}
		snapshot, err := m.readRuntimeMCP(ctx, *state.runtimeRequest)
		if err == nil && snapshot.ReadState == "ok" {
			return snapshot.ActivationDigest
		}
	}
	return state.runtimeDigest
}
