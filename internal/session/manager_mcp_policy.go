package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

// Root resource budgets describe the current configuration. Delegated sessions
// and instance ceilings retain their explicitly authorized server allowlist.
func (m *Manager) refreshRootMCPPolicy(ctx context.Context, session *Session) error {
	meta := session.Meta()
	lineage := store.NormalizeSessionLineage(meta.ID, meta.Lineage)
	if store.HierarchyParentSessionID(lineage) != "" || !lineage.PermissionPolicy.MaterializedMCPServers ||
		session.Info().InstanceID != "" ||
		(normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser &&
			normalizeSessionType(Type(meta.SessionType)) != SessionTypeSystem) {
		return nil
	}
	plan, err := m.currentMCPRuntimePlan(ctx, session)
	if err != nil {
		return err
	}
	spec := &plan.spec
	if spec.instancePermissionCeiling != nil {
		return nil
	}
	names := spec.lineage.PermissionPolicy.MCPServers
	if slices.Equal(names, lineage.PermissionPolicy.MCPServers) {
		return nil
	}
	session.mu.Lock()
	session.Lineage = store.NormalizeSessionLineage(session.ID, session.Lineage)
	session.Lineage.PermissionPolicy.MCPServers = slices.Clone(names)
	session.mu.Unlock()
	return m.persistSessionLifecycleState(ctx, session, false)
}

// ErrHostedMCPBindMissing identifies a lost proxy binding that requires native-session recovery.
var ErrHostedMCPBindMissing = errors.New("session: hosted MCP bind is missing")

// Hosts that cache tools reload the existing native session at a turn boundary.
type hostedMCPToolState interface {
	WaitForBindToolDigest(context.Context, string) (string, error)
	ToolDigest(context.Context, string) (string, error)
	InitialToolDigest(string) (string, bool)
}

func (m *Manager) captureHostedMCPDigest(ctx context.Context, session *Session) error {
	state, ok := m.hostedMCP.(hostedMCPToolState)
	if !ok {
		return m.hostedMCP.WaitForBind(ctx, session.ID)
	}
	digest, err := state.WaitForBindToolDigest(ctx, session.ID)
	if err != nil {
		return fmt.Errorf("session: confirm MCP projection: %w", err)
	}
	session.mu.Lock()
	session.hostedMCPRequiresReload = true
	session.hostedMCPDigest = digest
	session.mu.Unlock()
	return nil
}

func (m *Manager) hostedMCPToolsChanged(ctx context.Context, session *Session) bool {
	if session.processHandle() == nil {
		return false
	}
	session.mu.RLock()
	before, reload := session.hostedMCPDigest, session.hostedMCPRequiresReload
	directBefore := session.directMCPDigest
	session.mu.RUnlock()
	if m.hostedMCP == nil && directBefore != "" {
		return m.directMCPConfigurationChanged(ctx, session, directBefore)
	}
	if !reload {
		return false
	}
	state, ok := m.hostedMCP.(hostedMCPToolState)
	if !ok {
		return false
	}
	if initial, ready := state.InitialToolDigest(session.ID); ready {
		before = initial
		session.mu.Lock()
		session.hostedMCPDigest = initial
		session.mu.Unlock()
	} else if before == "" {
		// Some hosts establish MCP only during their first prompt.
		return false
	}
	digest, err := state.ToolDigest(ctx, session.ID)
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		// A lost proxy must be reloaded; failing admission here would strand every later turn.
		return errors.Is(err, ErrHostedMCPBindMissing)
	}
	return digest != before
}

func (m *Manager) directMCPConfigurationChanged(ctx context.Context, session *Session, before string) bool {
	plan, err := m.currentMCPRuntimePlan(ctx, session)
	if err != nil {
		return false
	}
	servers, err := m.sessionMCPServers(ctx, &plan.spec, plan.runtime.agent, plan.runtime.agentDef)
	return err == nil && ctx.Err() == nil && directMCPDigest(servers) != before
}

// Match the accepted fallback route when checking its MCP servers, rather than re-resolving the primary account.
func (m *Manager) currentMCPRuntimePlan(ctx context.Context, session *Session) (*promptRuntimePlan, error) {
	binding := session.runtimeBindingSnapshot()
	return m.preparePromptRuntimePlanForRoute(
		ctx,
		session,
		binding.selection,
		binding.acceptedCommand,
		binding.acceptedSessionEnv,
	)
}

// The launch fingerprint stays in memory; neither it nor config secrets are logged or persisted.
func directMCPDigest(servers []compozyconfig.MCPServer) string {
	servers = slices.Clone(servers)
	slices.SortFunc(servers, func(a, b compozyconfig.MCPServer) int {
		return strings.Compare(a.EffectiveRuntimeName(), b.EffectiveRuntimeName())
	})
	data, err := json.Marshal(servers)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
