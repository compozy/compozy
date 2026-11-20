package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync/atomic"

	"github.com/compozy/compozy/internal/agentinstance"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/managedsidecar"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type mcpProjectionSessionReader interface {
	MCPPolicyIdentity(context.Context, string) (session.MCPPolicyIdentity, bool)
}

type mcpProjectionInstanceReader interface {
	MCPProjectionObservationContext(context.Context) (context.Context, context.CancelFunc)
	MCPProjectionState(context.Context, string, string) (agentinstance.ProjectionState, error)
	ApprovedAuthority(context.Context, string, string) (string, error)
	MCPProjectionCaller(context.Context, string) (agentinstance.Caller, agentinstance.ProjectionState, error)
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

func (p *sessionProjectionPolicyIdentity) isCheckIn() bool {
	return p.session.Kind == session.SessionTypeTriage ||
		p.session.Kind == "" && p.instance.SessionType == string(session.SessionTypeTriage)
}

type projectionDefinitionIdentity struct {
	heartbeat                 [sha256.Size]byte
	workspace                 *workspacepkg.ResolvedAgentConfig
	path, approved, published string
}

type projectionDefinitionKey struct {
	profile, workspace, agent string
	checkIn                   bool
}

type projectionDefinition struct {
	workspace, epoch   uint64
	catalog, heartbeat int64
	path, profileName  string
	resolved           *workspacepkg.ResolvedAgentConfig
}

func (p *projectionDefinition) matches(fence sessionMCPReadFence, heartbeat int64) bool {
	return p.workspace == fence.workspace && p.epoch == fence.epoch &&
		p.catalog == fence.catalog && p.heartbeat == heartbeat
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
	grantScope tools.Scope
	grants     [][4]string
	mismatch   atomic.Bool
	durable    bool
	runtime    *session.MCPSummaryIdentity
}

func (r *nativeToolPolicyResolver) BeginProjectionBuild(
	ctx context.Context, scope tools.Scope, observed any,
) (context.Context, func() bool) {
	identity, ok := observed.(hostedPolicyIdentity)
	if !ok {
		return ctx, func() bool { return false }
	}
	return r.beginProjectionPolicyBuild(ctx, scope, &identity.policy, nil)
}

func (r *nativeToolPolicyResolver) beginProjectionPolicyBuild(
	ctx context.Context,
	scope tools.Scope,
	observed *sessionProjectionPolicyIdentity,
	runtime *session.MCPSummaryIdentity,
) (context.Context, func() bool) {
	_, durable := r.instanceCallers.(mcpProjectionInstanceReader)
	build := &projectionPolicyBuild{
		id:      scope.SessionID,
		mainID:  observed.instance.MainSessionID,
		policy:  *observed,
		durable: durable,
		runtime: runtime,
	}
	if build.mainID != "" && observed.isCheckIn() {
		build.grantScope = observed.checkInGrantScope(scope)
		if observed.grants != "" && json.Unmarshal([]byte(observed.grants), &build.grants) != nil {
			build.mismatch.Store(true)
		}
	}
	var heartbeatRevision int64
	if r.heartbeatCatalogRevision != nil {
		heartbeatRevision = r.heartbeatCatalogRevision()
	}
	return context.WithValue(ctx, projectionPolicyBuildKey{}, build), func() bool {
		return !build.mismatch.Load() &&
			(r.heartbeatCatalogRevision == nil || heartbeatRevision == r.heartbeatCatalogRevision())
	}
}

func (b *projectionPolicyBuild) checkSession(id string, info *session.Info) {
	identity, state := b.policy.session, b.policy.instance
	if id != b.id {
		if id != b.mainID {
			b.mismatch.Store(true)
			return
		}
		identity, state = b.policy.main, b.policy.mainInstance
	}
	var direct *string
	if b.durable {
		direct = &state.DirectInstanceID
	}
	if !identity.MatchesPolicyInfo(info, direct) {
		b.mismatch.Store(true)
	}
}

func (r *nativeToolPolicyResolver) projectionInstanceCaller(
	ctx context.Context, id string,
) (agentinstance.Caller, error) {
	build, capturing := ctx.Value(projectionPolicyBuildKey{}).(*projectionPolicyBuild)
	if !capturing {
		return r.instanceCallers.InstanceCaller(ctx, id)
	}
	repo, ok := r.instanceCallers.(mcpProjectionInstanceReader)
	if !ok {
		build.mismatch.Store(true)
		return r.instanceCallers.InstanceCaller(ctx, id)
	}
	caller, read, err := repo.MCPProjectionCaller(ctx, id)
	want := build.policy.instance
	switch id {
	case build.id:
	case build.mainID:
		want = build.policy.mainInstance
	default:
		build.mismatch.Store(true)
	}
	if read.CallerInstanceID != want.CallerInstanceID || read.CallerInstanceID != "" &&
		(read.Depth != want.Depth || read.Policy != want.Policy ||
			read.LineageKind != want.LineageKind || read.SessionType != want.SessionType || read.Grant != want.Grant ||
			read.ProfileID != want.ProfileID || read.AgentName != want.AgentName) {
		build.mismatch.Store(true)
	}
	return caller, err
}

func (b *projectionPolicyBuild) checkRuntimeSnapshot(snapshot *session.MCPRuntimeSnapshot) {
	if b.runtime == nil {
		return
	}
	var cfg *compozyconfig.Config
	if workspace := b.policy.definition.workspace; workspace != nil {
		cfg = &workspace.Config
	}
	if !snapshot.MatchesSummaryIdentity(b.runtime, cfg) {
		b.mismatch.Store(true)
	}
}

func (b *projectionPolicyBuild) checkWorkspaceConfig(scope tools.Scope, value *workspacepkg.ResolvedAgentConfig) {
	owner := projectionAuthorityScope(b.policy.session, b.policy.instance)
	if scope.ProfileID == owner.ProfileID && scope.WorkspaceID == owner.WorkspaceID {
		if !sameProjectionWorkspace(b.policy.definition.workspace, value) {
			b.mismatch.Store(true)
		}
		return
	}
	owner = projectionAuthorityScope(b.policy.main, b.policy.mainInstance)
	if b.mainID == "" || scope.ProfileID != owner.ProfileID || scope.WorkspaceID != owner.WorkspaceID ||
		!sameProjectionWorkspace(b.policy.mainDefinition.workspace, value) {
		b.mismatch.Store(true)
	}
}

func sameProjectionWorkspace(a, b *workspacepkg.ResolvedAgentConfig) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.ProfileName == b.ProfileName && a.RootDir == b.RootDir &&
		reflect.DeepEqual(&a.Config, &b.Config) && reflect.DeepEqual(a.Agents, b.Agents)
}

