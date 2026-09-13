package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

const marketplaceConfigSection = "marketplace"
const marketplaceSourceEnabledKey = "enabled"
const marketplacePluginSourcesKey = "plugin_sources"

func (r *marketplaceRuntime) AddSource(
	ctx context.Context,
	ref, name string,
	dryRun bool,
) (marketplace.SourceState, error) {
	ref, err := pluginsource.NormalizeRef(ref)
	if err != nil {
		return marketplace.SourceState{}, err
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		name = marketplaceSourceDefaultName(ref)
	}
	if err := pluginsource.ValidateName(name); err != nil {
		return marketplace.SourceState{}, err
	}
	source := compozyconfig.MarketplacePluginSourceConfig{Name: name, Source: ref, Enabled: new(true)}
	if err := r.validateNewSource(ctx, source); err != nil {
		return marketplace.SourceState{}, err
	}
	if dryRun {
		return r.previewSource(ctx, source)
	}
	// Validate the document before registration; the normal refresh owns acquisition and diagnostics.
	if _, err := r.resolver.Sources.Fetch(ctx, ref); err != nil {
		return marketplace.SourceState{}, err
	}
	r.mu.Lock()
	err = r.addSourceLocked(ctx, source)
	r.mu.Unlock()
	if err != nil {
		return marketplace.SourceState{}, err
	}
	return r.RefreshSource(ctx, name)
}

func (r *marketplaceRuntime) validateNewSource(
	ctx context.Context,
	source compozyconfig.MarketplacePluginSourceConfig,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, err := r.newSourceConfig(ctx, source)
	return err
}

func (r *marketplaceRuntime) newSourceConfig(
	ctx context.Context,
	source compozyconfig.MarketplacePluginSourceConfig,
) (compozyconfig.MarketplaceRuntimeConfig, error) {
	if r.stopped {
		return compozyconfig.MarketplaceRuntimeConfig{}, marketplace.ErrServiceClosed
	}
	states, err := r.service.Status(ctx)
	if err != nil {
		return compozyconfig.MarketplaceRuntimeConfig{}, err
	}
	for _, state := range states {
		if state.Source == source.Name {
			return compozyconfig.MarketplaceRuntimeConfig{}, &marketplace.SourceExistsError{
				Name: source.Name, SuggestedName: marketplaceSourceSuggestedName(source.Name, states),
			}
		}
	}
	cfg := compozyconfig.CloneConfig(&compozyconfig.Config{Marketplace: r.config}).Marketplace
	cfg.PluginSources = append(cfg.PluginSources, source)
	bindings, _, _, err := r.buildBindings(cfg, r.presets)
	if err == nil {
		err = r.service.ValidateSources(ctx, bindings)
	}
	return cfg, err
}

func (r *marketplaceRuntime) addSourceLocked(
	ctx context.Context,
	source compozyconfig.MarketplacePluginSourceConfig,
) error {
	cfg, err := r.newSourceConfig(ctx, source)
	if err != nil {
		return err
	}
	return r.persistSourceConfig(ctx, cfg, func(editor *compozyconfig.OverlayEditor) error {
		return editor.UpsertArrayTableItem(
			[]string{marketplaceConfigSection, marketplacePluginSourcesKey},
			"name",
			source.Name,
			map[string]any{"source": source.Source, marketplaceSourceEnabledKey: true},
		)
	})
}

func (r *marketplaceRuntime) previewSource(
	ctx context.Context,
	source compozyconfig.MarketplacePluginSourceConfig,
) (marketplace.SourceState, error) {
	defer r.resolver.Cache.Hold()()
	projector, err := marketplace.NewPluginProjector(r.resolver, extensionpkg.InspectPluginPackage)
	if err != nil {
		return marketplace.SourceState{}, err
	}
	reader, err := marketplace.NewPluginSource(marketplace.ResolvedSource{
		Name: source.Name, Ref: source.Source, Kind: marketplace.SourceKindCustom, Enabled: true,
	}, &r.resolver.Sources, projector)
	if err != nil {
		return marketplace.SourceState{}, err
	}
	doc, err := reader.Fetch(ctx)
	if err != nil {
		return marketplace.SourceState{}, err
	}
	state := marketplace.SourceState{
		Source: source.Name, SourceRef: source.Source, Kind: marketplace.SourceKindCustom, Enabled: true,
		EntryCount: len(doc.Entries), DocumentPath: doc.DocumentPath, Owner: doc.Owner,
		Diagnostics: doc.Diagnostics, FetchedAt: time.Now().UTC(),
	}
	for _, entry := range doc.Entries {
		if entry.Installable {
			state.Installable++
		}
	}
	return state, nil
}

func (r *marketplaceRuntime) UpdateSource(
	ctx context.Context,
	name string,
	enabled bool,
) (marketplace.SourceState, error) {
	r.mu.Lock()
	err := r.updateSourceLocked(ctx, name, enabled)
	r.mu.Unlock()
	if err != nil {
		return marketplace.SourceState{}, err
	}
	if enabled {
		return r.RefreshSource(ctx, name)
	}
	return r.sourceStatus(ctx, name)
}

