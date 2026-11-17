package session

import (
	"context"
	"encoding/json/v2"
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
