package session

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"slices"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/runtimemcp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/tools"
)

const EventTypeMCPChanged = "session.mcp_changed"

type MCPSummary struct {
	Usable        int  `json:"usable"`
	Attention     int  `json:"attention"`
	PendingReload bool `json:"pending_reload"`
}

type MCPServerRow struct {
	Name          string `json:"name"`
	Owner         string `json:"owner"`
	State         string `json:"state"`
	ToolCount     int    `json:"tool_count"`
	CallableCount int    `json:"callable_count"`
	Reason        string `json:"reason"`
}

type RuntimeMCPRow struct {
	Name      string `json:"name"`
	Scope     string `json:"scope"`
	Transport string `json:"transport"`
	State     string `json:"state"`
}

type MCP struct {
	SessionID          string `json:"session_id"`
	Runtime            string `json:"runtime"`
	RuntimeDisplayName string `json:"runtime_display_name"`
	MCPSummary
	LastReload     *MCPReloadRecord `json:"last_reload,omitempty"`
	CompozyServers []MCPServerRow   `json:"compozy_servers"`
	RuntimeServers RuntimeMCP       `json:"runtime_servers"`
}

type RuntimeMCP struct {
	RuntimeMCP string          `json:"runtime_mcp"`
	Servers    []RuntimeMCPRow `json:"servers"`
}

type MCPReloadRecord struct {
	At      time.Time `json:"at"`
	Outcome string    `json:"outcome"`
	Reason  string    `json:"reason"`
}

type MCPReader interface {
	ListSessionMCP(context.Context, string) (MCP, error)
}

var _ MCPReader = (*Manager)(nil)

func (m *Manager) SetMCPReader(reader MCPReader) { m.mcpReader = reader }

func (m *Manager) ListSessionMCP(ctx context.Context, id string) (MCP, error) {
	if m.mcpReader == nil {
		return MCP{}, errors.New("session: MCP read model unavailable")
	}
	return m.mcpReader.ListSessionMCP(tools.WithCachedMCPProjection(ctx), id)
}

type MCPRuntimeSnapshot struct {
	Inventory      runtimemcp.Snapshot
	Mode           string
	Pending        bool
	Bound          bool
	BoundServers   map[string]int
	PendingServers map[string]bool
	LastReload     *MCPReloadRecord
}

// MCPReadSnapshot reads accepted launch context without preparing or launching a runtime.
func (m *Manager) MCPReadSnapshot(
	ctx context.Context,
	id string,
	catalog tools.AdvertisedCatalog,
) (MCPRuntimeSnapshot, error) {
	ctx = tools.WithCachedMCPProjection(ctx)
	provider := ""
	state := activationState{}
	bound, reloadRequired := false, true
	if active, ok := m.Get(id); ok {
		snapshot := active.runtimeBindingSnapshot()
		state, bound, reloadRequired = snapshot.activation, snapshot.process != nil, snapshot.hostedMCPRequiresReload
		provider = snapshot.selection.Provider
	}
	mode, request, err := m.readMCPRuntimeContext(ctx, id, provider, state.runtimeRequest)
	if err != nil {
		return MCPRuntimeSnapshot{}, err
	}
	result := MCPRuntimeSnapshot{
		Mode:         mode,
		Bound:        bound,
		BoundServers: state.boundServers,
	}
	if state.lastReload == nil {
		events, err := m.Events(ctx, id, store.EventQuery{Type: EventTypeMCPReloaded, Limit: 1})
		if err != nil {
			return result, err
		}
		if len(events) > 0 {
			var record MCPReloadRecord
			if err := json.Unmarshal([]byte(events[0].Content), &record); err != nil {
				return result, err
			}
			record.At = events[0].Timestamp
			result.LastReload = &record
		}
	}

	if state.lastReload != nil {
		result.LastReload = &MCPReloadRecord{
			At:      state.lastReload.At,
			Outcome: state.lastReload.Outcome,
			Reason:  state.lastReload.Reason,
		}
	}
	result.Inventory, err = m.readRuntimeMCP(ctx, *request)
	if err != nil {
		return result, err
	}
	if result.LastReload != nil && result.LastReload.Outcome == string(BadgeFailed) {
		result.Pending = true
	}
	if bound {
		m.applyMCPPendingState(ctx, id, &result, state, reloadRequired, catalog)
	}
	return result, nil
}

