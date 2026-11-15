package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/runtimemcp"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/vault"
)

const runtimeMCPCacheTTL = 45 * time.Second
const runtimeMCPCacheLimit = 128

type runtimeMCPCacheEntry struct {
	revision    uint64
	readContext string
	inventory   runtimemcp.Inventory
	paths       []string
	stamps      map[string]filesnap.Snapshot
	expires     time.Time
}

type runtimeMCPInventory struct {
	reader   runtimemcp.Reader
	revision uint64
	mu       sync.Mutex
	cache    map[string]runtimeMCPCacheEntry
	now      func() time.Time
}

var _ settingspkg.RuntimeMCPInventory = (*runtimeMCPInventory)(nil)

func newRuntimeMCPInventory(keys vault.KeyProvider) (*runtimeMCPInventory, error) {
	if keys == nil {
		return nil, errors.New("daemon: MCP digest key provider is required")
	}
	key, err := keys.Key()
	if err != nil {
		return nil, err
	}
	reader, err := runtimemcp.NewReader(key)
	if err != nil {
		return nil, err
	}
	return &runtimeMCPInventory{reader: reader, cache: map[string]runtimeMCPCacheEntry{}, now: time.Now}, nil
}

func (s *runtimeMCPInventory) List(ctx context.Context, req runtimemcp.Request) (runtimemcp.Inventory, error) {
	req.Context = runtimemcp.ResolveContext(req.Context)
	data, err := json.Marshal(req, json.Deterministic(true))
	if err != nil {
		return runtimemcp.Inventory{}, err
	}
	hash := sha256.Sum256(data)
	key := hex.EncodeToString(hash[:])
	s.mu.Lock()
	cached, ok := s.cache[key]
	s.mu.Unlock()
	before := captureRuntimeMCPFiles(cached.paths)
	if !ok || !s.now().Before(cached.expires) || !sameRuntimeMCPStamps(cached.stamps, before) {
		inventory, paths, err := s.reader.List(ctx, req)
		if err != nil {
			return runtimemcp.Inventory{}, err
		}
		after := captureRuntimeMCPFiles(paths)
		if !ok {
			before = after
		} else if !slices.Equal(paths, cached.paths) {
			before = after
			inventory, paths, err = s.reader.List(ctx, req)
			if err != nil {
				return runtimemcp.Inventory{}, err
			}
			after = captureRuntimeMCPFiles(paths)
		}
		for i := range inventory.Runtimes {
			current := &inventory.Runtimes[i]
			if current.ReadState != "unreadable" {
				continue
			}
			for _, old := range cached.inventory.Runtimes {
				if old.Runtime == current.Runtime {
					current.ActivationDigest = old.ActivationDigest
				}
			}
		}
		contextData, err := json.Marshal(req.Context, json.Deterministic(true))
		if err != nil {
			return runtimemcp.Inventory{}, err
		}
		cached = runtimeMCPCacheEntry{
			readContext: string(contextData), inventory: inventory, paths: paths,
			stamps: after, expires: s.now().Add(runtimeMCPCacheTTL),
		}
		if !sameRuntimeMCPStamps(before, after) {
			cached.expires = time.Time{}
		}
		s.mu.Lock()
		s.evict(s.now())
		s.revision++
		cached.revision = s.revision
		s.cache[key] = cached
		s.mu.Unlock()
	}
	result := cloneRuntimeMCPInventory(cached.inventory)
	result.GeneratedAt = s.now().UTC()
	return result, nil
}

func (s *runtimeMCPInventory) evict(now time.Time) {
	var oldest string
	var expires time.Time
	for key, entry := range s.cache {
		if !now.Before(entry.expires) {
			delete(s.cache, key)
			continue
		}
		if oldest == "" || entry.expires.Before(expires) {
			oldest, expires = key, entry.expires
		}
	}
	if len(s.cache) >= runtimeMCPCacheLimit {
		delete(s.cache, oldest)
	}
}

func (s *runtimeMCPInventory) Poll() {
	s.mu.Lock()
	entries := maps.Clone(s.cache)
	s.mu.Unlock()
	for key, entry := range entries {
		expired := !s.now().Before(entry.expires)
		if !expired && sameRuntimeMCPStamps(entry.stamps, captureRuntimeMCPFiles(entry.paths)) {
			continue
		}
		s.mu.Lock()
		if current, ok := s.cache[key]; ok && current.revision == entry.revision {
			// Retain the last activation identity if the next read is transiently unreadable.
			current.expires = time.Time{}
			s.cache[key] = current
		}
		s.mu.Unlock()
	}
}

func captureRuntimeMCPFiles(paths []string) map[string]filesnap.Snapshot {
	result := make(map[string]filesnap.Snapshot, len(paths))
	for _, path := range paths {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result[path] = filesnap.Uncacheable()
			}
			continue
		}
		file, err := fileutil.OpenRegularFile(resolved)
		if errors.Is(err, fileutil.ErrDirectory) {
			dir, dirErr := fileutil.OpenDirectory(resolved)
			if dirErr != nil {
				result[path] = filesnap.Uncacheable()
				continue
			}
			info, statErr := dir.Stat()
			closeErr := dir.Close()
			if statErr != nil || closeErr != nil {
				result[path] = filesnap.Uncacheable()
			} else {
				result[path] = filesnap.FromInfo(path, info)
			}
			continue
		}
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				result[path] = filesnap.Uncacheable()
			}
			continue
		}
		snapshot, statErr := filesnap.FromFile(file)
		closeErr := file.Close()
		if statErr != nil || closeErr != nil {
			result[path] = filesnap.Uncacheable()
		} else {
			result[path] = snapshot
		}
	}
	return result
}

func sameRuntimeMCPStamps(a, b map[string]filesnap.Snapshot) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		other, ok := b[key]
		if !ok || !value.Equal(other) {
			return false
		}
	}
	return true
}

func cloneRuntimeMCPInventory(value runtimemcp.Inventory) runtimemcp.Inventory {
	value.Runtimes = slices.Clone(value.Runtimes)
	for i := range value.Runtimes {
		group := &value.Runtimes[i]
		group.ConfigFiles = slices.Clone(group.ConfigFiles)
		group.Notes = slices.Clone(group.Notes)
		group.Servers = slices.Clone(group.Servers)
		for j := range group.Servers {
			group.Servers[j].EnvKeys = slices.Clone(group.Servers[j].EnvKeys)
			group.Servers[j].HeaderKeys = slices.Clone(group.Servers[j].HeaderKeys)
		}
	}
	return value
}

func (s *runtimeMCPInventory) Resolve(
	ctx context.Context,
	req runtimemcp.Request,
	name string,
) (runtimemcp.ResolvedServer, error) {
	req.Context = runtimemcp.ResolveContext(req.Context)
	data, err := json.Marshal(req.Context, json.Deterministic(true))
	if err != nil {
		return runtimemcp.ResolvedServer{}, err
	}
	var previous *runtimemcp.Server
	var latest time.Time
	s.mu.Lock()
	for _, cached := range s.cache {
		if cached.readContext != string(data) || !cached.inventory.GeneratedAt.After(latest) {
			continue
		}
		for _, runtime := range cached.inventory.Runtimes {
			if runtime.Runtime != req.Runtime {
				continue
			}
			latest = cached.inventory.GeneratedAt
			previous = nil
			for _, server := range runtime.Servers {
				if server.Name == name {
					previous = new(server)
					break
				}
			}
		}
	}
	s.mu.Unlock()
	return s.reader.Resolve(ctx, req, name, previous)
}

var _ settingspkg.RuntimeMCPResolver = (*runtimeMCPInventory)(nil)
