package extensionpkg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"slices"
	"strings"

	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/resources"

	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/toolruntime"

	"github.com/compozy/compozy/internal/version"
)

func (m *Manager) launchStartupRuntime(
	ctx context.Context,
	ext *managedExtension,
	grant EffectiveGrant,
	transaction *extensionStartupTransaction,
) (launchedRuntime, *hostAPIResourceSession, error) {
	launchCfg, runtime, healthInterval, cleanups, err := m.launchConfigFor(ctx, ext)
	if err != nil {
		return launchedRuntime{}, nil, err
	}
	ownership := m.newStartupRuntimeOwnership(ext.instanceKey(), nil, cleanups)
	transaction.add("redaction cleanup", func(context.Context) error {
		return ownership.releaseRedaction()
	})

	process, err := m.launch(ctx, launchCfg)
	if err != nil {
		return launchedRuntime{}, nil, fmt.Errorf(
			"launch subprocess: %w",
			err,
		)
	}
	ownership.process = process
	transaction.add("subprocess", ownership.stop)

	resourceSession, err := m.newHostAPIResourceSessionWithGrant(ctx, ext, grant)
	if err != nil {
		return launchedRuntime{}, nil, err
	}
	capabilityGrantID := extensionCapabilityGrantID(ext.instanceKey(), resourceSession.Actor.SessionNonce)
	registeredGrant, err := m.capChecker.RegisterForSession(
		capabilityGrantID,
		ext.info.Source,
		ext.manifest,
		ext.maxResourceScope(),
	)
	if err != nil {
		return launchedRuntime{}, nil, fmt.Errorf("register candidate capability grant: %w", err)
	}
	transaction.add("capability grant", func(context.Context) error {
		m.capChecker.Unregister(capabilityGrantID)
		return nil
	})
	resourceSession.Actor.ID = capabilityGrantID
	resourceSession.Actor.GrantedKinds = slices.Clone(registeredGrant.ResourceKinds)
	resourceSession.Actor.GrantedScopes = slices.Clone(registeredGrant.ResourceScopes)
	if err := m.registerRuntimeHostMethods(process, ext, runtime, resourceSession); err != nil {
		return launchedRuntime{}, nil, err
	}

	response, err := m.initializeRuntimeProcess(ctx, process, ext, registeredGrant, runtime, resourceSession)
	if err != nil {
		return launchedRuntime{}, nil, err
	}
	if err := validateSupportedHookEvents(response.SupportedHookEvents); err != nil {
		return launchedRuntime{}, nil, err
	}

	return launchedRuntime{
		process:           process,
		response:          response,
		runtime:           runtime,
		healthInterval:    healthInterval,
		sessionNonce:      resourceSession.Actor.SessionNonce,
		capabilityGrantID: capabilityGrantID,
		redactionCleanups: ownership.redactionCleanups,
	}, resourceSession, nil
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
			m.wrapHostHandler(
				ext.instanceKey(),
				method,
				runtime.Bridge,
				resourceSession,
				handler,
			),
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
	grant EffectiveGrant,
	runtime subprocess.InitializeRuntime,
	resourceSession *hostAPIResourceSession,
) (subprocess.InitializeResponse, error) {
	initCtx, cancel := context.WithTimeout(ctx, m.initializeTimeout)
	defer cancel()

	response, err := process.Initialize(initCtx, m.initializeRuntimeRequest(ext, grant, runtime, resourceSession))
	if err != nil {
		return subprocess.InitializeResponse{}, fmt.Errorf("initialize subprocess: %w", err)
	}
	return response, nil
}

