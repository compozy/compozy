// Package local implements the daemon-host execution sandbox provider.
package local

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/toolruntime"
)

var _ sandbox.Provider = (*localProvider)(nil)

// Option customizes the local provider.
type Option func(*localProvider)

type localProvider struct {
	logger          *slog.Logger
	stopTimeout     time.Duration
	permissionMode  aghconfig.PermissionMode
	processRegistry *toolruntime.Registry
}

// WithLogger directs provider-created launcher and tool-host diagnostics to logger.
func WithLogger(logger *slog.Logger) Option {
	return func(provider *localProvider) {
		provider.logger = logger
	}
}

// WithStopTimeout configures how long local launcher stop waits before escalation.
func WithStopTimeout(timeout time.Duration) Option {
	return func(provider *localProvider) {
		provider.stopTimeout = timeout
	}
}

// WithPermissionMode configures the local tool host permission policy.
func WithPermissionMode(mode aghconfig.PermissionMode) Option {
	return func(provider *localProvider) {
		provider.permissionMode = mode
	}
}

// WithProcessRegistry injects the shared process registry used for tool-host terminals.
func WithProcessRegistry(registry *toolruntime.Registry) Option {
	return func(provider *localProvider) {
		provider.processRegistry = registry
	}
}

// NewProvider returns the local daemon-host sandbox provider.
func NewProvider(opts ...Option) sandbox.Provider {
	provider := &localProvider{
		logger:         slog.Default(),
		permissionMode: aghconfig.PermissionModeApproveReads,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(provider)
		}
	}
	if provider.logger == nil {
		provider.logger = slog.Default()
	}
	return provider
}

// NewRegistry returns a provider registry with local registered as the default backend.
func NewRegistry(opts ...Option) (*sandbox.Registry, error) {
	return sandbox.NewRegistry(NewProvider(opts...))
}

func (p *localProvider) Backend() sandbox.Backend {
	return sandbox.BackendLocal
}

func (p *localProvider) Prepare(
	ctx context.Context,
	req sandbox.PrepareRequest,
) (sandbox.Prepared, error) {
	if ctx == nil {
		return sandbox.Prepared{}, errors.New("sandbox/local: prepare context is required")
	}

	launcher := acp.NewLocalLauncher(p.logger, p.stopTimeout)
	toolHost, err := acp.NewLocalToolHost(
		ctx,
		req.LocalRootDir,
		p.permissionModeFor(req),
		p.logger,
		acp.WithLocalProcessRegistry(p.processRegistry),
		acp.WithLocalAdditionalRoots(req.LocalAdditionalDirs...),
	)
	if err != nil {
		return sandbox.Prepared{}, fmt.Errorf("sandbox/local: create tool host: %w", err)
	}

	runtimeAdditionalDirs := cloneStrings(req.LocalAdditionalDirs)
	launchAdditionalDirs := cloneStrings(runtimeAdditionalDirs)
	agentEnv := cloneStrings(req.AgentEnv)
	preparedState := sandbox.SessionState{
		SandboxID:             req.SandboxID,
		Backend:               sandbox.BackendLocal,
		Profile:               req.Sandbox.Profile,
		InstanceID:            strings.TrimSpace(req.InstanceID),
		RuntimeRootDir:        req.LocalRootDir,
		RuntimeAdditionalDirs: cloneStrings(runtimeAdditionalDirs),
		ProviderState:         cloneRawMessage(req.ProviderState),
		PreparedAt:            time.Now().UTC(),
	}

	return sandbox.Prepared{
		State:                 preparedState,
		RuntimeRootDir:        req.LocalRootDir,
		RuntimeAdditionalDirs: runtimeAdditionalDirs,
		Launcher:              launcher,
		Launch: sandbox.LaunchSpec{
			Command:        req.AgentCommand,
			Cwd:            req.LocalRootDir,
			AdditionalDirs: launchAdditionalDirs,
			Env:            agentEnv,
		},
		ToolHost: toolHost,
	}, nil
}

func (p *localProvider) SyncToRuntime(
	ctx context.Context,
	_ sandbox.SessionState,
	_ sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	if ctx == nil {
		return sandbox.SyncResult{}, errors.New("sandbox/local: sync to runtime context is required")
	}
	return sandbox.SyncResult{}, nil
}

func (p *localProvider) SyncFromRuntime(
	ctx context.Context,
	_ sandbox.SessionState,
	_ sandbox.SyncOptions,
) (sandbox.SyncResult, error) {
	if ctx == nil {
		return sandbox.SyncResult{}, errors.New("sandbox/local: sync from runtime context is required")
	}
	return sandbox.SyncResult{}, nil
}

func (p *localProvider) Destroy(ctx context.Context, _ sandbox.SessionState) error {
	if ctx == nil {
		return errors.New("sandbox/local: destroy context is required")
	}
	return nil
}

func (p *localProvider) permissionModeFor(req sandbox.PrepareRequest) aghconfig.PermissionMode {
	if mode := strings.TrimSpace(req.Permissions); mode != "" {
		return aghconfig.PermissionMode(mode)
	}
	return p.permissionMode
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneRawMessage(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	cloned := make(json.RawMessage, len(value))
	copy(cloned, value)
	return cloned
}
