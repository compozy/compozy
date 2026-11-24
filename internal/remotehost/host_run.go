package remotehost

import (
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/remoteproto"
)

func (h *Host) start(ctx context.Context, cfg config.RemoteConfig, syncCloud bool) error {
	identity := h.identity
	keys, err := loadHostKeys(h.account, identity, h.opts.Now())
	if err != nil {
		return err
	}
	verifier, err := remoteproto.NewJWSVerifier(h.cloud.Origin(), remoteproto.JWKS{}, h.cloud.JWKS)
	if err != nil {
		return err
	}
	// #nosec G118 -- the run owns cancel until its listeners and workers have joined.
	lifetime, cancel := context.WithCancel(h.ctx)
	run := &hostRun{ctx: lifetime, cancel: cancel, authority: keys.ca, syncEndpoint: h.opts.SyncIngress}
	run.setIngressConfig(cfg)
	link := &controlLink{
		cloud: h.cloud, id: identity.ComputerID, key: keys.signing, verifier: verifier,
		logger: h.opts.Logger, outgoing: make(chan remoteproto.ControlMessage, 256),
		unlink: func(ctx context.Context) {
			keys.ca.retire()
			run.unlinked.Store(true)
			run.listeners.close()
			run.syncIngress(ctx)
			if err := h.account.ObserveHostLink(ctx, identity.AccountID, reasonComputerUnlinked); err != nil {
				h.opts.Logger.WarnContext(ctx, "Could not reconcile the unlinked installation", "error", err)
			}
			h.deleteKeys(ctx, identity)
			h.wakeRevalidation()
		},
	}
	sessions, err := NewSessions(ctx, h.store, identity, verifier, SessionOptions{
		Now: h.opts.Now, RequireApproval: cfg.RequireDeviceApproval, MaxScopes: cfg.MaxScopes, Report: link.report,
	})
	if err != nil {
		cancel()
		return err
	}
	link.sessions = sessions
	run.sessions, run.control = sessions, link
	link.onLease = func(ctx context.Context) {
		run.syncIngress(ctx)
		if hosted := run.hosted.Load(); hosted != nil {
			select {
			case hosted.refresh <- struct{}{}:
			default:
			}
		}
	}
	run.handler = h.newListenerHandler(identity, keys, run)
	var hosted string
	var initial remoteproto.HostedTunnel
	publish := func(endpoints []string, bound string) error {
		hosted = bound
		if !syncCloud {
			return nil
		}
		if err := h.cloud.UpdateRemoteAccess(ctx, identity.ComputerID, keys.signing, true, endpoints); err != nil {
			return err
		}
		if cfg.CloudRoute.Enabled && !h.config.CloudRoute.Enabled {
			var err error
			initial, err = h.provisionHosted(ctx, run, bound)
			if err != nil {
				return errors.Join(err, h.cloud.UpdateRemoteAccess(ctx, identity.ComputerID, keys.signing, false, nil))
			}
		}
		return nil
	}
	if err := run.listeners.apply(run.ctx, cfg, run.handler, publish); err != nil {
		cancel()
		return err
	}
	h.run = run
	h.replaceHostedRoute(run, hosted, initial)
	run.syncIngress(ctx)
	run.workers.Go(func() {
		link.run(run.ctx)
		if run.unlinked.Load() {
			run.cancel()
		}
	})
	run.workers.Go(func() { h.sweepSessions(run) })
	return nil
}

func (h *Host) newListenerHandler(identity Identity, keys hostKeys, run *hostRun) func(Listener) http.Handler {
	tickets := newStreamTicketStore(DefaultStreamTicketLimit, DefaultStreamTicketTTL, h.opts.Now, rand.Reader)
	var redeem func(context.Context, remoteproto.BrowserTokenRequest) (remoteproto.GrantResponse, error)
	if cloud, ok := h.cloud.(browserCloud); ok {
		redeem = func(ctx context.Context, input remoteproto.BrowserTokenRequest) (remoteproto.GrantResponse, error) {
			return cloud.RedeemBrowserCode(ctx, keys.signing, input)
		}
	}
	var leafMu sync.Mutex
	savedExpiry := identity.TLSLeafNotAfter
	tlsConfig := keys.ca.serverTLS(h.opts.Now, func(ctx context.Context, expiry time.Time) error {
		leafMu.Lock()
		defer leafMu.Unlock()
		if savedExpiry.Equal(expiry) {
			return nil
		}
		if err := h.store.UpdateRemoteLeaf(ctx, identity.ComputerID, expiry); err != nil {
			return err
		}
		savedExpiry = expiry
		return nil
	})
	return func(kind Listener) http.Handler {
		handler := listenerHandler{
			descriptor: remoteproto.Descriptor{
				ComputerID: identity.ComputerID, Name: h.name, Version: h.version, Platform: h.platform,
				Protocol: remoteproto.CurrentProtocol, TLSRootSHA256: identity.TLSRootSHA256,
				BrowserSignIn: kind == ListenerHosted,
			},
			kind: kind, tls: tlsConfig, sessions: run.sessions, operator: h.opts.Operator, run: run,
			hostedApp: h.opts.HostedApp, redeemBrowserCode: redeem, browserTickets: tickets,
			webhook: func() http.Handler {
				if handler := h.webhook.Load(); handler != nil {
					return *handler
				}
				return nil
			},
			wrapTunnelHandler: h.opts.WrapTunnelHandler,
		}
		return handler.outer()
	}
}

func (h *Host) sweepSessions(run *hostRun) {
	expiry := time.NewTicker(250 * time.Millisecond)
	defer expiry.Stop()
	sweep := time.NewTicker(time.Minute)
	defer sweep.Stop()
	for {
		select {
		case <-run.ctx.Done():
			return
		case <-expiry.C:
			run.syncIngress(run.ctx)
			if err := run.sessions.Expire(run.ctx); err != nil {
				h.opts.Logger.Warn("Remote session expiry failed", "error", err)
			}
		case <-sweep.C:
			if !run.unlinked.Load() {
				if err := run.sessions.Sweep(run.ctx); err != nil {
					h.opts.Logger.Warn("Remote session sweep failed", "error", err)
				}
			}
		}
	}
}

func (r *hostRun) close() {
	r.sessions.Stop()
	r.cancel()
	r.stopHostedRoute()
	r.listeners.close()
	r.workers.Wait()
}

func (h *Host) deleteKeys(ctx context.Context, identity Identity) {
	for _, ref := range []string{identity.ComputerKeyRef, identity.TLSRootRef, linkDigestRef} {
		if err := h.account.DeleteHostSecret(ref); err != nil {
			h.opts.Logger.WarnContext(ctx, "Could not delete a remote computer key", "computer_id", identity.ComputerID)
		}
	}
}
