package extensionpkg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"slices"

	"time"

	"github.com/compozy/agh/internal/resources"

	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/toolruntime"

	"github.com/compozy/agh/internal/version"
)

func (m *Manager) launchRuntime(
	ctx context.Context,
	ext *managedExtension,
) (launchedRuntime, error) {
	launchCfg, runtime, healthInterval, cleanups, err := m.launchConfigFor(ctx, ext)
	if err != nil {
		return launchedRuntime{}, err
	}

	process, err := m.launch(ctx, launchCfg)
	if err != nil {
		runExtensionRedactionCleanups(cleanups)
		return launchedRuntime{}, fmt.Errorf(
			"launch subprocess: %w",
			err,
		)
	}

	resourceSession, err := m.prepareExtensionResourceSession(ctx, ext)
	if err != nil {
		return launchedRuntime{}, m.cleanupLaunchedProcess(
			ctx,
			process,
			cleanups,
			err,
		)
	}
	if err := m.registerRuntimeHostMethods(process, ext, runtime, resourceSession); err != nil {
		return launchedRuntime{}, m.cleanupLaunchedProcess(
			ctx,
			process,
			cleanups,
			err,
		)
	}

	response, err := m.initializeRuntimeProcess(ctx, process, ext, runtime, resourceSession)
	if err != nil {
		return launchedRuntime{}, m.cleanupLaunchedProcess(
			ctx,
			process,
			cleanups,
			err,
		)
	}
	if err := validateSupportedHookEvents(response.SupportedHookEvents); err != nil {
		return launchedRuntime{}, m.cleanupLaunchedProcess(
			ctx,
			process,
			cleanups,
			err,
		)
	}

	return launchedRuntime{
		process:           process,
		response:          response,
		runtime:           runtime,
		healthInterval:    healthInterval,
		sessionNonce:      resourceSession.Actor.SessionNonce,
		redactionCleanups: cleanups,
	}, nil
}

func (m *Manager) cleanupLaunchedProcess(
	ctx context.Context,
	process processHandle,
	cleanups []func(),
	err error,
) error {
	runExtensionRedactionCleanups(cleanups)
	if process == nil {
		return err
	}
	if shutdownErr := shutdownProcessWithTimeout(ctx, process, m.defaultShutdownTimeout); shutdownErr != nil {
		return errors.Join(err, shutdownErr)
	}
	return err
}

func (m *Manager) prepareExtensionResourceSession(
	ctx context.Context,
	ext *managedExtension,
) (*hostAPIResourceSession, error) {
	resourceSession, err := m.newHostAPIResourceSession(ext)
	if err != nil {
		return nil, err
	}
	if err := m.activateExtensionSourceSession(ctx, resourceSession.Actor); err != nil {
		return nil, err
	}
	return resourceSession, nil
}

func (m *Manager) registerRuntimeHostMethods(
	process processHandle,
	ext *managedExtension,
	runtime subprocess.InitializeRuntime,
	resourceSession *hostAPIResourceSession,
) error {
	for method, handler := range m.hostMethods {
		if err := process.HandleMethod(
			method,
			m.wrapHostHandler(ext.info.Name, method, runtime.Bridge, resourceSession, handler),
		); err != nil {
			return fmt.Errorf("register host method %q: %w", method, err)
		}
	}
	return nil
}

func (m *Manager) initializeRuntimeProcess(
	ctx context.Context,
	process processHandle,
	ext *managedExtension,
	runtime subprocess.InitializeRuntime,
	resourceSession *hostAPIResourceSession,
) (subprocess.InitializeResponse, error) {
	initCtx, cancel := context.WithTimeout(ctx, m.initializeTimeout)
	defer cancel()

	response, err := process.Initialize(initCtx, m.initializeRuntimeRequest(ext, runtime, resourceSession))
	if err != nil {
		return subprocess.InitializeResponse{}, fmt.Errorf("initialize subprocess: %w", err)
	}
	return response, nil
}

func (m *Manager) initializeRuntimeRequest(
	ext *managedExtension,
	runtime subprocess.InitializeRuntime,
	resourceSession *hostAPIResourceSession,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          m.protocolVersion,
		SupportedProtocolVersion: slices.Clone(m.supportedProtocolVersions),
		AGHVersion:               version.Current().Version,
		SessionNonce:             resourceSession.Actor.SessionNonce,
		Extension: subprocess.InitializeExtension{
			Name:       ext.manifest.Name,
			Version:    ext.manifest.Version,
			SourceTier: ext.info.Source.String(),
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides:              normalizeUniqueStrings(ext.manifest.Capabilities.Provides),
			GrantedActions:        hostAPIMethodsFromStrings(ext.grantedActions),
			GrantedSecurity:       normalizeUniqueStrings(ext.grantedSecurity),
			GrantedResourceKinds:  resourceKindStrings(ext.grantedResourceKinds),
			GrantedResourceScopes: resourceScopeStrings(ext.grantedResourceScopes),
		},
		Methods: subprocess.InitializeMethods{
			DaemonRequests:    daemonRequestMethods(),
			ExtensionServices: capabilityMethods(ext.manifest.Capabilities.Provides),
		},
		Runtime: runtime,
	}
}

