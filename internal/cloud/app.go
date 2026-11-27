// SPDX-License-Identifier: FSL-1.1-MIT

package cloud

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/compozy/compozy/internal/cloud/admin"
	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/auth"
	"github.com/compozy/compozy/internal/cloud/billing"
	"github.com/compozy/compozy/internal/cloud/cloudflare"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/email"
	"github.com/compozy/compozy/internal/cloud/observe"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/cloud/web"
	"github.com/compozy/compozy/internal/remoteproto"
)

type Config struct {
	Postgres        store.Config
	Keys            *api.SigningKeys
	Issuer          string
	Now             func() time.Time
	Logger          *slog.Logger
	Auth            *auth.Config
	MonthlyPrice    string
	AnnualPrice     string
	HostedTunnels   *cloudflare.Client
	Stripe          *billing.Stripe
	Email           *email.Resend
	UnsubscribeKey  []byte
	MetricsToken    string
	Publisher       *observe.Publisher
	Reporter        *observe.Sentry
	ExpectedRestore time.Time
	Revision        string
	Admin           *admin.AccessConfig
}

type App struct {
	Store           *store.Store
	Control         *control.Hub
	Keys            *api.SigningKeys
	AccountLimit    *api.AccountLimiter
	Issuer          string
	Auth            *auth.Service
	Billing         *billing.Service
	Accounts        *control.Service
	Metrics         *observe.Metrics
	publisher       *observe.Publisher
	reporter        *observe.Sentry
	mailer          *email.Resend
	monthly, annual string

	router   *api.Router
	tunnels  *control.Service
	draining atomic.Bool
	now      func() time.Time
	logger   *slog.Logger
}

var _ http.Handler = (*App)(nil)