func (m *Manager) initializeRuntimeRequest(
	ext *managedExtension,
	grant EffectiveGrant,
	runtime subprocess.InitializeRuntime,
	resourceSession *hostAPIResourceSession,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          m.protocolVersion,
		SupportedProtocolVersion: slices.Clone(m.supportedProtocolVersions),
		CompozyVersion:           version.Current().Version,
		SessionNonce:             resourceSession.Actor.SessionNonce,
		Extension: subprocess.InitializeExtension{
			Name:       ext.manifest.Name,
			Version:    ext.manifest.Version,
			SourceTier: ext.info.Source.String(),
		},
		Capabilities: subprocess.InitializeCapabilities{
			Provides:              append([]string{}, normalizeUniqueStrings(ext.manifest.Capabilities.Provides)...),
			GrantedPermissions:    hostAPIMethodsFromStrings(grant.Permissions),
			GrantedResourceKinds:  resourceKindStrings(grant.ResourceKinds),
			GrantedResourceScopes: resourceScopeStrings(grant.ResourceScopes),
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
	env, cleanups, err := m.resolveInstanceEnvMap(
		ctx,
		ext.instanceKey(),
		ext.manifest.RequiresEnv,
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
		DefaultViewTimeoutMS:  m.defaultViewTimeout.Milliseconds(),
		Bridge:                bridgeRuntime,
	}

	launchCfg := subprocess.LaunchConfig{
		Command: command,
		Args:    args,
		Dir:     ext.rootDir,
		Env:     env,
		Logger:  m.logger,
		StderrSink: extensionLogWriter{
			ring:           ext.logRing,
			generationHash: ext.generationHash,
		},
		StderrTransform: diagnostics.Redact,
		ShutdownTimeout: shutdownTimeout,
		PostSignalGrace: m.subprocessSignalGrace,
		ProcessRegistry: m.processRegistry,
		ProcessRecord: toolruntime.RegisterConfig{
			Source: toolruntime.ProcessSourceExtension,
			Owner: toolruntime.ProcessOwner{
				ExtensionName: ext.instanceKey().runtimeID(),
			},
		},
	}
	return launchCfg, runtime, healthInterval, cleanups, nil
}

func (m *Manager) wrapHostHandler(
	instance any,
	method string,
	bridgeRuntime *subprocess.InitializeBridgeRuntime,
	resourceSession *hostAPIResourceSession,
	handler subprocess.HandlerFunc,
) subprocess.HandlerFunc {
	key := instanceKeyFromAny(instance)
	capabilityGrantID := key.runtimeID()
	if resourceSession != nil && resourceSession.Actor.ID != "" {
		capabilityGrantID = resourceSession.Actor.ID
	}
	return func(ctx context.Context, params json.RawMessage) (any, error) {
		if err := m.capChecker.CheckHostAPI(capabilityGrantID, method); err != nil {
			return nil, rpcCapabilityDenied(err)
		}

		hostCtx := withHostAPIExtensionName(ctx, key.Name)
		hostCtx = withHostAPICapabilityGrantID(hostCtx, capabilityGrantID)
		if bridgeRuntime != nil {
			hostCtx = withHostAPIBridgeRuntime(hostCtx, bridgeRuntime)
		}
		if resourceSession != nil {
			hostCtx = withHostAPIResourceSession(hostCtx, resourceSession)
		}
		return handler(hostCtx, params)
	}
}

func (m *Manager) newHostAPIResourceSession(
	ctx context.Context,
	ext *managedExtension,
) (*hostAPIResourceSession, error) {
	return m.newHostAPIResourceSessionWithGrant(ctx, ext, ext.pendingGrant)
}

func (m *Manager) newHostAPIResourceSessionWithGrant(
	ctx context.Context,
	ext *managedExtension,
	grant EffectiveGrant,
) (*hostAPIResourceSession, error) {
	if ext == nil {
		return nil, errors.New("extension: managed extension is required")
	}

	sessionNonce, err := newExtensionSessionNonce()
	if err != nil {
		return nil, fmt.Errorf("extension: generate session nonce for %q: %w", ext.info.Name, err)
	}

	maxScope, err := m.extensionWorkspaceScope(ctx, ext)
	if err != nil {
		return nil, err
	}

	return &hostAPIResourceSession{
		Actor: resources.MutationActor{
			Kind:         resources.MutationActorKindExtension,
			ID:           ext.instanceKey().runtimeID(),
			SessionNonce: sessionNonce,
			Source:       extensionResourceSource(ext.instanceKey()),
			MaxScope:     maxScope,
			GrantedKinds: append(
				[]resources.ResourceKind(nil),
				grant.ResourceKinds...,
			),
			GrantedScopes: append(
				[]resources.ResourceScopeKind(nil),
				grant.ResourceScopes...,
			),
		},
	}, nil
}

func (m *Manager) extensionWorkspaceScope(
	ctx context.Context,
	ext *managedExtension,
) (resources.ResourceScope, error) {
	key := ext.instanceKey()
	if !key.IsGlobal() {
		return resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   key.WorkspaceID,
		}, nil
	}
	switch ext.info.Source {
	case SourceBundled, SourceUser, SourceMarketplace:
		// Marketplace artifacts are installed once under the daemon home, not
		// owned by any project workspace. Their read-oriented capability ceiling
		// is the trust boundary; the runtime principal is explicitly global.
		return resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, nil
	case SourceWorkspace:
		if m.workspaceResolver == nil {
			return resources.ResourceScope{}, errors.New(
				"extension: workspace-scoped extension requires a workspace resolver",
			)
		}
		resolved, err := m.workspaceResolver.Resolve(ctx, ext.rootDir)
		if err != nil {
			return resources.ResourceScope{}, fmt.Errorf(
				"extension: bind %s extension %q to the workspace owning %q: %w",
				ext.info.Source,
				ext.info.Name,
				ext.rootDir,
				err,
			)
		}
		workspaceID := strings.TrimSpace(resolved.ID)
		if workspaceID == "" {
			return resources.ResourceScope{}, fmt.Errorf(
				"extension: bind %s extension %q: resolved workspace id is empty",
				ext.info.Source,
				ext.info.Name,
			)
		}
		return resources.ResourceScope{
			Kind: resources.ResourceScopeKindWorkspace,
			ID:   workspaceID,
		}, nil
	default:
		return resources.ResourceScope{}, fmt.Errorf(
			"extension: bind extension %q: unsupported source %d",
			ext.info.Name,
			ext.info.Source,
		)
	}
}

func extensionManagerResourceActor() resources.MutationActor {
	return resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "extension-manager",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "extension-manager",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}
}

func extensionResourceSource(instance any) resources.ResourceSource {
	var key InstanceKey
	switch value := instance.(type) {
	case InstanceKey:
		key = value.Normalize()
	case string:
		key = GlobalInstanceKey(value)
	}
	return resources.ResourceSource{
		Kind: resources.ResourceSourceKind(managerExtensionKey),
		ID:   key.runtimeID(),
	}
}

func newExtensionSessionNonce() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}
