package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/runtimemcp"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

const runtimeMCPCacheTTL = 45 * time.Second

type runtimeMCPCacheEntry struct {
	inventory runtimemcp.Inventory
	paths     []string
	stamps    map[string]filesnap.Snapshot
	expires   time.Time
}

type runtimeMCPInventory struct {
	mu    sync.Mutex
	cache map[string]runtimeMCPCacheEntry
	now   func() time.Time
}

var _ settingspkg.RuntimeMCPInventory = (*runtimeMCPInventory)(nil)

func newRuntimeMCPInventory() *runtimeMCPInventory {
	return &runtimeMCPInventory{cache: map[string]runtimeMCPCacheEntry{}, now: time.Now}
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
	defer s.mu.Unlock()
	cached, ok := s.cache[key]
	if !ok || !s.now().Before(cached.expires) || !sameRuntimeMCPStamps(cached.stamps, captureRuntimeMCPFiles(cached.paths)) {
		before := captureRuntimeMCPFiles(cached.paths)
		inventory, paths, err := (runtimemcp.Reader{}).List(ctx, req)
		if err != nil {
			return runtimemcp.Inventory{}, err
		}
		after := captureRuntimeMCPFiles(paths)
		if !slices.Equal(paths, cached.paths) {
			before = after
			inventory, paths, err = (runtimemcp.Reader{}).List(ctx, req)
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
		cached = runtimeMCPCacheEntry{inventory: inventory, paths: paths, stamps: after, expires: s.now().Add(runtimeMCPCacheTTL)}
		if !sameRuntimeMCPStamps(before, after) {
			cached.expires = time.Time{}
		}
		s.cache[key] = cached
	}
	result := cloneRuntimeMCPInventory(cached.inventory)
	result.GeneratedAt = s.now().UTC()
	return result, nil
}

func (s *runtimeMCPInventory) Poll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, entry := range s.cache {
		if !sameRuntimeMCPStamps(entry.stamps, captureRuntimeMCPFiles(entry.paths)) {
			entry.expires = time.Time{}
			s.cache[key] = entry
		}
	}
}

func captureRuntimeMCPFiles(paths []string) map[string]filesnap.Snapshot {
	result := make(map[string]filesnap.Snapshot, len(paths))
	for _, path := range paths {
		file, err := fileutil.OpenRegularFile(path)
		if errors.Is(err, fileutil.ErrDirectory) {
			dir, dirErr := fileutil.OpenDirectory(path)
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
