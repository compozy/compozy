package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
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
	publication uint64
	readContext string
	inventory   runtimemcp.Inventory
	paths       []string
	stamps      map[string]filesnap.Snapshot
	expires     time.Time
}

type runtimeMCPPollEntry struct {
	key         string
	publication uint64
	paths       []string
	stamps      map[string]filesnap.Snapshot
}

type runtimeMCPObservation struct {
	key      string
	revision uint64
	unknown  bool
}

type runtimeMCPObservationKey struct{}

type runtimeMCPInventory struct {
	reader      runtimemcp.Reader
	revision    uint64
	publication uint64
	mu          sync.Mutex
	cache       map[string]runtimeMCPCacheEntry
	now         func() time.Time
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
	cached, err := s.readInventory(ctx, req, key)
	if err != nil {
		return runtimemcp.Inventory{}, fmt.Errorf("daemon: read runtime MCP inventory: %w", err)
	}
	result := cloneRuntimeMCPInventory(cached.inventory)
	result.GeneratedAt = s.now().UTC()
	if err := ctx.Err(); err != nil {
		return runtimemcp.Inventory{}, err
	}
	if observed, ok := ctx.Value(runtimeMCPObservationKey{}).(*runtimeMCPObservation); ok {
		switch {
		case cached.expires.IsZero():
			observed.unknown = true
		case observed.unknown:
		case observed.key == "":
			*observed = runtimeMCPObservation{key: key, revision: cached.revision}
		case observed.key != key || observed.revision != cached.revision:
			observed.unknown = true
		}
	}
	return result, nil
}

func (s *runtimeMCPInventory) readInventory(
	ctx context.Context, req runtimemcp.Request, key string,
) (runtimeMCPCacheEntry, error) {
	s.mu.Lock()
	cached, found := s.cache[key]
	s.mu.Unlock()
	before, err := captureRuntimeMCPFiles(ctx, cached.paths)
	if err != nil {
		return runtimeMCPCacheEntry{}, err
	}
	if found && s.now().Before(cached.expires) && sameRuntimeMCPStamps(cached.stamps, before) {
		return cached, nil
	}
	inventory, paths, err := s.reader.List(ctx, req)
	if err != nil {
		return runtimeMCPCacheEntry{}, err
	}
	after, err := captureRuntimeMCPFiles(ctx, paths)
	if err != nil {
		return runtimeMCPCacheEntry{}, err
	}
	if !found || !slices.Equal(paths, cached.paths) {
		before = after
		inventory, paths, err = s.reader.List(ctx, req)
		if err != nil {
			return runtimeMCPCacheEntry{}, err
		}
		after, err = captureRuntimeMCPFiles(ctx, paths)
		if err != nil {
			return runtimeMCPCacheEntry{}, err
		}
	}
	retainRuntimeMCPActivationDigests(&inventory, cached.inventory)
	contextData, err := json.Marshal(req.Context, json.Deterministic(true))
	if err != nil {
		return runtimeMCPCacheEntry{}, err
	}
	changed := !found || !sameRuntimeMCPStamps(cached.stamps, after) || !slices.Equal(cached.paths, paths)
	entry := runtimeMCPCacheEntry{
		revision:    cached.revision,
		publication: cached.publication,
		readContext: string(contextData),
		inventory:   inventory,
		paths:       paths,
		stamps:      after,
		expires:     s.now().Add(runtimeMCPCacheTTL),
	}
	if !sameRuntimeMCPStamps(before, after) {
		entry.expires = time.Time{}
	}
	if err := s.storeInventory(ctx, key, &entry, found, changed); err != nil {
		return runtimeMCPCacheEntry{}, err
	}
	return entry, nil
}

func retainRuntimeMCPActivationDigests(inventory *runtimemcp.Inventory, previous runtimemcp.Inventory) {
	for i := range inventory.Runtimes {
		current := &inventory.Runtimes[i]
		if current.ReadState != "unreadable" {
			continue
		}
		for _, old := range previous.Runtimes {
			if old.Runtime == current.Runtime {
				current.ActivationDigest = old.ActivationDigest
			}
		}
	}
}