func (b *projectionPolicyBuild) checkAuthorityMaterial(id string, heartbeat [sha256.Size]byte, published string) {
	definition := b.policy.definition
	if id != b.id {
		if id != b.mainID {
			b.mismatch.Store(true)
			return
		}
		definition = b.policy.mainDefinition
	}
	if heartbeat != definition.heartbeat || published != definition.published {
		b.mismatch.Store(true)
	}
}

func (b *projectionPolicyBuild) checkApprovedAuthority(info *session.Info, dir, approved string) {
	definition := b.policy.definition
	owner := projectionAuthorityScope(b.policy.session, b.policy.instance)
	switch info.ID {
	case b.id:
	case b.mainID:
		definition = b.policy.mainDefinition
		owner = projectionAuthorityScope(b.policy.main, b.policy.mainInstance)
	default:
		b.mismatch.Store(true)
	}
	if dir != definition.path || approved != definition.approved || info.ProfileID != owner.ProfileID {
		b.mismatch.Store(true)
	}
}

func (b *projectionPolicyBuild) checkApprovalDecision(key tools.ApprovalGrantKey, allowed bool) {
	if key.ProfileID != b.grantScope.ProfileID || key.WorkspaceID != b.grantScope.WorkspaceID ||
		key.AgentName != b.grantScope.AgentName || key.AccessScope != "" {
		b.mismatch.Store(true)
		return
	}
	// Rows are ascending by agent, tool, and input digest. The lookup prefers exact matches.
	expected := false
	for _, row := range slices.Backward(b.grants) {
		if row[1] == key.ToolID.String() && (row[0] == "" || row[0] == key.AgentName) &&
			(row[2] == "" || row[2] == key.InputDigest) {
			expected = row[3] == string(tools.ApprovalGrantAllow)
			break
		}
	}
	if allowed != expected {
		b.mismatch.Store(true)
	}
}

