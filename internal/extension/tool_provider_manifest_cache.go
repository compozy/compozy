package extensionpkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type extensionManifestTool struct {
	info       ExtensionInfo
	key        InstanceKey
	descriptor ManifestToolDescriptor
}

const extensionManifestDisabledFingerprint = "disabled"

type extensionToolProviderCache struct {
	fingerprint                  string
	tools                        []extensionManifestTool
	manifestSnapshots            map[string]extensionManifestSnapshot
	manifestContentStates        map[extensionManifestContentKey]extensionManifestContentState
	manifestWarnings             map[extensionManifestWarningKey]struct{}
	manifestSnapshotPathsByScope map[string]map[string]struct{}
}

type extensionManifestToolSource struct {
	info     ExtensionInfo
	key      InstanceKey
	snapshot extensionManifestSnapshot
}

type extensionManifestSnapshot struct {
	content     []byte
	fingerprint string
	err         error
	metadata    extensionManifestFileMetadata
	reusable    bool
}

type extensionManifestReadFile func(string) ([]byte, error)

func (p *ExtensionToolProvider) manifestTool(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (extensionManifestTool, bool, error) {
	manifestTools, err := p.manifestTools(ctx, scope)
	if err != nil {
		return extensionManifestTool{}, false, err
	}
	for i := range manifestTools {
		if manifestTools[i].descriptor.Tool.ID == id {
			return manifestTools[i], true, nil
		}
	}
	return extensionManifestTool{}, false, nil
}

func (p *ExtensionToolProvider) manifestTools(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]extensionManifestTool, error) {
	tools, _, err := p.manifestToolsWithFingerprint(ctx, scope)
	return tools, err
}

