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
	key := authorityPolicyKey{profile: info.ProfileID, dir: dir, digest: sha256.Sum256(data)}
	r.authorityMu.Lock()
	defer r.authorityMu.Unlock()
	if cached, ok := r.authorityCache[key]; ok {
		return cached.policy, cached.digest, cached.dir, cached.err
	}
	value := r.loadAuthorityPolicy(ctx, agent, info, target, key.digest, pathErr, readErr, dirErr)
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
	root := ""
	if workspace != nil {
		root = workspace.RootDir
	}
	return heartbeat.AuthoringTarget{
		AgentPath:     agent.SourcePath,
		WorkspaceRoot: root,
		AgentName:     info.AgentName,
		ProfileID:     info.ProfileID,
		WorkspaceID:   info.WorkspaceID,
		Config:        cfg.Agents.Heartbeat,
		CheckInLimits: &heartbeat.CheckInLimits{
			MaxToolCalls:  cfg.Agents.AlwaysOn.CheckInMaxToolCalls,
			MaxWriteCalls: cfg.Agents.AlwaysOn.CheckInMaxWriteCalls,
		},
	}
}

func (r *nativeToolPolicyResolver) loadAuthorityPolicy(
	ctx context.Context, agent compozyconfig.AgentDef, info *session.Info,
	target heartbeat.AuthoringTarget, digest [32]byte, pathErr, readErr, dirErr error,
) authorityPolicyValue {
	var resolver heartbeat.PolicyResolver
	if candidate, ok := r.agentResolver.(heartbeat.PolicyResolver); ok {
		resolver = candidate
	}
	value := authorityPolicyValue{err: pathErr}
	if value.err == nil && readErr != nil && !os.IsNotExist(readErr) {
		value.err = readErr
	}
	if value.err == nil {
		value.err = dirErr
	}
	if value.err == nil {
		value.policy, value.err = resolveTimerPolicy(ctx, resolver, target)
	}
	if value.err == nil {
		value.digest, value.err = heartbeat.AuthorityDigest(
			value.policy.Frontmatter.CheckIns,
			heartbeat.AuthorityActions(value.policy.Frontmatter.Actions),
			heartbeat.AuthorityOverrides(agent.MCPServers),
		)
	}
	if value.err == nil {
		value.dir, value.err = filepath.EvalSymlinks(filepath.Dir(agent.SourcePath))
	}
	if ctx.Err() == nil {
		if value.err != nil {
			slog.Warn(
				"heartbeat.authority.unavailable",
				"profile_id",
				info.ProfileID,
				"agent_name",
				info.AgentName,
				"definition_dir",
				filepath.Dir(agent.SourcePath),
				"digest",
				digest,
				"error",
				value.err,
			)
		}
	}
	return value
}
