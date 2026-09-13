package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/marketplace/pluginsource"
	"github.com/compozy/compozy/internal/store"
)

const defaultMarketplaceEventWriteTimeout = 5 * time.Second

type marketplaceRuntime struct {
	mu         sync.RWMutex
	store      marketplace.Store
	service    *marketplace.CatalogService
	resolver   *pluginsource.Resolver
	config     compozyconfig.MarketplaceRuntimeConfig
	presets    *marketplace.PresetDocument
	presetPath string
	homePaths  compozyconfig.HomePaths
	stopped    bool
}

var _ marketplace.Service = (*marketplaceRuntime)(nil)

func (r *marketplaceRuntime) Browse(
	ctx context.Context,
	query string,
	offset int,
	limit int,
) (marketplace.BrowseResult, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return marketplace.BrowseResult{}, err
	}
	return service.Browse(ctx, query, offset, limit)
}

func (r *marketplaceRuntime) Detail(
	ctx context.Context,
	source, entryID string,
) (*marketplace.Entry, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return nil, err
	}
	return service.Detail(ctx, source, entryID)
}

func (r *marketplaceRuntime) Entry(ctx context.Context, origin marketplace.Origin) (*marketplace.Entry, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return nil, err
	}
	return service.Entry(ctx, origin)
}

func (r *marketplaceRuntime) ResolveExtensionInstall(
	ctx context.Context,
	installSlug string,
	version string,
) (*marketplace.Entry, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return nil, err
	}
	return service.ResolveExtensionInstall(ctx, installSlug, version)
}

func (r *marketplaceRuntime) Refresh(
	ctx context.Context, names ...string,
) (marketplace.RefreshReport, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return marketplace.RefreshReport{}, err
	}
	report, refreshErr := service.Refresh(ctx, names...)
	if len(names) > 0 || ctx.Err() != nil {
		return report, refreshErr
	}
	states, err := service.Status(ctx)
	if err != nil {
		return report, errors.Join(refreshErr, err)
	}
	outcomes := make(map[string]marketplace.RefreshOutcome, len(report.Outcomes))
	for _, outcome := range report.Outcomes {
		outcomes[outcome.Source] = outcome
	}
	var added []string
	for _, state := range states {
		if _, refreshed := outcomes[state.Source]; state.Enabled && !refreshed {
			added = append(added, state.Source)
		}
	}
	if len(added) > 0 {
		more, err := service.Refresh(ctx, added...)
		refreshErr = errors.Join(refreshErr, err)
		for _, outcome := range more.Outcomes {
			outcomes[outcome.Source] = outcome
		}
	}
	report.Outcomes = report.Outcomes[:0]
	for _, state := range states {
		if outcome, refreshed := outcomes[state.Source]; refreshed {
			report.Outcomes = append(report.Outcomes, outcome)
		}
	}
	return report, refreshErr
}

func (r *marketplaceRuntime) Status(ctx context.Context) ([]marketplace.SourceState, error) {
	service, err := r.currentService(ctx)
	if err != nil {
		return nil, err
	}
	return service.Status(ctx)
}

func (r *marketplaceRuntime) ReconcileConfig(ctx context.Context, cfg *compozyconfig.Config) error {
	if ctx == nil {
		return errors.New("daemon: marketplace config reconciliation context is required")
	}
	if r == nil || r.store == nil {
		return errors.New("daemon: marketplace runtime is unavailable")
	}
	if cfg == nil {
		return errors.New("daemon: marketplace config is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("daemon: marketplace config reconciliation canceled: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return errors.New("daemon: marketplace runtime is stopped")
	}
	presets := r.presets
	if r.config.Catalog.EffectiveBaseURL() != cfg.Marketplace.Catalog.EffectiveBaseURL() {
		var err error
		presets, err = readMarketplacePresets(r.presetPath, cfg.Marketplace.Catalog.EffectiveBaseURL())
		if err != nil {
			return err
		}
	}
	sources, ttl, timeout, err := r.buildBindings(cfg.Marketplace, presets)
	if err != nil {
		return err
	}
	if err := r.service.Reconfigure(ctx, sources, ttl, timeout); err != nil {
		return err
	}
	r.config = compozyconfig.CloneConfig(cfg).Marketplace
	r.presets = presets
	return nil
}

func (r *marketplaceRuntime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return nil
	}
	r.stopped = true
	service := r.service
	r.service = nil
	r.mu.Unlock()
	if service == nil {
		return nil
	}
	return service.Close(ctx)
}

func (r *marketplaceRuntime) Close(ctx context.Context) error { return r.Shutdown(ctx) }

