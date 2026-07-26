package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/compozy/agh/internal/diagnostics"
	redactpkg "github.com/compozy/agh/internal/redact"
	"github.com/compozy/agh/internal/subprocess"
	"github.com/compozy/agh/internal/toolruntime"
	"github.com/compozy/agh/internal/version"
)

func (m *Manager) executeBridgeControl(
	ctx context.Context,
	extension bridgeControlExtension,
	bridgeRuntime *subprocess.InitializeBridgeRuntime,
	method bridgepkg.ControlMethod,
	call bridgeControlProcessCall,
) (resultErr error) {
	launchConfig, runtime, cleanups, err := m.bridgeControlLaunchConfig(ctx, extension, bridgeRuntime)
	if err != nil {
		return err
	}
	defer func() { runExtensionRedactionCleanups(cleanups) }()

	process, err := m.launch(ctx, launchConfig)
	if err != nil {
		return redactedBridgeControlError("launch transient bridge control process", err)
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.defaultShutdownTimeout)
		defer cancel()
		if cleanupErr := process.Shutdown(cleanupCtx); cleanupErr != nil {
			resultErr = errors.Join(
				resultErr,
				redactedBridgeControlError("shutdown transient bridge control process", cleanupErr),
			)
		}
	}()

	sessionNonce, err := newExtensionSessionNonce()
	if err != nil {
		return fmt.Errorf("extension: generate bridge control session nonce: %w", err)
	}
	request := m.bridgeControlInitializeRequest(extension, runtime, sessionNonce, method)
	initCtx, cancelInitialize := context.WithTimeout(ctx, m.initializeTimeout)
	_, err = process.Initialize(initCtx, request)
	cancelInitialize()
	if err != nil {
		return redactedBridgeControlError("initialize transient bridge control process", err)
	}

	callCtx, cancelCall := context.WithTimeout(ctx, m.defaultHookTimeout)
	err = call(callCtx, process)
	cancelCall()
	if err != nil {
		return redactedBridgeControlError("call transient bridge control process", err)
	}
	return nil
}

func (m *Manager) bridgeControlLaunchConfig(
	ctx context.Context,
	extension bridgeControlExtension,
	bridgeRuntime *subprocess.InitializeBridgeRuntime,
) (subprocess.LaunchConfig, subprocess.InitializeRuntime, []func(), error) {
	command, err := m.resolveCommand(extension.rootDir, extension.manifest.Subprocess.Command)
	if err != nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, nil, err
	}
	args, err := m.resolveStringSlice(extension.rootDir, extension.manifest.Subprocess.Args)
	if err != nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, nil, err
	}
	env, cleanups, err := m.resolveEnvMap(
		ctx,
		extension.rootDir,
		extension.manifest.Subprocess.Env,
		extension.manifest.Subprocess.SecretEnv,
	)
	if err != nil {
		return subprocess.LaunchConfig{}, subprocess.InitializeRuntime{}, nil, err
	}
	cleanups = append(cleanups, registerBridgeControlSecrets(bridgeRuntime)...)

	healthInterval := durationOr(
		extension.manifest.Subprocess.HealthCheckInterval,
		defaultHealthCheckInterval,
	)
	shutdownTimeout := durationOr(extension.manifest.Subprocess.ShutdownTimeout, m.defaultShutdownTimeout)
	runtime := subprocess.InitializeRuntime{
		HealthCheckIntervalMS: healthInterval.Milliseconds(),
		HealthCheckTimeoutMS:  m.healthCheckTimeout.Milliseconds(),
		ShutdownTimeoutMS:     shutdownTimeout.Milliseconds(),
		DefaultHookTimeoutMS:  m.defaultHookTimeout.Milliseconds(),
		Bridge:                subprocess.CloneInitializeBridgeRuntime(bridgeRuntime),
	}
	launchConfig := subprocess.LaunchConfig{
		Command:         command,
		Args:            args,
		Dir:             extension.rootDir,
		Env:             env,
		Logger:          m.logger,
		ShutdownTimeout: shutdownTimeout,
		PostSignalGrace: m.subprocessSignalGrace,
		ProcessRegistry: m.processRegistry,
		ProcessRecord: toolruntime.RegisterConfig{
			Source: toolruntime.ProcessSourceExtension,
			Owner:  toolruntime.ProcessOwner{ExtensionName: extension.info.Name},
		},
	}
	return launchConfig, runtime, cleanups, nil
}

func (m *Manager) bridgeControlInitializeRequest(
	extension bridgeControlExtension,
	runtime subprocess.InitializeRuntime,
	sessionNonce string,
	method bridgepkg.ControlMethod,
) subprocess.InitializeRequest {
	return subprocess.InitializeRequest{
		ProtocolVersion:          m.protocolVersion,
		SupportedProtocolVersion: slices.Clone(m.supportedProtocolVersions),
		AGHVersion:               version.Current().Version,
		SessionNonce:             sessionNonce,
		Extension: subprocess.InitializeExtension{
			Name:       extension.manifest.Name,
			Version:    extension.manifest.Version,
			SourceTier: extension.info.Source.String(),
		},
		Methods: subprocess.InitializeMethods{
			DaemonRequests:    []string{managerShutdownKey},
			ExtensionServices: []string{string(method)},
		},
		Runtime: runtime,
	}
}

func registerBridgeControlSecrets(runtime *subprocess.InitializeBridgeRuntime) []func() {
	if runtime == nil {
		return nil
	}
	cleanups := make([]func(), 0)
	for _, managed := range runtime.ManagedInstances {
		for _, secret := range managed.BoundSecrets {
			cleanups = append(cleanups, diagnostics.RegisterDynamicSecret(secret.Value))
		}
	}
	return cleanups
}

type bridgeControlRedactedError struct {
	message string
	cause   error
}

func redactedBridgeControlError(operation string, cause error) error {
	if cause == nil {
		return nil
	}
	return &bridgeControlRedactedError{
		message: redactpkg.String(fmt.Sprintf("extension: %s: %s", strings.TrimSpace(operation), cause.Error())),
		cause:   cause,
	}
}

func (e *bridgeControlRedactedError) Error() string {
	return e.message
}

func (e *bridgeControlRedactedError) Unwrap() error {
	return e.cause
}
