package extensionpkg

import "slices"

// EnabledBundledNames returns a cached snapshot of enabled bundled extension names.
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
	names := make([]string, 0)
	for _, info := range infos {
		if info.Enabled && info.Source == SourceBundled {
			names = append(names, info.Name)
		}
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
