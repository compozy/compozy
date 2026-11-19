package daemon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/managedsidecar"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type mcpProjectionSessionReader interface {
	MCPPolicyIdentity(string) (session.MCPPolicyIdentity, bool)
}

type mcpProjectionInstanceReader interface {
	MCPProjectionState(context.Context, string, string) (agentinstance.ProjectionState, error)
	ApprovedAuthority(context.Context, string, string) (string, error)
	BeginMCPProjectionRead(context.Context) func() bool
	MCPProjectionObservation(context.Context) (context.Context, func(), error)
}

type mcpProjectionGrantReader interface {
	CheckInApprovalProjection(context.Context, tools.Scope) (string, error)
	CheckInApprovalDecision(context.Context, tools.ApprovalGrantKey) (bool, error)
}

type sessionProjectionPolicyIdentity struct {
	session, main              session.MCPPolicyIdentity
	instance, mainInstance     agentinstance.ProjectionState
	definition, mainDefinition projectionDefinitionIdentity
	grants                     string
}

type projectionDefinitionIdentity struct {
	heartbeat                 filesnap.Snapshot
	path, approved, published string
}

type projectionDefinitionKey struct {
	profile, workspace, agent string
	checkIn                   bool
}

type projectionDefinition struct {
	workspace, epoch   uint64
	catalog, heartbeat int64
	path, published    string
}

type hostedPolicyIdentity struct {
	policy          sessionProjectionPolicyIdentity
	auth, workspace uint64
	catalog         int64
}

// ProjectionPolicyIdentity observes inputs to a hosted policy without compiling it.
func (r *nativeToolPolicyResolver) ProjectionPolicyIdentity(ctx context.Context, scope tools.Scope) (any, bool) {
	if r.readFence == nil {
		return nil, false
	}
	fence := r.readFence()
	policy, known := r.sessionProjectionPolicy(ctx, scope, fence)
	return hostedPolicyIdentity{policy, fence.auth, fence.workspace, fence.catalog}, known
}

type projectionPolicyBuildKey struct{}

type projectionPolicyBuild struct {
	id, mainID string
	policy     sessionProjectionPolicyIdentity
	mismatch   atomic.Bool
}

func (r *nativeToolPolicyResolver) BeginProjectionBuild(
	ctx context.Context, scope tools.Scope, observed any,
) (context.Context, func() bool) {
	identity, ok := observed.(hostedPolicyIdentity)
	if !ok {
		return ctx, func() bool { return false }
	}
	return r.beginProjectionPolicyBuild(ctx, scope, &identity.policy)
}

func (r *nativeToolPolicyResolver) beginProjectionPolicyBuild(
	ctx context.Context, scope tools.Scope, observed *sessionProjectionPolicyIdentity,
) (context.Context, func() bool) {
	build := &projectionPolicyBuild{id: scope.SessionID, mainID: observed.instance.MainSessionID, policy: *observed}
	var finish func() bool
	if repo, ok := r.instanceCallers.(mcpProjectionInstanceReader); ok {
		finish = repo.BeginMCPProjectionRead(ctx)
	}
	var heartbeatRevision int64
	if r.heartbeatCatalogRevision != nil {
		heartbeatRevision = r.heartbeatCatalogRevision()
	}
	return context.WithValue(ctx, projectionPolicyBuildKey{}, build), func() bool {
		if finish == nil && r.instanceCallers != nil || finish != nil && !finish() {
			return false
		}
		return !build.mismatch.Load() &&
			(r.heartbeatCatalogRevision == nil || heartbeatRevision == r.heartbeatCatalogRevision())
	}
}

func (b *projectionPolicyBuild) checkSession(id string, info *session.Info) {
	switch id {
	case b.id:
		if !b.policy.session.MatchesPolicyInfo(info) {
			b.mismatch.Store(true)
		}
	case b.mainID:
		if !b.policy.main.MatchesPolicyInfo(info) {
			b.mismatch.Store(true)
		}
	}
}

func (r *nativeToolPolicyResolver) sessionProjectionPolicy(
	ctx context.Context, scope tools.Scope, fence sessionMCPReadFence,
) (sessionProjectionPolicyIdentity, bool) {
	reader, ok := r.sessions.(mcpProjectionSessionReader)
	if !ok {
		return sessionProjectionPolicyIdentity{}, false
	}
	identity, known := reader.MCPPolicyIdentity(scope.SessionID)
	if !known {
		return sessionProjectionPolicyIdentity{}, false
	}
	result := sessionProjectionPolicyIdentity{session: identity}
	repo, ok := r.instanceCallers.(mcpProjectionInstanceReader)
	if !ok {
		return result, r.instanceCallers == nil && identity.Kind != session.SessionTypeTriage
	}
	ctx, release, err := repo.MCPProjectionObservation(ctx)
	if err != nil {
		return sessionProjectionPolicyIdentity{}, false
	}
	defer release()
	result.instance, err = repo.MCPProjectionState(ctx, scope.SessionID, string(identity.Kind))
	if err != nil {
		return sessionProjectionPolicyIdentity{}, false
	}
	checkIn := identity.Kind == session.SessionTypeTriage ||
		identity.Kind == "" && result.instance.SessionType == string(session.SessionTypeTriage)
	owner := projectionAuthorityScope(identity, result.instance)
	result.definition, known = r.observeProjectionDefinition(ctx, repo, owner, fence, checkIn)
	if !known {
		return sessionProjectionPolicyIdentity{}, false
	}
	if !checkIn || result.instance.MainSessionID == "" {
		return result, true
	}
	// The check-in's grant lookup retains its original agent selector and stored owner.
	scope = normalizeToolPolicyScope(scope)
	scope.ProfileID, scope.WorkspaceID = owner.ProfileID, owner.WorkspaceID
	if scope.AgentName == "" {
		scope.AgentName = owner.AgentName
	}
	known = r.observeCheckInMainPolicy(ctx, reader, repo, scope, fence, &result)
	return result, known
}

