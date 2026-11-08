package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/managedsidecar"
	"github.com/compozy/compozy/internal/session"
)

const instanceSourceDeclarationCacheLimit = 128

type instanceSourceDeclarationKey struct {
	path, root   string
	config, body [32]byte
	file         filesnap.Snapshot
	packageOwned bool
	limits       heartbeat.CheckInLimits
}

type instanceSourceDeclarations struct {
	mu   sync.Mutex
	refs map[instanceSourceDeclarationKey][]source.Ref
}

func (c *instanceSourceDeclarations) resolve(
	ctx context.Context,
	agents session.AgentResolver,
	artifacts *session.AgentArtifacts,
	target heartbeat.AuthoringTarget,
) ([]source.Ref, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key, err := instanceSourceKey(artifacts, target)
	if err != nil {
		return nil, err
	}
	if c != nil {
		c.mu.Lock()
		refs, ok := c.refs[key]
		c.mu.Unlock()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if ok {
			return refs, nil
		}
	}
	var policy heartbeat.ResolvedPolicy
	if artifacts != nil && artifacts.PackageOwned {
		if strings.TrimSpace(artifacts.HeartbeatBody) == "" {
			policy, err = heartbeat.Empty(target.Config, artifacts.HeartbeatSourcePath)
		} else {
			policy, err = heartbeat.Parse(ctx, heartbeat.ParseRequest{
				SourcePath: artifacts.HeartbeatSourcePath, WorkspaceRoot: target.WorkspaceRoot,
				Content: []byte(artifacts.HeartbeatBody), Config: target.Config,
				CheckInLimits: target.CheckInLimits,
			})
		}
	} else {
		var resolver heartbeat.PolicyResolver
		if artifacts == nil {
			if candidate, ok := agents.(heartbeat.PolicyResolver); ok {
				resolver = candidate
			}
		}
		policy, err = resolveTimerPolicy(ctx, resolver, target)
	}
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	refs := policy.Preferences.Context.Sources
	if c != nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.refs == nil || len(c.refs) >= instanceSourceDeclarationCacheLimit {
			c.refs = make(map[instanceSourceDeclarationKey][]source.Ref)
		}
		// Only retain the latest freshness key for a policy in this scope.
		for old := range c.refs {
			if old.path == key.path && old.root == key.root {
				delete(c.refs, old)
			}
		}
		c.refs[key] = refs
	}
	return refs, nil
}

func instanceSourceKey(
	artifacts *session.AgentArtifacts,
	target heartbeat.AuthoringTarget,
) (instanceSourceDeclarationKey, error) {
	config, err := json.Marshal(target.Config, json.Deterministic(true),
		json.WithMarshalers(json.MarshalFunc(func(value time.Duration) ([]byte, error) {
			return json.Marshal(value.String())
		})))
	if err != nil {
		return instanceSourceDeclarationKey{}, err
	}
	key := instanceSourceDeclarationKey{
		root: timerPolicyRoot(target), config: sha256.Sum256(config),
	}
	if target.CheckInLimits != nil {
		key.limits = *target.CheckInLimits
	}
	if artifacts != nil && artifacts.PackageOwned {
		key.path = artifacts.HeartbeatSourcePath
		key.root = target.WorkspaceRoot
		key.packageOwned = true
		key.body = sha256.Sum256([]byte(artifacts.HeartbeatBody))
		return key, nil
	}
	key.path, err = managedsidecar.SidecarPath(target.AgentPath, heartbeat.FileName)
	if err != nil {
		return key, err
	}
	// Resolve symlinks on every read so a retarget cannot reuse a validated projection.
	path, err := filepath.EvalSymlinks(key.path)
	if errors.Is(err, os.ErrNotExist) {
		return key, nil
	}
	if err != nil {
		return key, err
	}
	key.path = path
	root, err := filepath.EvalSymlinks(key.root)
	if err != nil {
		return key, err
	}
	key.root = root
	info, err := os.Stat(path)
	if err != nil {
		return key, err
	}
	key.file = filesnap.FromInfo(path, info)
	return key, nil
}