func Open(ctx context.Context, cfg Config) (*App, error) {
	if cfg.Keys == nil {
		return nil, errors.New("cloud signing keys are required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	metrics := observe.NewMetrics()
	cfg.Postgres.Metrics = metrics
	db, err := store.Open(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	if err := db.CheckRestoreReady(ctx, cfg.ExpectedRestore); err != nil {
		db.Close()
		return nil, err
	}
	a := &App{
		Metrics: metrics, publisher: cfg.Publisher, reporter: cfg.Reporter, mailer: cfg.Email,
		monthly: cfg.MonthlyPrice, annual: cfg.AnnualPrice,
		Store:   db,
		Control: control.NewHub(),
		Keys:    cfg.Keys,
		Issuer:  cmp.Or(cfg.Issuer, remoteproto.CloudIssuer),
		now:     cfg.Now,
		logger:  cmp.Or(cfg.Logger, slog.Default()),
	}
	a.AccountLimit = api.NewAccountLimiter(a.now)
	a.router = api.NewRouter(db, a.Keys, a.Control.Ready, a.draining.Load, a.now)
	a.router.Observe(metrics, a.reporter)
	a.router.SetRevision(cfg.Revision)
	a.router.UseSigningKeySource(a.currentSigningKeys)
	domain := cloudflare.ProductionDomain
	if cfg.Auth != nil && cfg.Auth.HostedDomain != "" {
		domain = cfg.Auth.HostedDomain
	}
	a.Billing = billing.New(db, billing.Config{
		Metrics: metrics, Stripe: cfg.Stripe, Issuer: a.Issuer, HostedDomain: domain,
		MonthlyPrice: cfg.MonthlyPrice, AnnualPrice: cfg.AnnualPrice, Now: a.now,
	})
	if err := a.configureServices(ctx, cfg); err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

func (a *App) configureServices(ctx context.Context, cfg Config) error {
	var links *email.UnsubscribeLinks
	if cfg.Auth != nil {
		key := cfg.UnsubscribeKey
		if len(key) == 0 {
			key = cfg.Auth.EncryptionKey
		}
		configured, err := email.NewUnsubscribeLinks(a.Issuer, key)
		if err != nil {
			return err
		}
		links = configured
	}
	service := control.NewService(a.Store, control.ServiceConfig{
		Metrics: a.Metrics, Signer: a.Keys.Signer(), Issuer: a.Issuer, Now: a.now,
		MonthlyPrice: cfg.MonthlyPrice, AnnualPrice: cfg.AnnualPrice,
		Logger: a.logger, HostedTunnels: cfg.HostedTunnels, Billing: a.Billing,
		Email: cfg.Email, Unsubscribe: links,
	})
	a.Accounts = service
	if err := admin.Register(a.Routes(), a.Store, service, a.Billing, admin.Config{
		Access: cfg.Admin, Now: a.now, Logger: a.logger,
		Metrics: func(ctx context.Context) ([]observe.Sample, error) {
			return a.operationalSamples(ctx, a.now())
		},
	}); err != nil {
		return err
	}
	var authenticator api.DeviceAuthenticator
	if cfg.Auth != nil {
		if err := a.registerAuth(*cfg.Auth, links); err != nil {
			return err
		}
		authenticator = a.Auth
	}
	if _, err := a.currentSigningKeys(ctx); err != nil {
		return err
	}
	a.router.RegisterAccountMetrics(a.Store, cfg.MetricsToken)
	if cfg.HostedTunnels != nil {
		a.tunnels = service
	}
	a.router.RegisterControl(api.ControlRoutesConfig{
		Service: service, Hub: a.Control, Authenticator: authenticator,
		Limiter: a.AccountLimit, Issuer: a.Issuer, Logger: a.logger, Billing: a.Billing,
	})
	return nil
}

func (a *App) registerAuth(cfg auth.Config, links *email.UnsubscribeLinks) error {
	cfg.Metrics = a.Metrics
	service, err := auth.New(a.Store, cfg, a.Issuer, a.Keys.Signer(), a.Keys.JWKS(), a.AccountLimit.RetryAfter, a.now)
	if err != nil {
		return err
	}
	if err := web.Register(a.Routes(), service, a.Billing, a.AccountLimit, a.Accounts, links); err != nil {
		return err
	}
	api.RegisterAuthRoutes(a.Routes(), service)
	a.Auth = service
	return nil
}

func (a *App) Routes() *http.ServeMux {
	return a.router.Routes()
}

func (a *App) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.router.ServeHTTP(w, req)
}

func (a *App) RunBackground(ctx context.Context) {
	var group sync.WaitGroup
	if a.reporter != nil {
		group.Go(func() { a.reporter.Run(ctx, func() { a.logger.Warn("Cloud error export failed") }) })
	}
	if a.mailer != nil || a.publisher != nil {
		group.Go(func() { a.runOperations(ctx) })
	}
	group.Go(func() { a.Control.Listen(ctx, a.Store.ConnectDirect, a.logger) })
	group.Go(func() { a.runRetention(ctx) })
	group.Go(func() { a.runBillingReconciliation(ctx) })
	group.Go(func() { a.runComplimentaryReconciliation(ctx) })
	if a.tunnels != nil {
		group.Go(func() { a.runHostedTunnelSweep(ctx) })
	}
	if a.Auth != nil {
		group.Go(func() { a.runAuthCleanup(ctx) })
		group.Go(func() { a.runAccountLifecycle(ctx) })
	}
	group.Wait()
}

func (a *App) runBillingReconciliation(ctx context.Context) {
	ticker := time.NewTicker(billing.ReconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.Billing.Reconcile(ctx); err != nil && ctx.Err() == nil {
				a.logger.WarnContext(ctx, "Stripe subscription reconciliation failed")
				a.reporter.Capture(ctx, "stripe_reconciliation", observe.Client{})
			}
		}
	}
}

func (a *App) runComplimentaryReconciliation(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		work, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := a.Billing.ReconcileComplimentary(work)
		cancel()
		if err != nil && ctx.Err() == nil {
			a.logger.WarnContext(ctx, "Complimentary entitlement reconciliation failed")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) Close() {
	a.Store.Close()
}

func (a *App) StartDrain() <-chan struct{} {
	a.draining.Store(true)
	return a.Control.Drain()
}

func (a *App) runAuthCleanup(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			job, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := a.Auth.Sweep(job)
			cancel()
			if err != nil && ctx.Err() == nil {
				a.logger.Warn("Cloud auth cleanup failed")
			}
		}
	}
}

func (a *App) runRetention(ctx context.Context) {
	const interval = 24 * time.Hour
	for {
		job, cancel := context.WithTimeout(ctx, time.Minute)
		now := a.now()
		err := errors.Join(a.Store.CompactSecurityEvents(job, now), a.Store.CompactControlRecords(job, now),
			a.Store.SweepDeletedAccounts(job, now))
		cancel()
		if err != nil && ctx.Err() == nil {
			a.logger.Warn("Cloud security event retention failed", "error", err)
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func (a *App) runHostedTunnelSweep(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		work, cancel := context.WithTimeout(ctx, time.Minute)
		err := errors.Join(a.tunnels.SweepHostedTunnels(work), a.tunnels.LogHostedTunnelCapacity(work))
		cancel()
		if err != nil && ctx.Err() == nil {
			a.logger.WarnContext(ctx, "Hosted tunnel maintenance failed")
			a.reporter.Capture(ctx, "hosted_tunnel_maintenance", observe.Client{})
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (a *App) currentSigningKeys(ctx context.Context) (remoteproto.JWKS, error) {
	retired, err := a.Store.RetiredSigningKeys(ctx)
	if err != nil {
		return remoteproto.JWKS{}, err
	}
	keys, err := a.Keys.Filtered(retired)
	if err == nil && a.Auth != nil && len(keys.Retired) > 0 {
		err = a.Auth.ReplaceSigningKeys(keys)
	}
	return keys, err
}
