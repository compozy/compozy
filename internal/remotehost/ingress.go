package remotehost

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/ingress"
	"github.com/compozy/compozy/internal/remoteproto"
)

type ingressBaseKey struct{}

func WebhookIngressBase(ctx context.Context) string {
	if base, ok := ctx.Value(ingressBaseKey{}).(string); ok {
		return base
	}
	return ""
}

func (h *Host) SetWebhookHandler(handler http.Handler) {
	h.webhook.Store(&handler)
}

func (r *hostRun) setIngressConfig(cfg config.RemoteConfig) {
	r.ingressConfig.Store(new(cfg))
}

func (r *hostRun) selectedIngress() ingress.RemoteEndpoint {
	cfg := r.ingressConfig.Load()
	if cfg == nil || r.unlinked.Load() {
		return ingress.RemoteEndpoint{}
	}
	public, err := ingress.AdvertisedBase("", cfg.Ingress.PublicURL)
	hasPublic := err == nil
	if !cfg.CloudRoute.Enabled {
		return ingress.RemoteEndpoint{Base: public, Managed: hasPublic, Reachable: hasPublic}
	}
	view := r.hostedView()
	mode := r.hostedEntitlement(view)
	if mode == remoteproto.HostedTunnelEntitlementOnly {
		return ingress.RemoteEndpoint{Base: public, Managed: true, Reachable: hasPublic}
	}
	if hostname := hostnameValue(view); hostname != "" {
		base, err := ingress.AdvertisedBase(hostname, "")
		ready := err == nil && view.status.State == remoteproto.HostedRouteReady &&
			mode == remoteproto.HostedTunnelActive
		return ingress.RemoteEndpoint{Base: base, Managed: true, Reachable: ready}
	}
	return ingress.RemoteEndpoint{Base: public, Managed: true, Pending: true}
}

func (r *hostRun) syncIngress(ctx context.Context) {
	if r.syncEndpoint == nil || ctx.Err() != nil {
		return
	}
	r.ingressMu.Lock()
	defer r.ingressMu.Unlock()
	endpoint := r.selectedIngress()
	if r.ingressSynced && r.ingressSelection == endpoint {
		return
	}
	if err := r.syncEndpoint(ctx, endpoint); err != nil {
		r.control.logger.WarnContext(ctx, "Could not update the advertised webhook endpoint", "error", err)
		return
	}
	r.ingressSynced, r.ingressSelection = true, endpoint
}

func (h listenerHandler) webhookRoutes() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		cfg := h.run.ingressConfig.Load()
		if cfg == nil {
			http.NotFound(w, req)
			return
		}
		var base string
		var err error
		switch h.kind {
		case ListenerHosted:
			base, err = ingress.AdvertisedBase(hostnameValue(h.run.hostedView()), "")
		case ListenerTunnel:
			base, err = ingress.AdvertisedBase("", cfg.Ingress.PublicURL)
		}
		if err != nil || base == "" || h.webhook == nil {
			http.NotFound(w, req)
			return
		}
		if err := RejectClaimToken(req); err != nil {
			writeFailure(w, err)
			return
		}
		handler := h.webhook()
		if handler == nil {
			writeFailure(w, ErrHostedUnavailable)
			return
		}
		handler.ServeHTTP(w, req.WithContext(context.WithValue(req.Context(), ingressBaseKey{}, base)))
	}
}
