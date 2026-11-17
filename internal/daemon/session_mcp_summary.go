package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/tools"
)

type sessionMCPSummarySnapshot struct {
	key        string
	summary    session.MCPSummary
	hasServers bool
	lastRead   time.Time
}

type sessionMCPScopeSnapshot struct {
	fence, fingerprint string
	expiresAt          time.Time
}

func (r *sessionMCPReadModel) SessionMCPSummary(ctx context.Context, id string) (*session.MCPSummary, error) {
	ctx = r.registry.ReadContext(tools.EnsureProjectionListings(tools.WithCachedMCPProjection(ctx)))
	local, err := r.summaryLocalInputs(ctx, id)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			r.mu.Lock()
			delete(r.snapshots, id)
			r.mu.Unlock()
		}
		return nil, err
	}
	key, err := r.summaryInputs(ctx, id, local)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	cached, found := r.snapshots[id]
	if found {
		cached.lastRead = time.Now()
		r.snapshots[id] = cached
	}
	r.mu.Unlock()
	if !found || local == "" || cached.key != key {
		result, err := r.ListSessionMCP(ctx, id)
		if err != nil {
			return nil, err
		}
		cached = sessionMCPSummarySnapshot{
			key: key, summary: result.MCPSummary,
			lastRead:   time.Now(),
			hasServers: len(result.CompozyServers)+len(result.RuntimeServers.Servers) > 0,
		}
		// Re-read live fences, not the request-local policy/listing snapshots.
		after, err := r.summaryLocalInputs(ctx, id)
		if err != nil {
			return nil, err
		}
		if local != "" && local == after {
			r.mu.Lock()
			if r.snapshots == nil {
				r.snapshots = make(map[string]sessionMCPSummarySnapshot)
			}
			r.snapshots[id] = cached
			r.mu.Unlock()
		}
	}
	if !cached.hasServers {
		return nil, nil
	}
	return &cached.summary, nil
}

func (r *sessionMCPReadModel) summaryInputs(ctx context.Context, id, local string) (string, error) {
	info, err := storedSessionStatus(ctx, r.state.sessions, id)
	if err != nil {
		return "", err
	}
	fingerprint, err := r.summaryScopeInputs(ctx, info)
	if err != nil {
		return "", err
	}
	return local + fingerprint, nil
}

func (r *sessionMCPReadModel) summaryScopeInputs(ctx context.Context, info *session.Info) (string, error) {
	scope := tools.Scope{
		ProfileID: info.ProfileID, WorkspaceID: info.WorkspaceID, SessionID: info.ID, AgentName: info.AgentName,
	}
	key, fence := r.registry.ExternalProjectionScopeKey(scope), r.summaryScopeFence()
	now := time.Now()
	r.mu.Lock()
	cached, found := r.scopes[key]
	r.mu.Unlock()
	if found && cached.fence == fence && now.Before(cached.expiresAt) {
		return cached.fingerprint, nil
	}
	collection, err := r.sessionMCPCollection(ctx, info)
	if err != nil {
		return "", err
	}
	external, err := r.registry.ExternalProjectionFingerprint(ctx, scope)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		External   string
		Collection any
	}{external, collection.MCPServers})
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	fingerprint := hex.EncodeToString(digest[:])
	if fence == r.summaryScopeFence() {
		interval := r.summaryIdleTTL()
		r.mu.Lock()
		if r.scopes == nil {
			r.scopes = make(map[string]sessionMCPScopeSnapshot)
		}
		for scopeKey, snapshot := range r.scopes {
			if !now.Before(snapshot.expiresAt) {
				delete(r.scopes, scopeKey)
			}
		}
		r.scopes[key] = sessionMCPScopeSnapshot{fence, fingerprint, now.Add(interval)}
		r.mu.Unlock()
	}
	return fingerprint, nil
}