func (r *marketplaceRuntime) currentService(ctx context.Context) (marketplace.Service, error) {
	if ctx == nil {
		return nil, errors.New("daemon: marketplace context is required")
	}
	if r == nil {
		return nil, errors.New("daemon: marketplace service is unavailable")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.stopped || r.service == nil {
		return nil, errors.New("daemon: marketplace service is unavailable")
	}
	return r.service, nil
}

type marketplaceEventWriter interface {
	WriteEventSummary(context.Context, store.EventSummary) error
}

type daemonMarketplaceNotifier struct {
	writer       marketplaceEventWriter
	logger       *slog.Logger
	now          func() time.Time
	writeTimeout time.Duration
}

var _ marketplace.Notifier = (*daemonMarketplaceNotifier)(nil)

func (n *daemonMarketplaceNotifier) NotifyCatalogRefresh(
	ctx context.Context,
	outcome marketplace.RefreshOutcome,
) error {
	if n == nil || n.writer == nil {
		return nil
	}
	content, err := json.Marshal(outcome)
	if err != nil {
		n.logFailure("catalog.refresh", "marshal", err)
		return fmt.Errorf("daemon: marshal marketplace catalog refresh event: %w", err)
	}
	eventOutcome := eventspkg.OutcomeInfo
	switch outcome.Outcome {
	case marketplace.RefreshOutcomeSucceeded:
		eventOutcome = eventspkg.OutcomeSuccess
	case marketplace.RefreshOutcomeFailed:
		eventOutcome = eventspkg.OutcomeFailure
	}
	return n.writeEvent(ctx, "catalog.refresh", daemonEventSummary(store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventspkg.MarketplaceCatalogRefresh,
		Outcome:   string(eventOutcome),
		Summary:   fmt.Sprintf("marketplace catalog %s refresh %s", outcome.Source, outcome.Outcome),
	}, content))
}

func (n *daemonMarketplaceNotifier) NotifyInstall(ctx context.Context, outcome marketplace.InstallOutcome) error {
	if n == nil || n.writer == nil {
		return nil
	}
	content, err := json.Marshal(outcome)
	if err != nil {
		n.logFailure("install", "marshal", err)
		return fmt.Errorf("daemon: marshal marketplace install event: %w", err)
	}
	eventOutcome := eventspkg.OutcomeInfo
	switch outcome.Outcome {
	case marketplace.InstallOutcomeSucceeded:
		eventOutcome = eventspkg.OutcomeSuccess
	case marketplace.InstallOutcomeFailed:
		eventOutcome = eventspkg.OutcomeFailure
	}
	return n.writeEvent(ctx, "install", daemonEventSummary(store.EventSummary{
		ProfileID: store.DefaultProfileID,
		Type:      eventspkg.MarketplaceInstall,
		Outcome:   string(eventOutcome),
		Summary:   fmt.Sprintf("marketplace extension install %s", outcome.Outcome),
	}, content))
}

func (n *daemonMarketplaceNotifier) writeEvent(
	ctx context.Context,
	event string,
	summary store.EventSummary,
) error {
	if n == nil || n.writer == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("daemon: marketplace event context is required")
	}
	if n.now != nil {
		summary.Timestamp = n.now().UTC()
	} else {
		summary.Timestamp = time.Now().UTC()
	}
	writeTimeout := n.writeTimeout
	if writeTimeout <= 0 {
		writeTimeout = defaultMarketplaceEventWriteTimeout
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), writeTimeout)
	defer cancel()
	if err := n.writer.WriteEventSummary(writeCtx, summary); err != nil {
		n.logFailure(event, "write", err)
		return fmt.Errorf("daemon: persist marketplace %s event: %w", event, err)
	}
	return nil
}

func (n *daemonMarketplaceNotifier) logFailure(event string, operation string, err error) {
	if n == nil || n.logger == nil || err == nil {
		return
	}
	n.logger.Warn(
		"daemon.marketplace.event_failed",
		"event", strings.TrimSpace(event),
		"operation", strings.TrimSpace(operation),
		"error", diagnostics.RedactAndBound(err.Error(), 1024),
	)
}

func (d *Daemon) bootMarketplace(ctx context.Context, state *bootState, cleanup *bootCleanup) error {
	if ctx == nil {
		return errors.New("daemon: marketplace boot context is required")
	}
	if state == nil {
		return errors.New("daemon: marketplace boot state is required")
	}
	marketplaceRepository, ok := state.registry.(store.MarketplaceCatalogRepository)
	if !ok {
		if state.logger != nil {
			state.logger.Warn("daemon.marketplace.disabled", "reason", "registry_missing_marketplace_repository")
		}
		return nil
	}
	marketplaceStore, err := marketplace.NewSQLiteStore(marketplaceRepository)
	if err != nil {
		return fmt.Errorf("daemon: create marketplace store: %w", err)
	}
	notifier := &daemonMarketplaceNotifier{writer: state.registry, logger: state.logger, now: d.now}
	options := []marketplace.ServiceOption{marketplace.WithLogger(state.logger)}
	if dbSource, ok := state.registry.(extensionDBSource); ok && dbSource.DB() != nil {
		registry := extensionpkg.NewRegistry(dbSource.DB())
		options = append(
			options,
			marketplace.WithInstalledPackages(func(ctx context.Context) ([]marketplace.InstalledPackage, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				installed, err := registry.List()
				if err != nil {
					return nil, err
				}
				packages := make([]marketplace.InstalledPackage, 0, len(installed))
				for index := range installed {
					item := &installed[index]
					packages = append(packages, marketplace.InstalledPackage{
						Name: item.Name, SourceName: item.Provenance.SourceName,
						SourceRef: item.Provenance.SourceRef, DigestSHA256: item.Provenance.ArchiveDigestSHA256,
					})
				}
				return packages, nil
			}),
		)
	}
	runtime, err := newMarketplaceRuntime(
		ctx,
		marketplaceStore,
		notifier,
		state.cfg.Marketplace,
		d.homePaths,
		d.now,
		options...)
	if err != nil {
		return err
	}
	state.marketplace = runtime
	state.marketplaceNotifier = notifier
	if cleanup != nil {
		cleanup.add(runtime.Shutdown)
	}
	return nil
}
