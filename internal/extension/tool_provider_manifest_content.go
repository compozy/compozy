package extensionpkg

import (
	"path/filepath"
	"strings"
)

type extensionManifestContentState struct {
	descriptors []ManifestToolDescriptor
	err         error
}

type extensionManifestContentKey struct {
	path        string
	fingerprint string
}

type extensionManifestWarningKey struct {
	contentKey extensionManifestContentKey
	name       string
}

func (p *ExtensionToolProvider) manifestToolDescriptors(
	source *extensionManifestToolSource,
) ([]ManifestToolDescriptor, error) {
	contentKey := extensionManifestContentKeyForSource(source)
	if state, ok := p.cachedManifestContentState(contentKey); ok {
		return state.descriptors, state.err
	}

	state := extensionManifestContentState{err: source.snapshot.err}
	if state.err == nil {
		manifest, err := loadManifestContentAtPath(source.info.ManifestPath, source.snapshot.content)
		if err != nil {
			state.err = err
		} else {
			state.descriptors, state.err = ResolveManifestToolDescriptors(manifest)
		}
	}
	p.storeManifestContentState(contentKey, state)
	return cloneExtensionManifestContentState(state).descriptors, state.err
}

func (p *ExtensionToolProvider) claimManifestToolWarning(source *extensionManifestToolSource) bool {
	warningKey := extensionManifestWarningKey{
		contentKey: extensionManifestContentKeyForSource(source),
		name:       strings.TrimSpace(source.info.Name),
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.cache.manifestWarnings[warningKey]; ok {
		return false
	}
	if p.cache.manifestWarnings == nil {
		p.cache.manifestWarnings = make(map[extensionManifestWarningKey]struct{})
	}
	p.cache.manifestWarnings[warningKey] = struct{}{}
	return true
}

func (p *ExtensionToolProvider) cachedManifestContentState(
	contentKey extensionManifestContentKey,
) (extensionManifestContentState, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	state, ok := p.cache.manifestContentStates[contentKey]
	return cloneExtensionManifestContentState(state), ok
}

func (p *ExtensionToolProvider) storeManifestContentState(
	contentKey extensionManifestContentKey,
	state extensionManifestContentState,
) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache.manifestContentStates == nil {
		p.cache.manifestContentStates = make(map[extensionManifestContentKey]extensionManifestContentState)
	}
	p.cache.manifestContentStates[contentKey] = cloneExtensionManifestContentState(state)
}

func (p *ExtensionToolProvider) pruneManifestContentStatesLocked() {
	activeContentKeys := make(map[extensionManifestContentKey]struct{}, len(p.cache.manifestSnapshots))
	for path, snapshot := range p.cache.manifestSnapshots {
		if snapshot.fingerprint != "" {
			activeContentKeys[extensionManifestContentKey{
				path:        filepath.Clean(path),
				fingerprint: snapshot.fingerprint,
			}] = struct{}{}
		}
	}
	for contentKey := range p.cache.manifestContentStates {
		if _, ok := activeContentKeys[contentKey]; !ok {
			delete(p.cache.manifestContentStates, contentKey)
		}
	}
	for warningKey := range p.cache.manifestWarnings {
		if _, ok := activeContentKeys[warningKey.contentKey]; !ok {
			delete(p.cache.manifestWarnings, warningKey)
		}
	}
}

func extensionManifestContentKeyForSource(source *extensionManifestToolSource) extensionManifestContentKey {
	return extensionManifestContentKey{
		path:        filepath.Clean(source.info.ManifestPath),
		fingerprint: source.snapshot.fingerprint,
	}
}

func cloneExtensionManifestContentState(
	src extensionManifestContentState,
) extensionManifestContentState {
	cloned := extensionManifestContentState{err: src.err}
	cloned.descriptors = make([]ManifestToolDescriptor, len(src.descriptors))
	for i := range src.descriptors {
		cloned.descriptors[i] = cloneManifestToolDescriptor(&src.descriptors[i])
	}
	return cloned
}