func (r *sessionMCPReadModel) summaryScopeFence() string {
	var epoch, inventory uint64
	if r.state.toolProjectionEpoch != nil {
		epoch = r.state.toolProjectionEpoch.Current()
	}
	if r.state.runtimeMCPInventory != nil {
		inventory = r.state.runtimeMCPInventory.Revision()
	}
	var policy uint64
	if r.state.workspaceResolver != nil {
		policy = r.state.workspaceResolver.AgentConfigRevision()
	}
	return fmt.Sprintf("%d:%d:%d:%d:%d", epoch, inventory, r.state.mcpAuthGeneration.GlobalRevision(),
		policy, r.state.agentCatalog.Revision())
}

func (r *sessionMCPReadModel) summaryIdleTTL() time.Duration {
	if interval := r.state.cfg.Skills.PollInterval; interval > 0 {
		return interval
	}
	return 3 * time.Second
}

func (r *sessionMCPReadModel) summaryLocalInputs(ctx context.Context, id string) (string, error) {
	info, err := storedSessionStatus(ctx, r.state.sessions, id)
	if err != nil {
		return "", err
	}
	if info.Type == session.SessionTypeTriage || info.InstanceID != "" {
		// Check-in/instance authority includes mutable repository inputs without a complete generation.
		return "", nil
	}
	identity, known := r.state.sessions.(interface{ MCPReadIdentity(string) (string, error) })
	if !known {
		// Unversioned implementations retain synchronous projection reads.
		return "", nil
	}
	runtime, err := identity.MCPReadIdentity(id)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		Profile, Workspace, Agent, Instance, Permissions, Runtime string
		Type                                                      session.Type
		Lineage                                                   any
		PolicyFence                                               string
	}{info.ProfileID, info.WorkspaceID, info.AgentName, info.InstanceID, info.EffectivePermissions, runtime,
		info.Type, info.Lineage, r.summaryScopeFence()})
	return string(data), err
}

// refresh publishes changed summaries without holding the cache lock across reads or durable writes.
func (r *sessionMCPReadModel) refresh(ctx context.Context) error {
	manager, ok := r.state.sessions.(interface {
		BoundMCPSessionIDs() []string
		RecordMCPChanged(context.Context, string, session.MCPSummary) error
	})
	if !ok {
		return nil
	}
	r.refreshMu.Lock()
	defer r.refreshMu.Unlock()
	// Revalidate unsignaled external health once per provider scope at each configured poll.
	r.mu.Lock()
	clear(r.scopes)
	r.mu.Unlock()
	active := map[string]bool{}
	var errs []error
	for _, id := range manager.BoundMCPSessionIDs() {
		active[id] = true
		readCtx := r.registry.ReadContext(tools.WithProjectionListings(
			tools.WithCachedMCPProjection(ctx), tools.NewProjectionListings(),
		))
		summary, err := r.SessionMCPSummary(readCtx, id)
		if err != nil {
			errs = append(errs, fmt.Errorf("daemon: read MCP summary for session %q: %w", id, err))
			continue
		}
		current := session.MCPSummary{}
		if summary != nil {
			current = *summary
		}
		r.mu.Lock()
		previous, exists := r.summaries[id]
		r.mu.Unlock()
		if exists && previous == current {
			continue
		}
		if err := manager.RecordMCPChanged(ctx, id, current); err != nil {
			errs = append(errs, fmt.Errorf("daemon: record MCP summary for session %q: %w", id, err))
			continue
		}
		r.mu.Lock()
		r.summaries[id] = current
		r.mu.Unlock()
	}
	r.mu.Lock()
	for id := range r.summaries {
		if !active[id] {
			delete(r.summaries, id)
		}
	}
	now := time.Now()
	for id, snapshot := range r.snapshots {
		if !active[id] && now.Sub(snapshot.lastRead) >= r.summaryIdleTTL() {
			delete(r.snapshots, id)
		}
	}
	r.mu.Unlock()
	return errors.Join(errs...)
}
