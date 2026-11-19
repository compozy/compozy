package session

import (
	"context"
	"encoding/json/v2"

	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/tools"
)

// MCPReadIdentity captures accepted activation and served receipts without loading projections or configuration.
func (m *Manager) MCPReadIdentity(id string) (string, error) {
	s, ok := m.Get(id)
	if !ok {
		return "", nil
	}
	s.mu.RLock()
	data, err := json.Marshal(struct {
		Generation                                                          int64
		Bound                                                               bool
		ReloadRequired                                                      bool
		BoundDigest, RuntimeMode, RuntimeDigest, HostedDigest, DirectDigest string
		LastReload                                                          *reloadRecord
		Selection                                                           *RuntimeSelection
		SelectionRevision                                                   int64
	}{
		s.RuntimeGeneration,
		s.process != nil,
		s.hostedMCPRequiresReload,
		s.activation.boundDigest,
		s.activation.runtimeMCP,
		s.activation.runtimeDigest,
		s.hostedMCPDigest,
		s.directMCPDigest,
		s.activation.lastReload,
		s.SelectedRuntime,
		s.RuntimeSelectionRevision,
	})
	s.mu.RUnlock()
	if err != nil {
		return "", err
	}
	served := ""
	if reader, ok := m.hostedMCP.(interface{ ServedCatalogDigest(string) string }); ok {
		served = reader.ServedCatalogDigest(id)
	}
	return string(data) + served, nil
}

// SessionMCPSummary uses a current summary when the read model supports one.
func (m *Manager) SessionMCPSummary(ctx context.Context, id string) (*MCPSummary, error) {
	if reader, ok := m.mcpReader.(interface {
		SessionMCPSummary(context.Context, string) (*MCPSummary, error)
	}); ok {
		return reader.SessionMCPSummary(ctx, id)
	}
	result, err := m.ListSessionMCP(ctx, id)
	if err != nil || len(result.CompozyServers)+len(result.RuntimeServers.Servers) == 0 {
		return nil, err
	}
	return &result.MCPSummary, nil
}

// MCPPolicyIdentity identifies the active session inputs read by native tool policy.
// Session lineage updates replace the immutable lineage pointer under the session lock.
type MCPPolicyIdentity struct {
	session                   *Session
	lineage                   *store.SessionLineage
	profile, workspace, agent string
	permissions               string
	kind                      Type
}

func (m *Manager) MCPPolicyIdentity(id string) (MCPPolicyIdentity, bool) {
	s, ok := m.Get(id)
	if !ok {
		return MCPPolicyIdentity{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mcpPolicyIdentityLocked()
}

func (s *Session) mcpPolicyIdentityLocked() (MCPPolicyIdentity, bool) {
	if s.InstanceID != "" || s.Type == SessionTypeTriage {
		// Instance and check-in policy consult live repository inputs.
		return MCPPolicyIdentity{}, false
	}
	caps, _ := s.capabilitiesSnapshotLocked()
	return MCPPolicyIdentity{
		session: s, lineage: s.Lineage, profile: s.ProfileID, workspace: s.WorkspaceID,
		agent: s.AgentName, permissions: reportedPermissionMode(s.EffectivePermissions, caps.PermissionModeDegraded),
		kind: s.Type,
	}, true
}

// MCPSummaryIdentity captures only inputs to a bound session's MCP summary.
type MCPSummaryIdentity struct {
	policy                                                           MCPPolicyIdentity
	process                                                          *AgentProcess
	generation, selectionRevision                                    int64
	requiresReload                                                   bool
	bound, mode, runtime, hosted, direct, served, provider, worktree string
	request                                                          *runtimemcp.Request
	reload                                                           *reloadRecord
}

// BoundMCPSummaryIdentity avoids cloning the session status and lineage on idle polls.
func (m *Manager) BoundMCPSummaryIdentity(id string) (tools.Scope, MCPSummaryIdentity, bool) {
	s, exists := m.Get(id)
	if !exists {
		return tools.Scope{}, MCPSummaryIdentity{}, false
	}
	s.mu.RLock()
	policy, known := s.mcpPolicyIdentityLocked()
	if !known || s.process == nil {
		s.mu.RUnlock()
		return tools.Scope{}, MCPSummaryIdentity{}, false
	}
	identity := MCPSummaryIdentity{
		policy: policy, process: s.process, generation: s.RuntimeGeneration,
		selectionRevision: s.RuntimeSelectionRevision, requiresReload: s.hostedMCPRequiresReload,
		bound: s.activation.boundDigest, mode: s.activation.runtimeMCP, runtime: s.activation.runtimeDigest,
		hosted: s.hostedMCPDigest, direct: s.directMCPDigest, provider: s.Provider, worktree: s.WorktreeID,
		request: s.activation.runtimeRequest, reload: s.activation.lastReload,
	}
	s.mu.RUnlock()
	if reader, ok := m.hostedMCP.(interface{ ServedCatalogDigest(string) string }); ok {
		identity.served = reader.ServedCatalogDigest(id)
	}
	return tools.Scope{
		SessionID:   id,
		ProfileID:   policy.profile,
		WorkspaceID: policy.workspace,
		AgentName:   policy.agent,
	}, identity, true
}