func (r *marketplaceRuntime) updateSourceLocked(ctx context.Context, name string, enabled bool) error {
	state, err := r.sourceStatusLocked(ctx, name)
	if err != nil {
		return err
	}
	if state.Kind == marketplace.SourceKindFeed {
		return marketplace.ErrSourcePresetReadonly
	}
	cfg := compozyconfig.CloneConfig(&compozyconfig.Config{Marketplace: r.config}).Marketplace
	// Preset overrides follow the ref even when upstream renamed the source.
	key := name
	index := slices.IndexFunc(
		cfg.PluginSources,
		func(item compozyconfig.MarketplacePluginSourceConfig) bool { return item.Name == name },
	)
	if index < 0 && state.Kind == marketplace.SourceKindPreset {
		index = slices.IndexFunc(
			cfg.PluginSources,
			func(item compozyconfig.MarketplacePluginSourceConfig) bool { return item.Source == state.SourceRef },
		)
	}
	if index >= 0 {
		key = cfg.PluginSources[index].Name
		cfg.PluginSources[index].Enabled = new(enabled)
	} else {
		cfg.PluginSources = append(
			cfg.PluginSources,
			compozyconfig.MarketplacePluginSourceConfig{Name: name, Source: state.SourceRef, Enabled: new(enabled)},
		)
	}
	return r.persistSourceConfig(ctx, cfg, func(editor *compozyconfig.OverlayEditor) error {
		return editor.UpsertArrayTableItem([]string{marketplaceConfigSection, marketplacePluginSourcesKey}, "name", key,
			map[string]any{"source": state.SourceRef, marketplaceSourceEnabledKey: enabled})
	})
}

func (r *marketplaceRuntime) RemoveSource(ctx context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.sourceStatusLocked(ctx, name)
	if err != nil {
		return err
	}
	if state.Kind != marketplace.SourceKindCustom {
		return marketplace.ErrSourcePresetReadonly
	}
	cfg := compozyconfig.CloneConfig(&compozyconfig.Config{Marketplace: r.config}).Marketplace
	cfg.PluginSources = slices.DeleteFunc(
		cfg.PluginSources,
		func(item compozyconfig.MarketplacePluginSourceConfig) bool { return item.Name == name },
	)
	return r.persistSourceConfig(ctx, cfg, func(editor *compozyconfig.OverlayEditor) error {
		_, err := editor.DeleteArrayTableItem(
			[]string{marketplaceConfigSection, marketplacePluginSourcesKey},
			"name",
			name,
		)
		return err
	})
}

// persistSourceConfig runs under the runtime lock and uses the canonical comment-preserving writer.
func (r *marketplaceRuntime) persistSourceConfig(
	ctx context.Context,
	cfg compozyconfig.MarketplaceRuntimeConfig,
	mutate func(*compozyconfig.OverlayEditor) error,
) error {
	bindings, _, _, err := r.buildBindings(cfg, r.presets)
	if err != nil {
		return err
	}
	if err := r.service.ValidateSources(ctx, bindings); err != nil {
		return err
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(r.homePaths, "", compozyconfig.WriteScopeUser, "default")
	if err != nil {
		return err
	}
	effective, err := compozyconfig.EditConfigOverlay(r.homePaths, "", target, mutate)
	if err != nil {
		return err
	}
	bindings, ttl, timeout, err := r.buildBindings(effective.Marketplace, r.presets)
	if err != nil {
		return err
	}
	if err := r.service.Reconfigure(ctx, bindings, ttl, timeout); err != nil {
		return err
	}
	r.config = effective.Marketplace
	return nil
}

func (r *marketplaceRuntime) RefreshSource(ctx context.Context, name string) (marketplace.SourceState, error) {
	state, err := r.sourceStatus(ctx, name)
	if err != nil {
		return marketplace.SourceState{}, err
	}
	if !state.Enabled {
		return state, nil
	}
	_, refreshErr := r.Refresh(ctx, name)
	state, err = r.sourceStatus(ctx, name)
	if err != nil {
		return marketplace.SourceState{}, errors.Join(err, refreshErr)
	}
	if ctx.Err() != nil {
		return marketplace.SourceState{}, ctx.Err()
	}
	// A failed refresh is represented by its durable degraded state.
	if refreshErr != nil && state.LastError == "" {
		return state, refreshErr
	}
	return state, nil
}

func (r *marketplaceRuntime) sourceStatus(ctx context.Context, name string) (marketplace.SourceState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sourceStatusLocked(ctx, name)
}

func (r *marketplaceRuntime) sourceStatusLocked(ctx context.Context, name string) (marketplace.SourceState, error) {
	if r.stopped {
		return marketplace.SourceState{}, marketplace.ErrServiceClosed
	}
	states, err := r.service.Status(ctx)
	if err != nil {
		return marketplace.SourceState{}, err
	}
	for _, state := range states {
		if state.Source == name {
			return state, nil
		}
	}
	return marketplace.SourceState{}, fmt.Errorf("%w: %s", marketplace.ErrSourceNotFound, name)
}

func marketplaceSourceDefaultName(ref string) string {
	if repo, ok := strings.CutPrefix(ref, "github:"); ok {
		return path.Base(repo)
	}
	parsed, err := url.Parse(strings.TrimPrefix(ref, "git+"))
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSuffix(path.Base(parsed.Path), ".git"))
}

func marketplaceSourceSuggestedName(name string, states []marketplace.SourceState) string {
	used := make(map[string]bool, len(states))
	for _, state := range states {
		used[state.Source] = true
	}
	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%.55s-%d", name, suffix)
		if !used[candidate] {
			return candidate
		}
	}
}