func (r *nativeToolPolicyResolver) canObserveProjectionApprovals() bool {
	return r.approvalGrants == nil || r.approvalProjection != nil
}

func (r *nativeToolPolicyResolver) sessionProjectionPolicy(
	ctx context.Context, scope tools.Scope, fence sessionMCPReadFence,
) (sessionProjectionPolicyIdentity, bool) {
	if ctx.Err() != nil || !r.canObserveProjectionApprovals() {
		return sessionProjectionPolicyIdentity{}, false
	}
	repo, hasRepo := r.instanceCallers.(mcpProjectionInstanceReader)
	if hasRepo {
		var cancel context.CancelFunc
		ctx, cancel = repo.MCPProjectionObservationContext(ctx)
		defer cancel()
	}
	reader, ok := r.sessions.(mcpProjectionSessionReader)
	if !ok {
		return sessionProjectionPolicyIdentity{}, false
	}
	identity, known := reader.MCPPolicyIdentity(ctx, scope.SessionID)
	if !known {
		return sessionProjectionPolicyIdentity{}, false
	}
	result := sessionProjectionPolicyIdentity{session: identity}
	if !hasRepo {
		return result, r.instanceCallers == nil && identity.Kind != session.SessionTypeTriage
	}
	if !r.observeProjectionInstances(ctx, reader, repo, scope.SessionID, &result) {
		return sessionProjectionPolicyIdentity{}, false
	}
	// Config resolution can read from the same pool, so it runs between connection leases.
	owner := projectionAuthorityScope(identity, result.instance)
	result.definition, known = r.observeProjectionDefinition(ctx, owner, fence, result.isCheckIn())
	if !known {
		return sessionProjectionPolicyIdentity{}, false
	}
	if result.isCheckIn() && result.instance.MainSessionID != "" {
		mainOwner := projectionAuthorityScope(result.main, result.mainInstance)
		result.mainDefinition, known = r.observeProjectionDefinition(ctx, mainOwner, fence, false)
		if !known {
			return sessionProjectionPolicyIdentity{}, false
		}
	}
	known = r.observeProjectionApprovals(ctx, repo, scope, owner, &result)
	return result, known
}

func (r *nativeToolPolicyResolver) observeProjectionInstances(
	ctx context.Context, reader mcpProjectionSessionReader, repo mcpProjectionInstanceReader, id string,
	result *sessionProjectionPolicyIdentity,
) bool {
	ctx, release, err := repo.MCPProjectionObservation(ctx)
	if err != nil {
		return false
	}
	defer release()
	result.instance, err = repo.MCPProjectionState(ctx, id, string(result.session.Kind))
	if err != nil {
		return false
	}
	if !result.isCheckIn() || result.instance.MainSessionID == "" {
		return true
	}
	mainID := result.instance.MainSessionID
	var known bool
	result.main, known = reader.MCPPolicyIdentity(ctx, mainID)
	if !known {
		return false
	}
	result.mainInstance, err = repo.MCPProjectionState(ctx, mainID, string(result.main.Kind))
	return err == nil
}

func (p *sessionProjectionPolicyIdentity) checkInGrantScope(scope tools.Scope) tools.Scope {
	// The lookup retains the original agent selector and uses the check-in's stored owner.
	owner := projectionAuthorityScope(p.session, p.instance)
	scope = normalizeToolPolicyScope(scope)
	scope.ProfileID, scope.WorkspaceID = owner.ProfileID, owner.WorkspaceID
	if scope.AgentName == "" {
		scope.AgentName = owner.AgentName
	}
	return scope
}

func projectionAuthorityScope(identity session.MCPPolicyIdentity, state agentinstance.ProjectionState) tools.Scope {
	if identity.Kind != "" {
		return normalizeToolPolicyScope(identity.Scope())
	}
	return normalizeToolPolicyScope(tools.Scope{
		ProfileID: state.ProfileID, WorkspaceID: state.WorkspaceID, AgentName: state.AgentName,
	})
}

