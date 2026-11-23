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

	"github.com/compozy/compozy/internal/cloud/api"
	"github.com/compozy/compozy/internal/cloud/control"
	"github.com/compozy/compozy/internal/cloud/store"
	"github.com/compozy/compozy/internal/remoteproto"
)

type Config struct {
	Postgres       store.Config
	Keys           *api.SigningKeys
	Issuer         string
	Now            func() time.Time
	Logger         *slog.Logger
	DeviceSessions api.DeviceSessionVerifier
	MonthlyPrice   string
	AnnualPrice    string
}

type App struct {
	Store        *store.Store
	Control      *control.Hub
	Keys         *api.SigningKeys
	AccountLimit *api.AccountLimiter
	Issuer       string

	router   *api.Router
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
	db, err := store.Open(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	a := &App{
		Store:   db,
		Control: control.NewHub(),
		Keys:    cfg.Keys,
		Issuer:  cmp.Or(cfg.Issuer, remoteproto.CloudIssuer),
		now:     cfg.Now,
		logger:  cmp.Or(cfg.Logger, slog.Default()),
	}
	a.AccountLimit = api.NewAccountLimiter(a.now)
	a.router = api.NewRouter(db, a.Keys, a.Control.Ready, a.draining.Load, a.now)
	service := control.NewService(db, control.ServiceConfig{
		Signer: a.Keys.Signer(), Issuer: a.Issuer, Now: a.now,
		MonthlyPrice: cfg.MonthlyPrice, AnnualPrice: cfg.AnnualPrice,
	})
	a.router.RegisterControl(api.ControlRoutesConfig{
		Service: service, Hub: a.Control, Sessions: cfg.DeviceSessions,
		Limiter: a.AccountLimit, Issuer: a.Issuer, Logger: a.logger,
	})
	return a, nil
}

func (a *App) Routes() *http.ServeMux {
	return a.router.Routes()
}

func (a *App) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	a.router.ServeHTTP(w, req)
}

func (a *App) RunBackground(ctx context.Context) {
	var group sync.WaitGroup
	group.Go(func() { a.Control.Listen(ctx, a.Store.ConnectDirect, a.logger) })
	group.Go(func() { a.runRetention(ctx) })
	group.Wait()
}

func (a *App) Close() {
	a.Store.Close()
}

func (a *App) StartDrain() <-chan struct{} {
	a.draining.Store(true)
	return a.Control.Drain()
}

func (a *App) runRetention(ctx context.Context) {
	const interval = 24 * time.Hour
	for {
		job, cancel := context.WithTimeout(ctx, time.Minute)
		now := a.now()
		err := errors.Join(a.Store.CompactSecurityEvents(job, now), a.Store.CompactControlRecords(job, now))
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
