package extensiontest

import (
	"context"

	"fmt"

	"strings"
	"sync/atomic"
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"

	aghconfig "github.com/compozy/agh/internal/config"

	observepkg "github.com/compozy/agh/internal/observe"

	"github.com/compozy/agh/internal/session"

	"github.com/compozy/agh/internal/subprocess"
	aghtestutil "github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/testutil/acpmock"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (h *Harness) stopSessions(t testing.TB) {
	t.Helper()
	for _, info := range h.Sessions.List() {
		if info == nil {
			continue
		}
		if err := h.Sessions.Stop(aghtestutil.Context(t), info.ID); err != nil {
			t.Fatalf("Sessions.Stop(%q) error = %v", info.ID, err)
		}
	}
}

type harnessBridgeSource struct {
	service *bridgepkg.Service
	broker  *bridgepkg.Broker
}

type deferredBridgeTelemetrySink struct {
	observer *observepkg.Observer
}

func (s *deferredBridgeTelemetrySink) RecordBridgeAuthFailure(bridgeInstanceID string) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.RecordBridgeAuthFailure(bridgeInstanceID)
}

func (s *deferredBridgeTelemetrySink) RecordBridgeRuntimeIssue(
	bridgeInstanceID string,
	status bridgepkg.BridgeStatus,
	message string,
) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.RecordBridgeRuntimeIssue(bridgeInstanceID, status, message)
}

func (s *deferredBridgeTelemetrySink) ClearBridgeRuntimeIssue(bridgeInstanceID string) {
	if s == nil || s.observer == nil {
		return
	}
	s.observer.ClearBridgeRuntimeIssue(bridgeInstanceID)
}

func (s harnessBridgeSource) ListInstances(ctx context.Context) ([]bridgepkg.BridgeInstance, error) {
	if s.service == nil {
		return nil, nil
	}
	return s.service.ListInstances(ctx)
}

func (s harnessBridgeSource) ListRoutes(ctx context.Context, bridgeInstanceID string) ([]bridgepkg.BridgeRoute, error) {
	if s.service == nil {
		return nil, nil
	}
	return s.service.ListRoutes(ctx, bridgeInstanceID)
}

type staticWorkspaceResolver struct {
	resolved workspacepkg.ResolvedWorkspace
}

func (r *staticWorkspaceResolver) Resolve(_ context.Context, idOrPath string) (workspacepkg.ResolvedWorkspace, error) {
	if r == nil {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	trimmed := strings.TrimSpace(idOrPath)
	if trimmed == "" {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	if trimmed == r.resolved.ID || trimmed == r.resolved.WorkspaceID || trimmed == r.resolved.RootDir {
		return r.resolved, nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *staticWorkspaceResolver) ResolveOrRegister(
	_ context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	if r == nil {
		return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == r.resolved.RootDir || trimmed == r.resolved.WorkspaceID || trimmed == r.resolved.ID {
		return r.resolved, nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

type stubBridgeRuntimeResolver struct {
	runtimes map[string]*subprocess.InitializeBridgeRuntime
}

func (r *stubBridgeRuntimeResolver) ResolveBridgeRuntime(
	_ context.Context,
	extensionName string,
) (*subprocess.InitializeBridgeRuntime, error) {
	if r == nil || r.runtimes == nil {
		return nil, nil
	}
	return subprocess.CloneInitializeBridgeRuntime(r.runtimes[strings.TrimSpace(extensionName)]), nil
}

func defaultResolvedWorkspace(root string, now time.Time) workspacepkg.ResolvedWorkspace {
	return workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:           "ws-bridge-adapter",
			RootDir:      root,
			Name:         "bridge-adapter-workspace",
			DefaultAgent: bridgeAdapterHarnessCoderKey,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		WorkspaceID: "ws-bridge-adapter",
		Config: aghconfig.Config{
			Defaults: aghconfig.DefaultsConfig{Agent: bridgeAdapterHarnessCoderKey},
			Providers: map[string]aghconfig.ProviderConfig{
				acpmock.ProviderName: acpmock.ProviderConfig("fake-agent"),
			},
			Permissions: aghconfig.PermissionsConfig{Mode: aghconfig.PermissionModeApproveAll},
		},
		Agents: []aghconfig.AgentDef{{
			Name:        bridgeAdapterHarnessCoderKey,
			Provider:    acpmock.ProviderName,
			Permissions: string(aghconfig.PermissionModeApproveAll),
			Prompt:      "You are a reliable coder.",
		}},
		ResolvedAt: now,
	}
}

func cloneBoundSecrets(src []subprocess.InitializeBridgeBoundSecret) []subprocess.InitializeBridgeBoundSecret {
	if len(src) == 0 {
		return nil
	}
	return append([]subprocess.InitializeBridgeBoundSecret(nil), src...)
}

func cloneManagedRuntime(
	src []subprocess.InitializeBridgeManagedInstance,
) []subprocess.InitializeBridgeManagedInstance {
	if len(src) == 0 {
		return nil
	}
	cloned := make([]subprocess.InitializeBridgeManagedInstance, 0, len(src))
	for _, managed := range src {
		item := managed
		item.BoundSecrets = cloneBoundSecrets(item.BoundSecrets)
		cloned = append(cloned, item)
	}
	return cloned
}

func sequentialIDGenerator(prefix string) session.IDGenerator {
	var counter atomic.Int64
	return func() string {
		return fmt.Sprintf("%s-%d", prefix, counter.Add(1))
	}
}