// readMCPRuntimeContext reads persisted identity; accepted launches retain their exact environment.
func (m *Manager) readMCPRuntimeContext(
	ctx context.Context,
	id, provider string,
	accepted *runtimemcp.Request,
) (string, *runtimemcp.Request, error) {
	meta, err := m.readSessionMetaReadOnly(ctx, id)
	if err != nil {
		return "", nil, err
	}
	workspace, err := m.resolveResumeWorkspace(ctx, &meta)
	if err != nil {
		return "", nil, err
	}
	if provider == "" {
		provider = meta.Provider
	}
	providerConfig, err := workspace.Config.ResolveProvider(provider)
	if err != nil {
		return "", nil, err
	}
	mode := providerConfig.EffectiveRuntimeMCP()
	if accepted != nil {
		return mode, accepted, nil
	}
	cwd, err := m.WorkingDir(ctx, id)
	if err != nil {
		return "", nil, err
	}
	_, worktreeRoot, err := m.resolveSessionWorktree(ctx, workspace.ID, meta.WorktreeIDValue())
	if err != nil {
		return "", nil, err
	}
	readContext, err := runtimemcp.ProviderContext(
		m.homePaths,
		provider,
		providerConfig,
		os.Environ(),
		cwd,
		worktreeRoot,
	)
	if err != nil {
		return "", nil, err
	}
	readContext.Profile, readContext.WorkspaceRoot = meta.ProfileID, workspace.RootDir
	runtimeID := providerConfig.RuntimeProviderName(provider)
	if providerConfig.EffectiveHarness() == compozyconfig.ProviderHarnessPiACP {
		runtimeID = "pi"
	}
	if runtimeID == "qwen" {
		runtimeID = runtimeMCPQwenCode
	}
	return mode, &runtimemcp.Request{Context: readContext, Runtime: runtimeID}, nil
}

func (m *Manager) BoundMCPSessionIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := []string{}
	for id, s := range m.sessions {
		if s.processHandle() != nil {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// RecordMCPChanged appends the canonical summary before publishing it to live readers.
func (m *Manager) RecordMCPChanged(ctx context.Context, id string, summary MCPSummary) error {
	s, ok := m.Get(id)
	if !ok || s.processHandle() == nil {
		return nil
	}
	content, err := json.Marshal(struct {
		MCPSummary MCPSummary `json:"mcp_summary"`
	}{summary})
	if err != nil {
		return err
	}
	eventID, err := store.NewID("ev")
	if err != nil {
		return err
	}
	turnID := s.CurrentTurnID()
	if turnID == "" {
		turnID, err = m.newPromptTurnID()
		if err != nil {
			return err
		}
	}
	persisted, _, err := m.appendDurableSessionEvent(
		ctx,
		id,
		store.SessionEvent{
			ID:        eventID,
			SessionID: id,
			AgentName: s.AgentName,
			TurnID:    turnID,
			Type:      EventTypeMCPChanged,
			Content:   string(content),
			Timestamp: m.now(),
		},
	)
	if err != nil {
		return err
	}
	m.publishSessionEventByID(ctx, id, persisted)
	return nil
}

func (m *Manager) applyMCPPendingState(
	ctx context.Context, id string, result *MCPRuntimeSnapshot, state activationState,
	reloadRequired bool, catalog tools.AdvertisedCatalog,
) {
	result.Pending = result.Pending || state.runtimeMCP != result.Mode ||
		m.currentRuntimeMCPDigest(ctx, state, result.Mode) != state.runtimeDigest
	if state.boundDigest != "" && reloadRequired {
		result.Pending = result.Pending ||
			state.boundDigest != activationFingerprint(catalog.Digest, result.Mode, state.runtimeDigest)
	}
	if !reloadRequired {
		if source, ok := m.hostedMCP.(interface{ ServedCatalogDigest(string) string }); ok {
			result.Pending = result.Pending || source.ServedCatalogDigest(id) != catalog.Digest
		}
	}
	if result.Pending {
		result.PendingServers = make(map[string]bool)
		for name, digest := range catalog.ServerDigests {
			if digest != state.boundServerDigests[name] {
				result.PendingServers[name] = true
			}
		}
	}
}
