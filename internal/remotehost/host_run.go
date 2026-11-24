package remotehost

import (
	"context"
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
	run := &hostRun{ctx: lifetime, cancel: cancel, authority: keys.ca}
	link := &controlLink{
		cloud: h.cloud, id: identity.ComputerID, key: keys.signing, verifier: verifier,
		logger: h.opts.Logger, outgoing: make(chan remoteproto.ControlMessage, 256),
		unlink: func(ctx context.Context) {
			keys.ca.retire()
			run.listeners.close()
			h.deleteKeys(ctx, identity)
			run.unlinked.Store(true)
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
	run.handler = h.newListenerHandler(identity, keys, sessions)
	publish := func(endpoints []string) error {
		if !syncCloud {
			return nil
		}
		return h.cloud.UpdateRemoteAccess(ctx, identity.ComputerID, keys.signing, true, endpoints)
	}
	if err := run.listeners.apply(run.ctx, cfg, run.handler, publish); err != nil {
		cancel()
		return err
	}
	h.run = run
	run.workers.Go(func() {
		link.run(run.ctx)
		if run.unlinked.Load() {
			run.cancel()
		}
	})
	run.workers.Go(func() { h.sweepSessions(run) })
	return nil
}

func (h *Host) newListenerHandler(identity Identity, keys hostKeys, sessions *Sessions) func(Listener) http.Handler {
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
			kind: kind, tls: tlsConfig, sessions: sessions, operator: h.opts.Operator,
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
