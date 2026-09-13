package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/store"
)

const defaultMarketplaceEventWriteTimeout = 5 * time.Second

type marketplaceRuntime struct {
	mu      sync.RWMutex
	store   marketplace.Store
	service *marketplace.CatalogService
	stopped bool
}

var _ marketplace.Service = (*marketplaceRuntime)(nil)

func newMarketplaceRuntime(
	ctx context.Context,
	store marketplace.Store,
	notifier marketplace.Notifier,
	cfg compozyconfig.MarketplaceCatalogConfig,
	now func() time.Time,
) (*marketplaceRuntime, error) {
	if store == nil {
		return nil, errors.New("daemon: marketplace store is required")
	}
	service, err := buildMarketplaceService(ctx, store, notifier, cfg, now)
	if err != nil {
		return nil, err
	}
	return &marketplaceRuntime{
		store:   store,
		service: service,
	}, nil
}

func buildMarketplaceService(
	ctx context.Context,
	marketplaceStore marketplace.Store,
	notifier marketplace.Notifier,
	cfg compozyconfig.MarketplaceCatalogConfig,
	now func() time.Time,
) (*marketplace.CatalogService, error) {
	sources, ttl, timeout, err := buildMarketplaceBindings(cfg)
	if err != nil {
		return nil, err
	}
	options := []marketplace.ServiceOption{marketplace.WithNotifier(notifier)}
	if now != nil {
		options = append(options, marketplace.WithNow(now))
	}
	service, err := marketplace.NewService(ctx, marketplaceStore, sources, ttl, timeout, options...)
	if err != nil {
		return nil, fmt.Errorf("daemon: create marketplace service: %w", err)
	}
	return service, nil
}

func buildMarketplaceBindings(
	cfg compozyconfig.MarketplaceCatalogConfig,
) ([]marketplace.SourceBinding, time.Duration, time.Duration, error) {
	if err := cfg.Validate("marketplace.catalog"); err != nil {
		return nil, 0, 0, err
	}
	ttl, err := time.ParseDuration(cfg.EffectiveTTL())
	if err != nil {
		return nil, 0, 0, fmt.Errorf("daemon: parse marketplace catalog TTL: %w", err)
	}
	timeout, err := time.ParseDuration(cfg.EffectiveTimeout())
	if err != nil {
		return nil, 0, 0, fmt.Errorf("daemon: parse marketplace catalog timeout: %w", err)
	}
	client := &http.Client{Timeout: timeout}
	source, err := marketplace.NewSource(cfg.EffectiveBaseURL(), client)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("daemon: create marketplace source: %w", err)
	}

	return []marketplace.SourceBinding{{Config: marketplace.ResolvedSource{
		Revision: cfg.EffectiveBaseURL(),
		Name:     marketplace.CompozyCatalogSource,
		Ref:      marketplace.CompozyCatalogRef,
		Kind:     marketplace.SourceKindFeed,
		Enabled:  true,
	}, Fetcher: source}}, ttl, timeout, nil
}

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
	return service.Refresh(ctx, names...)
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
	sources, ttl, timeout, err := buildMarketplaceBindings(cfg.Marketplace.Catalog)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return errors.New("daemon: marketplace runtime is stopped")
	}
	return r.service.Reconfigure(ctx, sources, ttl, timeout)
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
	runtime, err := newMarketplaceRuntime(ctx, marketplaceStore, notifier, state.cfg.Marketplace.Catalog, d.now)
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
