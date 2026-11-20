package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/managedsidecar"
	"github.com/compozy/compozy/internal/session"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type authorityPolicyKey struct {
	profile, dir string
	digest       [32]byte
	published    string
}
type authorityPolicyValue struct {
	policy      heartbeat.ResolvedPolicy
	digest, dir string
	err         error
}

func (r *nativeToolPolicyResolver) cachedAuthorityPolicy(
	ctx context.Context,
	agent compozyconfig.AgentDef,
	info *session.Info,
	workspace *workspacepkg.ResolvedAgentConfig,
	cfg *compozyconfig.Config,
) (heartbeat.ResolvedPolicy, string, string, error) {
	target := toolPolicyAuthorityTarget(agent, info, workspace, cfg)
	path, pathErr := managedsidecar.SidecarPath(agent.SourcePath, heartbeat.FileName)
	body, readErr := os.ReadFile(path)
	data, err := json.Marshal(struct {
		Target    heartbeat.AuthoringTarget
		Overrides map[string][]string
		Body      []byte
		ReadError string
	}{target, heartbeat.AuthorityOverrides(agent.MCPServers), body, authorityReadError(pathErr, readErr)},
		json.Deterministic(true),
		json.WithMarshalers(json.MarshalFunc(func(value time.Duration) ([]byte, error) {
			return json.Marshal(value.String())
		})),
	)
	if err != nil {
		return heartbeat.ResolvedPolicy{}, "", "", err
	}
	dir, dirErr := filepath.EvalSymlinks(filepath.Dir(agent.SourcePath))
	if dirErr != nil {
		dir = filepath.Dir(agent.SourcePath)
	}
	inputErr := pathErr
	if inputErr == nil && readErr != nil && !os.IsNotExist(readErr) {
		inputErr = readErr
	}
	if inputErr == nil {
		inputErr = dirErr
	}
	key := authorityPolicyKey{profile: info.ProfileID, dir: dir, digest: sha256.Sum256(data)}
	var material publishedHeartbeat
	switch resolver := r.agentResolver.(type) {
	case *resourceAgentCatalog:
		lens := agentCatalogLensForIdentity(target.WorkspaceID, target.ProfileID, target.ProfileName)
		material = resolver.capturePublishedHeartbeat(target.AgentName, lens)
		key.published = material.stamp()
	case heartbeat.PolicyResolver:
		// An alternate resolver cannot supply immutable source material for a reusable key.
		if build, ok := ctx.Value(projectionPolicyBuildKey{}).(*projectionPolicyBuild); ok {
			build.mismatch.Store(true)
		}
		if inputErr != nil {
			return heartbeat.ResolvedPolicy{}, "", "", inputErr
		}
		return r.resolveAlternateAuthorityPolicy(ctx, agent, target, resolver, dir)
	}
	if build, ok := ctx.Value(projectionPolicyBuildKey{}).(*projectionPolicyBuild); ok {
		build.checkAuthorityMaterial(info.ID, authorityBodyDigest(body, readErr), key.published)
	}
	r.authorityMu.Lock()
	defer r.authorityMu.Unlock()
	if cached, ok := r.authorityCache[key]; ok {
		return cached.policy, cached.digest, cached.dir, cached.err
	}
	value := r.loadAuthorityPolicy(ctx, agent, info, target, key, material, body, readErr, inputErr)
	if ctx.Err() != nil {
		return heartbeat.ResolvedPolicy{}, "", "", ctx.Err()
	}
	if r.authorityCache == nil || len(r.authorityCache) >= instanceSourceDeclarationCacheLimit {
		r.authorityCache = make(map[authorityPolicyKey]authorityPolicyValue)
	}
	for old := range r.authorityCache {
		if old.profile == key.profile && old.dir == key.dir {
			delete(r.authorityCache, old)
		}
	}
	r.authorityCache[key] = value

	return value.policy, value.digest, value.dir, value.err
}

func authorityReadError(errs ...error) string {
	for _, err := range errs {
		if err != nil && !os.IsNotExist(err) {
			return err.Error()
		}
	}
	return ""
}

func toolPolicyAuthorityTarget(
	agent compozyconfig.AgentDef,
	info *session.Info,
	workspace *workspacepkg.ResolvedAgentConfig,
	cfg *compozyconfig.Config,
) heartbeat.AuthoringTarget {
	var root, profileName string
	if workspace != nil {
		root, profileName = workspace.RootDir, workspace.ProfileName
	}
	return heartbeat.AuthoringTarget{
		AgentPath:     agent.SourcePath,
		WorkspaceRoot: root,
		AgentName:     info.AgentName,
		ProfileID:     info.ProfileID,
		ProfileName:   profileName,
		WorkspaceID:   info.WorkspaceID,
		Config:        cfg.Agents.Heartbeat,
		CheckInLimits: &heartbeat.CheckInLimits{
			MaxToolCalls:  cfg.Agents.AlwaysOn.CheckInMaxToolCalls,
			MaxWriteCalls: cfg.Agents.AlwaysOn.CheckInMaxWriteCalls,
		},
	}
}

func authorityBodyDigest(body []byte, err error) [sha256.Size]byte {
	if err != nil {
		return [sha256.Size]byte{}
	}
	return sha256.Sum256(body)
}

func (r *nativeToolPolicyResolver) resolveAlternateAuthorityPolicy(
	ctx context.Context, agent compozyconfig.AgentDef,
	target heartbeat.AuthoringTarget, resolver heartbeat.PolicyResolver, dir string,
) (heartbeat.ResolvedPolicy, string, string, error) {
	policy, err := resolveTimerPolicy(ctx, resolver, target)
	if err != nil {
		return policy, "", dir, err
	}
	digest, err := authorityPolicyDigest(agent, &policy)
	return policy, digest, dir, err
}

func authorityPolicyDigest(agent compozyconfig.AgentDef, policy *heartbeat.ResolvedPolicy) (string, error) {
	return heartbeat.AuthorityDigest(
		policy.Frontmatter.CheckIns,
		heartbeat.AuthorityActions(policy.Frontmatter.Actions),
		heartbeat.AuthorityOverrides(agent.MCPServers),
	)
}

func (r *nativeToolPolicyResolver) loadAuthorityPolicy(
	ctx context.Context, agent compozyconfig.AgentDef, info *session.Info,
	target heartbeat.AuthoringTarget, key authorityPolicyKey, material publishedHeartbeat,
	body []byte, readErr, inputErr error,
) authorityPolicyValue {
	value := authorityPolicyValue{err: inputErr, dir: key.dir}
	if value.err == nil {
		if material.owned {
			value.policy, value.err = material.resolve(ctx, target)
		} else {
			value.policy, value.err = heartbeat.ResolveFromContent(ctx, heartbeat.ResolveRequest{
				CheckInLimits: target.CheckInLimits,
				AgentPath:     target.AgentPath,
				WorkspaceRoot: timerPolicyRoot(target),
				Config:        target.Config,
			}, body, readErr)
		}
	}
	if value.err == nil {
		value.err = finishTimerPolicy(&value.policy, target)
	}
	if value.err == nil {
		value.digest, value.err = authorityPolicyDigest(agent, &value.policy)
	}
	if value.err != nil && ctx.Err() == nil {
		slog.Warn("heartbeat.authority.unavailable", "profile_id", info.ProfileID, "agent_name", info.AgentName,
			"definition_dir", key.dir, "digest", key.digest, "error", value.err)
	}
	return value
}
