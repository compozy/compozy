package session

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/store"
)

// MCPAuthorityResolver resolves authority using only the session row and its live scope.
type MCPAuthorityResolver interface {
	EffectiveMCPAuthority(context.Context, string) (store.SessionPermissionPolicy, error)
}

type authoritySessionKey struct{}

// WithMCPAuthoritySession supplies an immutable row before its first persistence.
func WithMCPAuthoritySession(ctx context.Context, info *Info) context.Context {
	return context.WithValue(ctx, authoritySessionKey{}, info)
}

func MCPAuthoritySession(ctx context.Context, id string) *Info {
	info, ok := ctx.Value(authoritySessionKey{}).(*Info)
	if ok && info != nil && info.ID == id {
		return info
	}
	return nil
}

func (m *Manager) SetMCPAuthorityResolver(resolver MCPAuthorityResolver) { m.mcpAuthority = resolver }

func (m *Manager) EffectiveMCPAuthority(ctx context.Context, id string) (store.SessionPermissionPolicy, error) {
	if m.mcpAuthority != nil {
		return m.mcpAuthority.EffectiveMCPAuthority(ctx, id)
	}
	return store.SessionPermissionPolicy{}, errors.New("session: MCP authority resolver is unavailable")
}

func (m *Manager) effectiveStartAuthority(
	ctx context.Context, spec *sessionStartSpec,
) (store.SessionPermissionPolicy, error) {
	return m.EffectiveMCPAuthority(WithMCPAuthoritySession(ctx, &Info{
		ID: spec.sessionID, ProfileID: spec.profileID, WorkspaceID: spec.workspace.ID,
		AgentName: spec.agentName, Type: spec.sessionType, Lineage: spec.lineage,
	}), spec.sessionID)
}

func (m *Manager) populateEffectiveMCPAuthority(ctx context.Context, info *Info) error {
	return PopulateEffectiveMCPAuthority(ctx, m.mcpAuthority, info)
}

// PopulateEffectiveMCPAuthority decorates a read snapshot without changing persisted intent.
func PopulateEffectiveMCPAuthority(ctx context.Context, resolver MCPAuthorityResolver, info *Info) error {
	if info == nil || resolver == nil {
		return nil
	}
	policy, err := resolver.EffectiveMCPAuthority(WithMCPAuthoritySession(ctx, info), info.ID)
	if err != nil {
		return err
	}
	info.Lineage = store.CloneSessionLineage(store.NormalizeSessionLineage(info.ID, info.Lineage))
	info.Lineage.EffectiveMCPServers = append([]string{}, policy.MCPServers...)
	return nil
}

func (m *Manager) mcpAuthorityReadContext(ctx context.Context) context.Context {
	if resolver, ok := m.mcpAuthority.(interface {
		MCPAuthorityReadContext(context.Context) context.Context
	}); ok {
		return resolver.MCPAuthorityReadContext(ctx)
	}
	return ctx
}

func (m *Manager) decorateMCPAuthority(ctx context.Context, info *Info) {
	if err := m.populateEffectiveMCPAuthority(ctx, info); err != nil {
		info.Lineage = store.CloneSessionLineage(store.NormalizeSessionLineage(info.ID, info.Lineage))
		info.Lineage.EffectiveMCPServers = []string{}
		if errors.Is(err, context.Canceled) {
			return
		}
		m.logger.WarnContext(ctx, "session.mcp_authority_unavailable", "session_id", info.ID,
			"profile_id", info.ProfileID, "workspace_id", info.WorkspaceID, "error", err)
	}
}

// decorateMCPAuthorities resolves list authority in read mode: one policy resolution per
// profile and workspace, no per-session MCP discovery. Archived sessions cannot run, so
// a listing leaves their servers empty; Status and the session detail resolve them.
func (m *Manager) decorateMCPAuthorities(ctx context.Context, infos []*Info, rows sessionInventory) {
	ctx = m.mcpAuthorityReadContext(ctx)
	for _, info := range infos {
		if info == nil {
			continue
		}
		if rows.archived(info) {
			info.Lineage = store.CloneSessionLineage(store.NormalizeSessionLineage(info.ID, info.Lineage))
			info.Lineage.EffectiveMCPServers = []string{}
			continue
		}
		m.decorateMCPAuthority(ctx, info)
	}
}

// archived reports archive state from the session or its catalog row; listings carry the
// archive timestamp only on the row.
func (rows sessionInventory) archived(info *Info) bool {
	if info.ArchivedAt != nil {
		return true
	}
	row := rows[info.ID]
	return row != nil && row.ArchivedAt != nil
}