func (p *ExtensionToolProvider) manifestToolsWithFingerprint(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]extensionManifestTool, string, error) {
	if p == nil || p.registry == nil {
		return nil, "", toolspkg.NewValidationError(
			"provider",
			toolspkg.ReasonDependencyMissing,
			"extension tool provider is required",
		)
	}
	if err := extensionProviderContextErr(ctx); err != nil {
		return nil, "", err
	}
	release, err := p.acquireManifestRefresh(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()
	if err := extensionProviderContextErr(ctx); err != nil {
		return nil, "", err
	}

	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	profileID := strings.TrimSpace(scope.ProfileID)
	if profileID == "" {
		profileID = store.DefaultProfileID
	}
	profileName := ""
	if p.profileNames != nil {
		profileName, err = p.profileNames.ProfileName(ctx, profileID)
		if err != nil {
			return nil, "", fmt.Errorf("extension: resolve tool profile %q: %w", profileID, err)
		}
		profileName = strings.TrimSpace(profileName)
	}
	infos, err := p.manifestToolInfos(workspaceID)
	if err != nil {
		return nil, "", err
	}
	sources, err := p.manifestToolSources(profileID, workspaceID, infos)
	if err != nil {
		return nil, "", err
	}
	scopeID := profileID + "\x00" + workspaceID
	p.reconcileManifestSnapshots(scopeID, sources)
	if err := extensionProviderContextErr(ctx); err != nil {
		return nil, "", err
	}
	fingerprint := extensionToolManifestFingerprint(profileID, profileName, workspaceID, sources)
	if cached, ok := p.cachedManifestTools(fingerprint); ok {
		return cached, fingerprint, nil
	}

	tools := p.resolveManifestTools(profileName, sources)
	if err := extensionProviderContextErr(ctx); err != nil {
		return nil, "", err
	}
	p.storeManifestTools(fingerprint, tools)
	return tools, fingerprint, nil
}

func (p *ExtensionToolProvider) acquireManifestRefresh(ctx context.Context) (func(), error) {
	select {
	case p.manifestRefresh <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := extensionProviderContextErr(ctx); err != nil {
		<-p.manifestRefresh
		return nil, err
	}
	return func() {
		<-p.manifestRefresh
	}, nil
}

func (p *ExtensionToolProvider) manifestToolInfos(workspaceID string) ([]ExtensionInfo, error) {
	if runtime := p.runtimeInstance(); runtime != nil {
		if scoped, ok := runtime.(extensionScopedToolRuntime); ok {
			if infos := scoped.ListForWorkspace(workspaceID); infos != nil {
				return infos, nil
			}
		}
	}
	infos, err := p.registry.List()
	if err != nil {
		return nil, fmt.Errorf("extension: list tool manifests: %w", err)
	}
	return infos, nil
}

func (p *ExtensionToolProvider) manifestToolSources(
	profileID string,
	workspaceID string,
	infos []ExtensionInfo,
) ([]extensionManifestToolSource, error) {
	sources := make([]extensionManifestToolSource, 0, len(infos))
	for i := range infos {
		info := infos[i]
		if normalizeExtensionFormat(info.Format) == FormatAgentPlugin {
			continue
		}
		enabled, err := p.registry.IsEnabledForProfile(info.Name, profileID)
		if err != nil {
			return nil, fmt.Errorf(
				"extension: resolve tool enablement for %q and profile %q: %w",
				info.Name,
				profileID,
				err,
			)
		}
		info.Enabled = enabled
		source := extensionManifestToolSource{info: info}
		if !info.Enabled {
			source.snapshot.fingerprint = extensionManifestDisabledFingerprint
			sources = append(sources, source)
			continue
		}

		source.key = ProfileInstanceKey(info.Name, profileID, "")
		if workspaceID != "" {
			if link, err := p.registry.GetDevLink(info.Name, workspaceID); err == nil &&
				strings.TrimSpace(link.BundleGeneration) == strings.TrimSpace(info.Checksum) {
				source.key.WorkspaceID = workspaceID
			}
		}
		source.snapshot = p.manifestSnapshot(info.ManifestPath)
		sources = append(sources, source)
	}
	return sources, nil
}

func (p *ExtensionToolProvider) manifestSnapshot(path string) extensionManifestSnapshot {
	info, err := os.Stat(path)
	if err != nil {
		metadata := unavailableExtensionManifestFileMetadata(err)
		if snapshot, ok := p.cachedManifestSnapshot(path, metadata); ok {
			return snapshot
		}
		snapshot := extensionManifestSnapshot{
			fingerprint: "unavailable:" + err.Error(),
			err:         fmt.Errorf("extension: read manifest %q: %w", path, err),
			metadata:    metadata,
			reusable:    true,
		}
		p.storeManifestSnapshot(path, snapshot)
		return snapshot
	}
	metadata := newExtensionManifestFileMetadata(info)
	if metadata.reliable {
		if snapshot, ok := p.cachedManifestSnapshot(path, metadata); ok {
			return snapshot
		}
	}
	// Platforms without a reliable change token must read and hash after each
	// successful stat so same-size, same-mtime edits remain observable.
	snapshot := p.readExtensionManifestSnapshot(path, metadata)
	p.storeManifestSnapshot(path, snapshot)
	return snapshot
}

func (p *ExtensionToolProvider) readExtensionManifestSnapshot(
	path string,
	metadata extensionManifestFileMetadata,
) extensionManifestSnapshot {
	content, err := p.manifestReadFile(path)
	if err != nil {
		return extensionManifestSnapshot{
			fingerprint: "unavailable:" + err.Error(),
			err:         fmt.Errorf("extension: read manifest %q: %w", path, err),
			metadata:    metadata,
			reusable:    false,
		}
	}
	digest := sha256.Sum256(content)
	return extensionManifestSnapshot{
		content:     content,
		fingerprint: "content:sha256:" + hex.EncodeToString(digest[:]),
		metadata:    metadata,
		reusable:    true,
	}
}

func (p *ExtensionToolProvider) resolveManifestTools(
	profileName string,
	sources []extensionManifestToolSource,
) []extensionManifestTool {
	manifestTools := make([]extensionManifestTool, 0)
	for i := range sources {
		source := &sources[i]
		if !source.info.Enabled {
			continue
		}

		descriptors, err := p.manifestToolDescriptors(source)
		if err != nil {
			if p.claimManifestToolWarning(source) {
				p.logInvalidToolManifest(source.info.Name, err)
			}
			continue
		}
		for j := range descriptors {
			if profileName != "" && !manifestPlacementVisible(descriptors[j].Profile, profileName) {
				continue
			}
			if descriptors[j].Tool.Backend.Kind != toolspkg.BackendExtensionHost {
				continue
			}
			manifestTools = append(manifestTools, extensionManifestTool{
				info:       cloneExtensionInfo(source.info),
				key:        source.key,
				descriptor: cloneManifestToolDescriptor(&descriptors[j]),
			})
		}
	}
	slices.SortFunc(manifestTools, func(left, right extensionManifestTool) int {
		return strings.Compare(left.descriptor.Tool.ID.String(), right.descriptor.Tool.ID.String())
	})
	return manifestTools
}

func (p *ExtensionToolProvider) logInvalidToolManifest(name string, err error) {
	if p == nil || p.logger == nil || err == nil {
		return
	}
	p.logger.Warn(
		"extension: skip invalid extension tool manifest",
		"extension_name", strings.TrimSpace(name),
		"error", err,
	)
}

func (p *ExtensionToolProvider) cachedManifestTools(fingerprint string) ([]extensionManifestTool, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.cache.fingerprint != fingerprint {
		return nil, false
	}
	return cloneExtensionManifestTools(p.cache.tools), true
}

func (p *ExtensionToolProvider) storeManifestTools(fingerprint string, tools []extensionManifestTool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cache.fingerprint = fingerprint
	p.cache.tools = cloneExtensionManifestTools(tools)
}

func (p *ExtensionToolProvider) cachedManifestSnapshot(
	path string,
	metadata extensionManifestFileMetadata,
) (extensionManifestSnapshot, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshot, ok := p.cache.manifestSnapshots[path]
	if !ok || !snapshot.reusable || !snapshot.metadata.Equal(metadata) {
		return extensionManifestSnapshot{}, false
	}
	return snapshot, true
}

func (p *ExtensionToolProvider) storeManifestSnapshot(path string, snapshot extensionManifestSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache.manifestSnapshots == nil {
		p.cache.manifestSnapshots = make(map[string]extensionManifestSnapshot)
	}
	p.cache.manifestSnapshots[path] = snapshot
	p.pruneManifestContentStatesLocked()
}

func (p *ExtensionToolProvider) reconcileManifestSnapshots(
	workspaceID string,
	sources []extensionManifestToolSource,
) {
	activeManifestPaths := make(map[string]struct{}, len(sources))
	for i := range sources {
		if path := strings.TrimSpace(sources[i].info.ManifestPath); path != "" {
			activeManifestPaths[sources[i].info.ManifestPath] = struct{}{}
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cache.manifestSnapshotPathsByScope == nil {
		p.cache.manifestSnapshotPathsByScope = make(map[string]map[string]struct{})
	}
	scopeID := strings.TrimSpace(workspaceID)
	if len(activeManifestPaths) == 0 {
		delete(p.cache.manifestSnapshotPathsByScope, scopeID)
	} else {
		p.cache.manifestSnapshotPathsByScope[scopeID] = activeManifestPaths
	}
	// Each effective scope replaces only its own active-path ownership. A poll
	// for one workspace cannot evict a global or peer-workspace snapshot; an
	// empty observed view releases only that scope's paths.
	ownedManifestPaths := make(map[string]struct{})
	for _, paths := range p.cache.manifestSnapshotPathsByScope {
		for path := range paths {
			ownedManifestPaths[path] = struct{}{}
		}
	}
	for path := range p.cache.manifestSnapshots {
		if _, ok := ownedManifestPaths[path]; !ok {
			delete(p.cache.manifestSnapshots, path)
		}
	}
	p.pruneManifestContentStatesLocked()
}

func extensionToolManifestFingerprint(
	profileID string,
	profileName string,
	workspaceID string,
	sources []extensionManifestToolSource,
) string {
	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(profileID))
	builder.WriteByte(0)
	builder.WriteString(strings.TrimSpace(profileName))
	builder.WriteByte(0)
	builder.WriteString(strings.TrimSpace(workspaceID))
	builder.WriteByte(0)
	for i := range sources {
		info := sources[i].info
		builder.WriteString(info.Name)
		builder.WriteByte(0)
		builder.WriteString(info.Version)
		builder.WriteByte(0)
		builder.WriteString(info.Source.String())
		builder.WriteByte(0)
		if info.Enabled {
			builder.WriteByte('1')
		} else {
			builder.WriteByte('0')
		}
		builder.WriteByte(0)
		builder.WriteString(info.ManifestPath)
		builder.WriteByte(0)
		builder.WriteString(info.Checksum)
		builder.WriteByte(0)
		builder.WriteString(sources[i].snapshot.fingerprint)
		builder.WriteByte(0)
	}
	return builder.String()
}

func cloneManifestToolDescriptor(src *ManifestToolDescriptor) ManifestToolDescriptor {
	cloned := *src
	cloned.Profile = strings.TrimSpace(src.Profile)
	cloned.Tool = src.Tool.Descriptor().Tool()
	cloned.RuntimeDescriptor.Capabilities = slices.Clone(src.RuntimeDescriptor.Capabilities)
	cloned.RuntimeDescriptor.Command = cloneExtensionCommandSpec(src.RuntimeDescriptor.Command)
	return cloned
}

func cloneExtensionManifestTools(src []extensionManifestTool) []extensionManifestTool {
	cloned := make([]extensionManifestTool, len(src))
	for i := range src {
		cloned[i] = extensionManifestTool{
			info:       cloneExtensionInfo(src[i].info),
			key:        src[i].key,
			descriptor: cloneManifestToolDescriptor(&src[i].descriptor),
		}
	}
	return cloned
}
