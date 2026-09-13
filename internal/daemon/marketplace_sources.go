package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
)

func newMarketplaceRuntime(
	ctx context.Context,
	store marketplace.Store,
	notifier marketplace.Notifier,
	cfg compozyconfig.MarketplaceRuntimeConfig,
	home compozyconfig.HomePaths,
	now func() time.Time,
) (*marketplaceRuntime, error) {
	if store == nil || home.HomeDir == "" {
		return nil, errors.New("daemon: marketplace store and home are required")
	}
	resolver := &pluginsource.Resolver{
		Cache: &pluginsource.PackageCache{Root: filepath.Join(home.HomeDir, "marketplace", "packages")},
	}
	runtime := &marketplaceRuntime{store: store, resolver: resolver,
		config:     compozyconfig.CloneConfig(&compozyconfig.Config{Marketplace: cfg}).Marketplace,
		presetPath: filepath.Join(home.HomeDir, "marketplace", "presets.json"),
	}
	presets, err := readMarketplacePresets(runtime.presetPath, cfg.Catalog.EffectiveBaseURL())
	if err != nil {
		return nil, err
	}
	runtime.presets = presets
	sources, ttl, timeout, err := runtime.buildBindings(cfg, presets)
	if err != nil {
		return nil, err
	}
	options := []marketplace.ServiceOption{marketplace.WithNotifier(notifier)}
	if now != nil {
		options = append(options, marketplace.WithNow(now))
	}
	service, err := marketplace.NewService(ctx, store, sources, ttl, timeout, options...)
	if err != nil {
		return nil, fmt.Errorf("daemon: create marketplace service: %w", err)
	}
	runtime.service = service
	return runtime, nil
}

// buildBindings composes domain readers with the install loader and one home-owned cache.
// Construction never contacts a source; the service owns enablement and refresh lifetime.
func (r *marketplaceRuntime) buildBindings(
	cfg compozyconfig.MarketplaceRuntimeConfig,
	presets *marketplace.PresetDocument,
) ([]marketplace.SourceBinding, time.Duration, time.Duration, error) {
	var entries []marketplace.Preset
	if presets != nil {
		entries = presets.Entries
	}
	resolved, err := marketplace.ResolveSources(cfg, entries)
	if err != nil {
		return nil, 0, 0, err
	}
	ttl, err := time.ParseDuration(cfg.Catalog.EffectiveTTL())
	if err != nil {
		return nil, 0, 0, fmt.Errorf("daemon: parse marketplace catalog TTL: %w", err)
	}
	timeout, err := time.ParseDuration(cfg.Catalog.EffectiveTimeout())
	if err != nil {
		return nil, 0, 0, fmt.Errorf("daemon: parse marketplace catalog timeout: %w", err)
	}
	feed, err := marketplace.NewSource(cfg.Catalog.EffectiveBaseURL(), &http.Client{Timeout: timeout})
	if err != nil {
		return nil, 0, 0, fmt.Errorf("daemon: create marketplace source: %w", err)
	}
	projector, err := marketplace.NewPluginProjector(r.resolver, extensionpkg.InspectPluginPackage)
	if err != nil {
		return nil, 0, 0, err
	}
	bindings := make([]marketplace.SourceBinding, 0, len(resolved))
	for _, source := range resolved {
		var fetcher marketplace.Source
		if source.Kind == marketplace.SourceKindFeed {
			source.Revision = cfg.Catalog.EffectiveBaseURL()
			fetcher = &marketplaceFeedSource{feed: feed, runtime: r, baseURL: cfg.Catalog.EffectiveBaseURL()}
		} else {
			fetcher, err = marketplace.NewPluginSource(source, &r.resolver.Sources, projector)
			if err != nil {
				return nil, 0, 0, err
			}
		}
		bindings = append(bindings, marketplace.SourceBinding{Config: source, Fetcher: fetcher})
	}
	return bindings, ttl, timeout, nil
}
