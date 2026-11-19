package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type sessionMCPSummarySnapshot struct {
	fence      sessionMCPReadFence
	inventory  runtimeMCPObservation
	scopeKey   string
	identity   any
	summary    session.MCPSummary
	hasServers bool
	lastRead   time.Time
}

type sessionMCPScopeSnapshot struct {
	fence       sessionMCPReadFence
	batch       uint64
	fingerprint string
	external    string
	policy      string
	collection  settings.CollectionEnvelope
	expiresAt   time.Time
}

type sessionMCPSummaryBatchKey struct{}

type sessionMCPSummaryBatch struct {
	id    uint64
	views map[string]sessionMCPToolViews
}

type sessionMCPReadFence struct {
	epoch, inventory, auth, workspace, heartbeatAuthority uint64
	catalog                                               int64
	redaction                                             [sha256.Size]byte
}

func (r *sessionMCPReadModel) SessionMCPSummary(ctx context.Context, id string) (*session.MCPSummary, error) {
	fence := r.state.sessionMCPReadFence()
	ctx = tools.WithCachedMCPProjection(ctx)
	// A sibling status read may have already memoized policy from before this fence.
	if read, ok := ctx.Value(toolPolicyReadKey{}).(*toolPolicyRead); ok && read.fence != fence {
		ctx = r.registry.FreshReadContext(ctx)
	} else {
		ctx = r.registry.ReadContext(tools.EnsureProjectionListings(ctx))
	}
	scope, local, err := r.summaryIdentity(ctx, id)
	if err != nil {
		if errors.Is(err, session.ErrSessionNotFound) {
			r.mu.Lock()
			delete(r.snapshots, id)
			r.mu.Unlock()
		}
		return nil, err
	}
	inputs, err := r.summaryScopeInputs(ctx, scope, fence)
	if err != nil {
		return nil, fmt.Errorf("daemon: read session %q scope for MCP summary: %w", id, err)
	}
	fence.inventory = 0 // Runtime dependencies are tracked per accepted inventory request.
	r.mu.Lock()
	cached, found := r.snapshots[id]
	if found {
		cached.lastRead = time.Now()
		r.snapshots[id] = cached
	}
	r.mu.Unlock()
	_, boundOrdinary := local.(session.MCPSummaryIdentity)
	fenceChanged := cached.fence != fence && (!boundOrdinary || inputs.policy == "")
	if !found || local == nil || fenceChanged || cached.identity != local || cached.scopeKey != inputs.fingerprint ||
		!r.state.runtimeMCPInventory.matches(cached.inventory) {
		cached, err = r.buildSessionMCPSummary(ctx, scope, inputs, fence, local)
		if err != nil {
			return nil, err
		}
	}
	if !cached.hasServers {
		return nil, nil
	}
	return &cached.summary, nil
}

func (r *sessionMCPReadModel) buildSessionMCPSummary(
	ctx context.Context, scope tools.Scope, inputs sessionMCPScopeSnapshot, fence sessionMCPReadFence, local any,
) (sessionMCPSummarySnapshot, error) {
	info, err := storedSessionStatus(ctx, r.state.sessions, scope.SessionID)
	if err != nil {
		return sessionMCPSummarySnapshot{}, err
	}
	collection := inputs.collection
	if info.ProfileID != scope.ProfileID || info.WorkspaceID != scope.WorkspaceID {
		// A session moved after the initial observation; use the current collection.
		collection, err = r.sessionMCPCollection(ctx, info)
		if err != nil {
			return sessionMCPSummarySnapshot{}, err
		}
	}
	var inventory runtimeMCPObservation
	result, err := r.listSessionMCP(
		context.WithValue(ctx, runtimeMCPObservationKey{}, &inventory),
		info,
		collection,
	)
	if err != nil {
		return sessionMCPSummarySnapshot{}, err
	}
	cached := sessionMCPSummarySnapshot{
		fence: fence, inventory: inventory, identity: local, scopeKey: inputs.fingerprint, summary: result.MCPSummary,
		lastRead:   time.Now(),
		hasServers: len(result.CompozyServers)+len(result.RuntimeServers.Servers) > 0,
	}
	// Re-read live fences, not the request-local policy/listing snapshots.
	currentScope, after, err := r.summaryIdentity(ctx, info.ID)
	if err != nil {
		return sessionMCPSummarySnapshot{}, err
	}
	afterFence := r.state.sessionMCPReadFence()
	afterFence.inventory = 0
	if local != nil && local == after && scope == currentScope && fence == afterFence &&
		r.state.runtimeMCPInventory.matches(inventory) {
		r.mu.Lock()
		if r.snapshots == nil {
			r.snapshots = make(map[string]sessionMCPSummarySnapshot)
		}
		r.snapshots[info.ID] = cached
		r.mu.Unlock()
	}
	return cached, nil
}

