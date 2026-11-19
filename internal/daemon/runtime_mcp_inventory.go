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

type runtimeMCPObservation struct {
	key      string
	revision uint64
	unknown  bool
}

type runtimeMCPObservationKey struct{}

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
		if !ok || !slices.Equal(paths, cached.paths) {
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
		changed := !ok || !sameRuntimeMCPStamps(cached.stamps, after) || !slices.Equal(cached.paths, paths)
		cached = runtimeMCPCacheEntry{
			revision: cached.revision, readContext: string(contextData), inventory: inventory, paths: paths,
			stamps: after, expires: s.now().Add(runtimeMCPCacheTTL),
		}
		if !sameRuntimeMCPStamps(before, after) {
			cached.expires = time.Time{}
		}
		s.mu.Lock()
		current, exists := s.cache[key]
		switch {
		case exists != ok || exists && current.revision != cached.revision:
			// Another read or poll advanced this context while we read its files.
			// This result can answer the read but cannot fence a cached summary.
			cached.revision = 0
		default:
			if !exists {
				s.evict()
			}
			if changed {
				s.revision++
				cached.revision = s.revision
			}
			s.cache[key] = cached
		}
		s.mu.Unlock()
	}
	result := cloneRuntimeMCPInventory(cached.inventory)
	result.GeneratedAt = s.now().UTC()
	if observed, ok := ctx.Value(runtimeMCPObservationKey{}).(*runtimeMCPObservation); ok {
		switch {
		case observed.key == "":
			*observed = runtimeMCPObservation{key: key, revision: cached.revision}
		case observed.key != key || observed.revision != cached.revision:
			observed.unknown = true
		}
	}
	return result, nil
}

func (s *runtimeMCPInventory) evict() {
	if len(s.cache) < runtimeMCPCacheLimit {
		return
	}
	// An expired value still supplies the last observed file stamps. Drop it only
	// to enforce the size bound, so expiry of one context cannot invalidate others.
	var oldest string
	var expires time.Time
	for key, entry := range s.cache {
		if oldest == "" || entry.expires.Before(expires) {
			oldest, expires = key, entry.expires
		}
	}
	delete(s.cache, oldest)
}

func (s *runtimeMCPInventory) Poll() {
	s.mu.Lock()
	entries := maps.Clone(s.cache)
	s.mu.Unlock()
	files := make(map[string]filesnap.Snapshot)
	observed := make(map[string]bool)
	for key, entry := range entries {
		if runtimeMCPFilesUnchanged(entry, files, observed) {
			continue
		}
		s.mu.Lock()
		if current, ok := s.cache[key]; ok && current.revision == entry.revision {
			// Retain the last activation identity if the next read is transiently unreadable.
			if !current.expires.IsZero() {
				s.revision++
				current.revision = s.revision
			}
			current.expires = time.Time{}
			s.cache[key] = current
		}
		s.mu.Unlock()
	}
}

func runtimeMCPFilesUnchanged(
	entry runtimeMCPCacheEntry, files map[string]filesnap.Snapshot, observed map[string]bool,
) bool {
	for _, path := range entry.paths {
		if !observed[path] {
			if snapshot, found := captureRuntimeMCPFile(path); found {
				files[path] = snapshot
			}
			observed[path] = true
		}
		current, present := files[path]
		expected, existed := entry.stamps[path]
		if present != existed || present && !expected.Equal(current) {
			return false
		}
	}
	return true
}

func (s *runtimeMCPInventory) matches(observation runtimeMCPObservation) bool {
	if observation.unknown {
		return false
	}
	if observation.key == "" || s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, found := s.cache[observation.key]
	return found && entry.revision == observation.revision && !entry.expires.IsZero()
}

func captureRuntimeMCPFiles(paths []string) map[string]filesnap.Snapshot {
	result := make(map[string]filesnap.Snapshot, len(paths))
	for _, path := range paths {
		if snapshot, present := captureRuntimeMCPFile(path); present {
			result[path] = snapshot
		}
	}
	return result
}

func captureRuntimeMCPFile(path string) (filesnap.Snapshot, bool) {
	// Most runtime locations are absent. A single lookup can prove absence without
	// walking each ancestor to canonicalize a path we cannot open.
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return filesnap.Snapshot{}, false
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filesnap.Uncacheable(), !errors.Is(err, os.ErrNotExist)
	}
	file, err := fileutil.OpenRegularFile(resolved)
	if errors.Is(err, fileutil.ErrDirectory) {
		return captureRuntimeMCPDirectory(path, resolved), true
	}
	if err != nil {
		return filesnap.Uncacheable(), !errors.Is(err, os.ErrNotExist)
	}
	snapshot, statErr := filesnap.FromFile(file)
	if closeErr := file.Close(); statErr != nil || closeErr != nil {
		return filesnap.Uncacheable(), true
	}
	return snapshot, true
}

func captureRuntimeMCPDirectory(path, resolved string) filesnap.Snapshot {
	dir, err := fileutil.OpenDirectory(resolved)
	if err != nil {
		return filesnap.Uncacheable()
	}
	info, statErr := dir.Stat()
	if closeErr := dir.Close(); statErr != nil || closeErr != nil {
		return filesnap.Uncacheable()
	}
	return filesnap.FromInfo(path, info)
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

func (s *runtimeMCPInventory) Revision() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}
