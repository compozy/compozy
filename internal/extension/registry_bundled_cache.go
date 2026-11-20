package extensionpkg

import (
	"context"
	"slices"
)

// EnabledBundledNames returns a cached snapshot of enabled bundled extension names.
// A tombstoned name is excluded even when a bundled row still exists, so a permanently
// removed built-in grants no tool trust.
func (r *Registry) EnabledBundledNames() ([]string, error) {
	if err := r.checkReady("list enabled bundled extensions"); err != nil {
		return nil, err
	}
	r.bundledCacheMu.RLock()
	if r.bundledCacheLoaded {
		names := slices.Clone(r.bundledCacheNames)
		r.bundledCacheMu.RUnlock()
		return names, nil
	}
	r.bundledCacheMu.RUnlock()

	r.bundledCacheMu.Lock()
	defer r.bundledCacheMu.Unlock()
	if r.bundledCacheLoaded {
		return slices.Clone(r.bundledCacheNames), nil
	}
	infos, err := r.List()
	if err != nil {
		return nil, err
	}
	tombstones, err := r.ListTombstones(context.Background())
	if err != nil {
		return nil, err
	}
	tombstoned := make(map[string]struct{}, len(tombstones))
	for _, tombstone := range tombstones {
		tombstoned[tombstone.Name] = struct{}{}
	}
	names := make([]string, 0)
	for infoIndex := range infos {
		if !infos[infoIndex].Enabled ||
			infos[infoIndex].Source != SourceBundled {
			continue
		}
		if _, removed := tombstoned[infos[infoIndex].Name]; removed {
			continue
		}
		names = append(names, infos[infoIndex].Name)
	}
	r.bundledCacheNames = names
	r.bundledCacheLoaded = true
	return slices.Clone(names), nil
}

func (r *Registry) invalidateEnabledBundledNames() {
	if r == nil {
		return
	}
	r.bundledCacheMu.Lock()
	defer r.bundledCacheMu.Unlock()
	r.bundledCacheLoaded = false
	r.bundledCacheNames = nil
}
