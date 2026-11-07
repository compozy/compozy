package session

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/store"
)

// Root resource budgets describe the current configuration. Delegated sessions
// and instance ceilings retain their explicitly authorized server allowlist.
func (m *Manager) refreshRootMCPPolicy(ctx context.Context, session *Session) error {
	meta := session.Meta()
	lineage := store.NormalizeSessionLineage(meta.ID, meta.Lineage)
	if lineage.Kind != store.LineageKindRoot || session.Info().InstanceID != "" ||
		(normalizeSessionType(Type(meta.SessionType)) != SessionTypeUser &&
			normalizeSessionType(Type(meta.SessionType)) != SessionTypeSystem) {
		return nil
	}
	workspace, err := m.resolveResumeWorkspace(ctx, &meta)
	if err != nil {
		return fmt.Errorf("session: refresh MCP policy workspace: %w", err)
	}
	spec, err := sessionStartSpecFromMeta(&meta, &workspace, meta.CWD)
	if err != nil {
		return err
	}
	runtime, err := m.resolveSessionStartRuntime(ctx, &spec, false)
	if err != nil {
		return err
	}
	if spec.instancePermissionCeiling != nil {
		return nil
	}
	if err := spec.materializeRootDelegationPolicy(
		runtime.agentDef,
		runtime.agent,
		m.toolsetCatalog,
		m.toolUniverse,
	); err != nil {
		return err
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

var ErrHostedMCPBindMissing = errors.New("session: hosted MCP bind is missing")

// Codex currently caches tools for the lifetime of its MCP connection (upstream
// openai/codex#10105 and #33266). Remove this adapter once it re-lists on change.
type hostedMCPToolState interface {
	WaitForBindToolDigest(context.Context, string) (string, error)
	ToolDigest(context.Context, string) (string, error)
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
	session.mu.RUnlock()
	if !reload || before == "" {
		return false
	}
	state, ok := m.hostedMCP.(hostedMCPToolState)
	if !ok {
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