func (r *sessionMCPReadModel) summaryScopeInputs(
	ctx context.Context, scope tools.Scope, fence sessionMCPReadFence,
) (sessionMCPScopeSnapshot, error) {
	key := r.registry.ExternalProjectionScopeKey(scope)
	if _, native := r.authority.(*nativeToolPolicyResolver); native {
		key += "\x00" + scope.AgentName
	}
	var batch uint64
	if active, ok := ctx.Value(sessionMCPSummaryBatchKey{}).(*sessionMCPSummaryBatch); ok {
		batch = active.id
	}
	fence.inventory = 0 // Runtime-owned files are never inputs to the Compozy server collection.
	now := time.Now()
	r.mu.Lock()
	cached, found := r.scopes[key]
	r.mu.Unlock()
	if found && cached.fence == fence &&
		(batch == 0 && now.Before(cached.expiresAt) || batch != 0 && batch == cached.batch) {
		return cached, nil
	}
	external, err := r.registry.ExternalProjectionFingerprint(ctx, scope)
	if err != nil {
		return sessionMCPScopeSnapshot{}, err
	}
	if !found || cached.external != external || cached.fence != fence || !r.summaryCollectionVersioned(cached, now) {
		collection, err := r.sessionMCPCollection(
			ctx,
			&session.Info{ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID},
		)
		if err != nil {
			return sessionMCPScopeSnapshot{}, err
		}
		data, err := json.Marshal(collection.MCPServers, json.Deterministic(true))
		if err != nil {
			return sessionMCPScopeSnapshot{}, err
		}
		digest := sha256.Sum256(data)
		policy, err := r.summaryScopedPolicy(ctx, scope)
		if err != nil {
			return sessionMCPScopeSnapshot{}, err
		}
		cached = sessionMCPScopeSnapshot{fence: fence, external: external, policy: policy,
			fingerprint: hex.EncodeToString(digest[:]) + policy + external, collection: collection}
	}
	cached.batch, cached.expiresAt = batch, now.Add(r.summaryIdleTTL())
	after := r.state.sessionMCPReadFence()
	after.inventory = 0
	if fence == after {
		r.mu.Lock()
		if r.scopes == nil {
			r.scopes = make(map[string]sessionMCPScopeSnapshot)
		}
		r.scopes[key] = cached
		r.mu.Unlock()
	}
	return cached, nil
}