func projectionAuthorityScope(identity session.MCPPolicyIdentity, state agentinstance.ProjectionState) tools.Scope {
	if identity.Kind != "" {
		return normalizeToolPolicyScope(identity.Scope())
	}
	return normalizeToolPolicyScope(tools.Scope{
		ProfileID: state.ProfileID, WorkspaceID: state.WorkspaceID, AgentName: state.AgentName,
	})
}

func (r *nativeToolPolicyResolver) observeCheckInMainPolicy(
	ctx context.Context, reader mcpProjectionSessionReader, repo mcpProjectionInstanceReader, scope tools.Scope,
	fence sessionMCPReadFence, result *sessionProjectionPolicyIdentity,
) bool {
	var known bool
	result.main, known = reader.MCPPolicyIdentity(result.instance.MainSessionID)
	if !known {
		return false
	}
	var err error
	result.mainInstance, err = repo.MCPProjectionState(ctx, result.instance.MainSessionID, string(result.main.Kind))
	if err != nil {
		return false
	}
	mainOwner := projectionAuthorityScope(result.main, result.mainInstance)
	result.mainDefinition, known = r.observeProjectionDefinition(ctx, repo, mainOwner, fence, false)
	if !known {
		return false
	}
	if r.approvalGrants != nil {
		if r.approvalProjection == nil {
			return false
		}
		result.grants, err = r.approvalProjection.CheckInApprovalProjection(ctx, scope)
	}
	return err == nil
}

func (r *nativeToolPolicyResolver) observeProjectionDefinition(
	ctx context.Context, repo mcpProjectionInstanceReader, scope tools.Scope, fence sessionMCPReadFence, checkIn bool,
) (projectionDefinitionIdentity, bool) {
	definition, known := r.projectionDefinition(ctx, scope, fence, checkIn)
	id := projectionDefinitionIdentity{published: definition.published}
	if !known || definition.path == "" {
		return id, known
	}
	snapshot, err := filesnap.FromPath(definition.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return projectionDefinitionIdentity{}, false
	}
	if !snapshot.Reusable() {
		return projectionDefinitionIdentity{}, false
	}
	id.heartbeat = snapshot
	dir, err := filepath.EvalSymlinks(filepath.Dir(definition.path))
	if errors.Is(err, os.ErrNotExist) {
		return id, true
	}
	if err != nil {
		return projectionDefinitionIdentity{}, false
	}
	id.path = dir
	id.approved, err = repo.ApprovedAuthority(ctx, scope.ProfileID, dir)
	return id, err == nil
}

func (r *nativeToolPolicyResolver) projectionDefinition(
	ctx context.Context, scope tools.Scope, fence sessionMCPReadFence, checkIn bool,
) (projectionDefinition, bool) {
	key := projectionDefinitionKey{scope.ProfileID, scope.WorkspaceID, scope.AgentName, checkIn}
	var heartbeatRevision int64
	if r.heartbeatCatalogRevision != nil {
		heartbeatRevision = r.heartbeatCatalogRevision()
	}
	r.authorityMu.Lock()
	definition, found := r.projectionDefinitions[key]
	r.authorityMu.Unlock()
	if found && definition.workspace == fence.workspace && definition.catalog == fence.catalog &&
		definition.epoch == fence.epoch &&
		definition.heartbeat == heartbeatRevision {
		return definition, true
	}
	definition = projectionDefinition{
		workspace: fence.workspace, epoch: fence.epoch, catalog: fence.catalog, heartbeat: heartbeatRevision,
	}
	read, found := ctx.Value(toolPolicyReadKey{}).(*toolPolicyRead)
	workspace, _, err := r.resolveWorkspaceConfig(r.PolicyReadContext(ctx, found && read.fence != fence), scope)
	if err != nil {
		return projectionDefinition{}, false
	}
	agent, err := r.resolveAgent(ctx, scope.AgentName, workspace)
	if err != nil && !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return projectionDefinition{}, false
	}
	if err == nil && (checkIn || len(heartbeat.AuthorityOverrides(agent.MCPServers)) != 0) {
		if path, err := managedsidecar.SidecarPath(agent.SourcePath, heartbeat.FileName); err == nil {
			definition.path = path
		}
		if catalog, ok := r.agentResolver.(*resourceAgentCatalog); ok {
			definition.published = catalog.projectionHeartbeat(scope)
		}
	}
	r.authorityMu.Lock()
	if r.projectionDefinitions == nil || len(r.projectionDefinitions) >= instanceSourceDeclarationCacheLimit {
		r.projectionDefinitions = make(map[projectionDefinitionKey]projectionDefinition)
	}
	r.projectionDefinitions[key] = definition
	r.authorityMu.Unlock()
	return definition, true
}