func (r *nativeToolPolicyResolver) observeProjectionApprovals(
	ctx context.Context, repo mcpProjectionInstanceReader, scope, owner tools.Scope,
	result *sessionProjectionPolicyIdentity,
) bool {
	checkGrants := result.isCheckIn() && result.instance.MainSessionID != "" && r.approvalGrants != nil
	if result.definition.path == "" && result.mainDefinition.path == "" && !checkGrants {
		return ctx.Err() == nil
	}
	ctx, release, err := repo.MCPProjectionObservation(ctx)
	if err != nil {
		return false
	}
	defer release()
	if result.definition.path != "" {
		result.definition.approved, err = repo.ApprovedAuthority(ctx, owner.ProfileID, result.definition.path)
		if err != nil {
			return false
		}
	}
	if result.mainDefinition.path != "" {
		mainOwner := projectionAuthorityScope(result.main, result.mainInstance)
		result.mainDefinition.approved, err = repo.ApprovedAuthority(
			ctx,
			mainOwner.ProfileID,
			result.mainDefinition.path,
		)
		if err != nil {
			return false
		}
	}
	if checkGrants {
		if r.approvalProjection == nil {
			return false
		}
		result.grants, err = r.approvalProjection.CheckInApprovalProjection(ctx, result.checkInGrantScope(scope))
	}
	return err == nil
}

func (r *nativeToolPolicyResolver) observeProjectionDefinition(
	ctx context.Context, scope tools.Scope, fence sessionMCPReadFence, checkIn bool,
) (projectionDefinitionIdentity, bool) {
	definition, known := r.projectionDefinition(ctx, scope, fence, checkIn)
	id := projectionDefinitionIdentity{workspace: definition.resolved}
	if catalog, ok := r.agentResolver.(*resourceAgentCatalog); ok && definition.path != "" {
		lens := agentCatalogLensForIdentity(scope.WorkspaceID, scope.ProfileID, definition.profileName)
		id.published = catalog.projectionHeartbeat(scope.AgentName, lens)
	}
	if !known || definition.path == "" {
		return id, known
	}
	body, err := os.ReadFile(definition.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return projectionDefinitionIdentity{}, false
	}
	id.heartbeat = authorityBodyDigest(body, err)
	dir, err := filepath.EvalSymlinks(filepath.Dir(definition.path))
	if errors.Is(err, os.ErrNotExist) {
		return id, true
	}
	if err != nil {
		return projectionDefinitionIdentity{}, false
	}
	id.path = dir
	return id, true
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
	if found && definition.matches(fence, heartbeatRevision) {
		return definition, true
	}
	definition = projectionDefinition{
		workspace: fence.workspace, epoch: fence.epoch, catalog: fence.catalog, heartbeat: heartbeatRevision,
	}
	ctx = r.PolicyReadContext(ctx, true)
	workspace, _, err := r.resolveWorkspaceConfig(ctx, scope)
	if err != nil {
		return projectionDefinition{}, false
	}
	agent, err := r.resolveAgent(ctx, scope.AgentName, workspace)
	if ctx.Err() != nil || err != nil && !errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
		return projectionDefinition{}, false
	}
	definition.resolved = workspace
	if err == nil && (checkIn || len(heartbeat.AuthorityOverrides(agent.MCPServers)) != 0) {
		if path, err := managedsidecar.SidecarPath(agent.SourcePath, heartbeat.FileName); err == nil {
			definition.path = path
		}
		if workspace != nil {
			definition.profileName = workspace.ProfileName
		}
	}
	r.authorityMu.Lock()
	defer r.authorityMu.Unlock()
	// Concurrent observations for the same fence retain one comparable snapshot identity.
	if previous, ok := r.projectionDefinitions[key]; ok && previous.matches(fence, heartbeatRevision) {
		return previous, true
	}
	if r.projectionDefinitions == nil || len(r.projectionDefinitions) >= instanceSourceDeclarationCacheLimit {
		r.projectionDefinitions = make(map[projectionDefinitionKey]projectionDefinition)
	}
	r.projectionDefinitions[key] = definition
	return definition, true
}