func (s *runtimeMCPInventory) storeInventory(
	ctx context.Context, key string, entry *runtimeMCPCacheEntry, found, changed bool,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	current, exists := s.cache[key]
	if exists != found || exists && current.publication != entry.publication {
		// Another read or poll advanced this context. Its revision cannot fence this result.
		entry.revision = 0
		return nil
	}
	if !exists {
		s.evict()
	}
	if changed {
		s.revision++
		entry.revision = s.revision
		s.publication++
		entry.publication = s.publication
	}
	s.cache[key] = *entry
	return nil
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

func (s *runtimeMCPInventory) Poll(ctx context.Context) {
	s.mu.Lock()
	entries := make([]runtimeMCPPollEntry, 0, len(s.cache))
	for key, entry := range s.cache {
		entries = append(entries, runtimeMCPPollEntry{key, entry.publication, entry.paths, entry.stamps})
	}
	s.mu.Unlock()
	files := make(map[string]filesnap.Snapshot)
	observed := make(map[string]bool)
	for _, entry := range entries {
		unchanged, err := runtimeMCPFilesUnchanged(ctx, entry, files, observed)
		if err != nil {
			return
		}
		if unchanged {
			continue
		}
		s.mu.Lock()
		if ctx.Err() != nil {
			s.mu.Unlock()
			return
		}
		if current, ok := s.cache[entry.key]; ok && current.publication == entry.publication {
			// Retain the last activation identity if the next read is transiently unreadable.
			if !current.expires.IsZero() {
				s.revision++
				current.revision = s.revision
			}
			current.expires = time.Time{}
			s.publication++
			current.publication = s.publication
			s.cache[entry.key] = current
		}
		s.mu.Unlock()
	}
}

func runtimeMCPFilesUnchanged(
	ctx context.Context, entry runtimeMCPPollEntry, files map[string]filesnap.Snapshot, observed map[string]bool,
) (bool, error) {
	for _, path := range entry.paths {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if !observed[path] {
			snapshot, found, err := captureRuntimeMCPFile(ctx, path)
			if err != nil {
				return false, err
			}
			if found {
				files[path] = snapshot
			}
			observed[path] = true
		}
		current, present := files[path]
		expected, existed := entry.stamps[path]
		if present != existed || present && !expected.Equal(current) {
			return false, ctx.Err()
		}
	}
	return true, ctx.Err()
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

func captureRuntimeMCPFiles(ctx context.Context, paths []string) (map[string]filesnap.Snapshot, error) {
	result := make(map[string]filesnap.Snapshot, len(paths))
	for _, path := range paths {
		snapshot, present, err := captureRuntimeMCPFile(ctx, path)
		if err != nil {
			return nil, err
		}
		if present {
			result[path] = snapshot
		}
	}
	return result, ctx.Err()
}

func captureRuntimeMCPFile(ctx context.Context, path string) (filesnap.Snapshot, bool, error) {
	if err := ctx.Err(); err != nil {
		return filesnap.Snapshot{}, false, err
	}
	// Most locations are absent. Avoid resolving any ancestors in that case.
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return filesnap.Snapshot{}, false, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return filesnap.Snapshot{}, false, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if canceled := ctx.Err(); canceled != nil {
		return filesnap.Snapshot{}, false, canceled
	}
	if err != nil {
		return filesnap.Uncacheable(), !errors.Is(err, os.ErrNotExist), nil
	}
	file, err := fileutil.OpenRegularFile(resolved)
	if errors.Is(err, fileutil.ErrDirectory) {
		snapshot, err := captureRuntimeMCPDirectory(ctx, path, resolved)
		return snapshot, true, err
	}
	if err != nil {
		return filesnap.Uncacheable(), !errors.Is(err, os.ErrNotExist), ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		_ = file.Close()
		return filesnap.Snapshot{}, false, err
	}
	snapshot, statErr := filesnap.FromFile(file)
	if closeErr := file.Close(); statErr != nil || closeErr != nil {
		return filesnap.Uncacheable(), true, ctx.Err()
	}
	return snapshot, true, ctx.Err()
}

func captureRuntimeMCPDirectory(ctx context.Context, path, resolved string) (filesnap.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return filesnap.Snapshot{}, err
	}
	dir, err := fileutil.OpenDirectory(resolved)
	if err != nil {
		return filesnap.Uncacheable(), ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		_ = dir.Close()
		return filesnap.Snapshot{}, err
	}
	info, statErr := dir.Stat()
	if closeErr := dir.Close(); statErr != nil || closeErr != nil {
		return filesnap.Uncacheable(), ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return filesnap.Snapshot{}, err
	}
	return filesnap.FromInfo(path, info), ctx.Err()
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