func (m *Manager) launchConfigFor(
	ctx context.Context,
	ext *managedExtension,
) (subprocess.LaunchConfig, subprocess.InitializeRuntime, time.Duration, []func(), error) {
	if ext.manifest == nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, 0, nil, errors.New("manifest is required")
	}

	command, err := m.resolveCommand(ext.rootDir, ext.manifest.Subprocess.Command)
	if err != nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, 0, nil, err
	}
	args, err := m.resolveStringSlice(ext.rootDir, ext.manifest.Subprocess.Args)
	if err != nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, 0, nil, err
	}
	env, cleanups, err := m.resolveEnvMap(
		ctx,
		ext.rootDir,
		ext.manifest.Subprocess.Env,
		ext.manifest.Subprocess.SecretEnv,
	)
	if err != nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, 0, nil, err
	}

	healthInterval := durationOr(ext.manifest.Subprocess.HealthCheckInterval, defaultHealthCheckInterval)
	shutdownTimeout := durationOr(ext.manifest.Subprocess.ShutdownTimeout, m.defaultShutdownTimeout)
	bridgeRuntime, err := m.resolveBridgeRuntime(ctx, ext)
	if err != nil {
		runExtensionRedactionCleanups(cleanups)
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, 0, nil, err
	}
	runtime := subprocess.InitializeRuntime{
		HealthCheckIntervalMS: healthInterval.Milliseconds(),
		HealthCheckTimeoutMS:  m.healthCheckTimeout.Milliseconds(),
		ShutdownTimeoutMS:     shutdownTimeout.Milliseconds(),
		DefaultHookTimeoutMS:  m.defaultHookTimeout.Milliseconds(),
		Bridge:                bridgeRuntime,
	}

	launchCfg := subprocess.LaunchConfig{
		Command:         command,
		Args:            args,
		Dir:             ext.rootDir,
		Env:             env,
		Logger:          m.logger,
		ShutdownTimeout: shutdownTimeout,
		PostSignalGrace: m.subprocessSignalGrace,
		ProcessRegistry: m.processRegistry,
		ProcessRecord: toolruntime.RegisterConfig{
			Source: toolruntime.ProcessSourceExtension,
			Owner: toolruntime.ProcessOwner{
				ExtensionName: ext.info.Name,
			},
		},
	}
	return launchCfg, runtime, healthInterval, cleanups, nil
}

func (m *Manager) wrapHostHandler(
	extName string,
	method string,
	bridgeRuntime *subprocess.InitializeBridgeRuntime,
	resourceSession *hostAPIResourceSession,
	handler subprocess.HandlerFunc,
) subprocess.HandlerFunc {
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := m.capChecker.CheckHostAPI(extName, method); err != nil {
			return nil, rpcCapabilityDenied(err)
		}

		hostCtx := withHostAPIExtensionName(ctx, extName)
		if bridgeRuntime != nil {
			hostCtx = withHostAPIBridgeRuntime(hostCtx, bridgeRuntime)
		}
		if resourceSession != nil {
			hostCtx = withHostAPIResourceSession(hostCtx, resourceSession)
		}
		return handler(hostCtx, params)
	}
}

func (m *Manager) newHostAPIResourceSession(ext *managedExtension) (*hostAPIResourceSession, error) {
	if ext == nil {
		return nil, errors.New("extension: managed extension is required")
	}

	sessionNonce, err := newExtensionSessionNonce()
	if err != nil {
		return nil, fmt.Errorf("extension: generate session nonce for %q: %w", ext.info.Name, err)
	}

	return &hostAPIResourceSession{
		Actor: resources.MutationActor{
			Kind:         resources.MutationActorKindExtension,
			ID:           ext.info.Name,
			SessionNonce: sessionNonce,
			Source:       extensionResourceSource(ext.info.Name),
			// Workspace binding is not yet carried on extension sessions, so v1
			// relies on granted scope kinds for narrowing and keeps the max scope
			// ceiling global here.
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			GrantedKinds: append(
				[]resources.ResourceKind(nil),
				ext.grantedResourceKinds...,
			),
			GrantedScopes: append(
				[]resources.ResourceScopeKind(nil),
				ext.grantedResourceScopes...,
			),
		},
	}, nil
}

func (m *Manager) activateExtensionSourceSession(ctx context.Context, actor resources.MutationActor) error {
	if m == nil || m.sourceSessions == nil {
		return nil
	}

	if err := m.sourceSessions.ActivateSourceSession(
		ctx,
		extensionManagerResourceActor(),
		actor.Source,
		actor.SessionNonce,
	); err != nil {
		return fmt.Errorf("extension: activate source session for %q: %w", actor.Source.ID, err)
	}
	return nil
}

func extensionManagerResourceActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "extension-manager",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "extension-manager",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
}

func extensionResourceSource(extensionName string) resources.ResourceSource {
	return resources.ResourceSource{
		Kind: resources.ResourceSourceKind(managerExtensionKey),
		ID:   extensionName,
	}
}

func newExtensionSessionNonce() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
