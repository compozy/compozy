package daemon

import (
	extensionpkg "github.com/compozy/agh/internal/extension"
	mcppkg "github.com/compozy/agh/internal/mcp"
	"github.com/compozy/agh/internal/resources"
)

func (d *Daemon) applyExtensionManagerFactoryDefault() {
	if d.newExtensionManager != nil {
		return
	}
	d.newExtensionManager = func(deps extensionManagerDeps) extensionRuntime {
		if deps.Registry == nil || deps.ResourceStore == nil || deps.SourceSessions == nil {
			return nil
		}

		capChecker := &extensionpkg.CapabilityChecker{}
		capChecker.SetResourcePolicy(deps.Extensions.Resources)
		hostAPI := extensionpkg.NewHostAPIHandler(
			newHostAPISessionManagerAdapter(deps.Sessions),
			deps.MemoryStore,
			deps.Observer,
			deps.SkillsRegistry,
			buildHostAPIOptions(&deps, capChecker, deps.ResourceStore)...,
		)
		hostAPIFacade := mcppkg.NewHostAPIFacade(
			hostAPI,
			capChecker,
			deps.WorkspaceResolver,
			deps.SourceSessions,
		)

		manager := extensionpkg.NewManager(
			deps.Registry,
			buildExtensionManagerOptions(
				&deps,
				capChecker,
				hostAPI,
				deps.SourceSessions,
			)...,
		)
		return newExtensionRuntimeWithMCPHostAPI(manager, hostAPIFacade)
	}
}

func buildExtensionManagerOptions(
	deps *extensionManagerDeps,
	capChecker *extensionpkg.CapabilityChecker,
	hostAPI *extensionpkg.HostAPIHandler,
	sourceSessions resources.SourceSessionManager,
) []extensionpkg.Option {
	opts := []extensionpkg.Option{
		extensionpkg.WithCapabilityChecker(capChecker),
		extensionpkg.WithLogger(deps.Logger),
		extensionpkg.WithSourceSessionManager(sourceSessions),
		extensionpkg.WithProcessRegistry(deps.ProcessRegistry),
		extensionpkg.WithAGHExecutableResolver(deps.AGHExecutable),
		extensionpkg.WithExtensionToolCallTracker(hostAPI),
	}
	if sink, ok := deps.Observer.(extensionpkg.BridgeTelemetrySink); ok {
		opts = append(opts, extensionpkg.WithBridgeTelemetrySink(sink))
	}
	if deps.BridgeRuntime != nil {
		opts = append(opts, extensionpkg.WithBridgeRuntimeResolver(deps.BridgeRuntime))
	}
	if deps.SecretResolver != nil {
		opts = append(opts, extensionpkg.WithSecretResolver(deps.SecretResolver))
	}
	for method, handler := range hostAPI.MethodHandlers() {
		opts = append(opts, extensionpkg.WithHostMethodHandler(method, handler))
	}
	return opts
}