// summaryScopedPolicy rebuilds structural policy only after a version or provider input changes.
func (r *sessionMCPReadModel) summaryScopedPolicy(ctx context.Context, scope tools.Scope) (string, error) {
	resolver, ok := r.authority.(*nativeToolPolicyResolver)
	if !ok {
		return "", nil
	}
	scope.SessionID = ""
	inputs, err := resolver.Resolve(ctx, scope)
	if err != nil {
		// Some delegated sessions retain an existing ceiling when their agent is removed.
		// Leave the original session-aware policy path responsible for those reads.
		return "", ctx.Err()
	}
	policy, err := tools.PolicyProjectionFingerprint(inputs)
	if err != nil {
		return "", err
	}
	workspace, cfg, err := resolver.resolveWorkspaceConfig(ctx, scope)
	if err != nil {
		return "", err
	}
	modes := make(map[string]string, len(cfg.Providers))
	for name, provider := range cfg.Providers {
		modes[name] = provider.EffectiveRuntimeMCP()
	}
	data, err := json.Marshal(struct {
		Policy    [sha256.Size]byte
		Modes     map[string]string
		Workspace *workspacepkg.Workspace
	}{policy, modes, summaryWorkspace(workspace)}, json.Deterministic(true))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func summaryWorkspace(resolved *workspacepkg.ResolvedAgentConfig) *workspacepkg.Workspace {
	if resolved == nil {
		return nil
	}
	return &resolved.Workspace
}

func (r *sessionMCPReadModel) summaryCollectionVersioned(cached sessionMCPScopeSnapshot, now time.Time) bool {
	if r.state.mcpAuthGeneration == nil || r.state.workspaceResolver == nil {
		return false
	}
	for i := range cached.collection.MCPServers {
		status := cached.collection.MCPServers[i].AuthStatus
		if status != nil && status.Status == "authenticated" && status.ExpiresAt != nil &&
			!now.Before(*status.ExpiresAt) {
			return false
		}
	}
	return true
}

func (s *bootState) sessionMCPReadFence() sessionMCPReadFence {
	fence := sessionMCPReadFence{
		auth:      s.mcpAuthGeneration.GlobalRevision(),
		redaction: redact.SecretFingerprint(),
		catalog:   s.agentCatalog.Revision(),
	}
	if s.toolProjectionEpoch != nil {
		fence.epoch = s.toolProjectionEpoch.Current()
	}
	if s.runtimeMCPInventory != nil {
		fence.inventory = s.runtimeMCPInventory.Revision()
	}
	if s.workspaceResolver != nil {
		fence.workspace = s.workspaceResolver.AgentConfigRevision()
	}
	if source, ok := s.registry.(interface{ HeartbeatAuthorityRevision() uint64 }); ok {
		fence.heartbeatAuthority = source.HeartbeatAuthorityRevision()
	}
	return fence
}

func (r *sessionMCPReadModel) summaryIdleTTL() time.Duration {
	return r.state.cfg.Skills.EffectivePollInterval()
}

func (r *sessionMCPReadModel) summaryIdentity(ctx context.Context, id string) (tools.Scope, any, error) {
	if manager, ok := r.state.sessions.(*session.Manager); ok {
		if scope, identity, known := manager.BoundMCPSummaryIdentity(id); known {
			return scope, identity, nil
		}
	}
	info, err := storedSessionStatus(ctx, r.state.sessions, id)
	if err != nil {
		return tools.Scope{}, nil, err
	}
	scope := tools.Scope{
		ProfileID:   info.ProfileID,
		WorkspaceID: info.WorkspaceID,
		SessionID:   info.ID,
		AgentName:   info.AgentName,
	}
	local, err := r.summaryLocalInputs(info)
	if local == "" {
		return scope, nil, err
	}
	return scope, local, err
}

func (r *sessionMCPReadModel) summaryLocalInputs(info *session.Info) (string, error) {
	if info.Type == session.SessionTypeTriage || info.InstanceID != "" {
		// Check-in/instance authority includes mutable repository inputs without a complete generation.
		return "", nil
	}
	identity, known := r.state.sessions.(interface{ MCPReadIdentity(string) (string, error) })
	if !known {
		// Unversioned implementations retain synchronous projection reads.
		return "", nil
	}
	runtime, err := identity.MCPReadIdentity(info.ID)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(struct {
		Profile, Workspace, Agent, Instance, Permissions, Runtime string
		Type                                                      session.Type
		Lineage                                                   any
	}{info.ProfileID, info.WorkspaceID, info.AgentName, info.InstanceID, info.EffectivePermissions, runtime,
		info.Type, info.Lineage})
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
	// Revalidate health once per scope, sharing discovery within this batch until a fence moves.
	r.batch++
	var readCtx context.Context
	var fence sessionMCPReadFence
	active := map[string]bool{}
	var errs []error
	for _, id := range manager.BoundMCPSessionIDs() {
		active[id] = true
		if current := r.state.sessionMCPReadFence(); readCtx == nil || fence != current {
			fence = current
			readCtx = r.registry.FreshReadContext(tools.WithCachedMCPProjection(
				context.WithValue(ctx, sessionMCPSummaryBatchKey{}, &sessionMCPSummaryBatch{id: r.batch}),
			))
		}
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
	for key, snapshot := range r.scopes {
		if snapshot.batch != r.batch && !now.Before(snapshot.expiresAt) {
			delete(r.scopes, key)
		}
	}
	for id, snapshot := range r.snapshots {
		if !active[id] && now.Sub(snapshot.lastRead) >= r.summaryIdleTTL() {
			delete(r.snapshots, id)
		}
	}
	r.mu.Unlock()
	return errors.Join(errs...)
}
