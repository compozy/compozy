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
	base, _ := ctx.Value(ingressBaseKey{}).(string)
	return base
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
	public, _ := ingress.AdvertisedBase("", cfg.Ingress.PublicURL)
	if !cfg.CloudRoute.Enabled {
		return ingress.RemoteEndpoint{Base: public, Managed: public != "", Reachable: public != ""}
	}
	view := r.hostedView()
	if r.hostedEntitlement(view) == remoteproto.HostedTunnelEntitlementOnly {
		return ingress.RemoteEndpoint{Base: public, Managed: true, Reachable: public != ""}
	}
	if hostname := hostnameValue(view); hostname != "" {
		base, _ := ingress.AdvertisedBase(hostname, "")
		return ingress.RemoteEndpoint{Base: base, Managed: true, Reachable: view.status.State == hostedStateReady &&
			r.hostedEntitlement(view) == remoteproto.HostedTunnelActive}
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
		switch h.kind {
		case ListenerHosted:
			base, _ = ingress.AdvertisedBase(hostnameValue(h.run.hostedView()), "")
		case ListenerTunnel:
			if cfg.Ingress.PublicURL == "" {
				http.NotFound(w, req)
				return
			}
			base, _ = ingress.AdvertisedBase("", cfg.Ingress.PublicURL)
		}
		if base == "" || h.webhook == nil {
			http.NotFound(w, req)
			return
		}
		if len(req.Header.Values("X-Compozy-Claim-Token")) != 0 {
			writeFailure(w, &Failure{http.StatusBadRequest, "claim_token_not_remote",
				"Task leases stay on the computer that owns them.", nil})
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
