package remotehost

import (
	"net"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/remoteproto"
)

func (r *hostRun) hostedView() *hostedView {
	if hosted := r.hosted.Load(); hosted != nil {
		return hosted.view.Load()
	}
	return nil
}

func (r *hostRun) hostedEntitlement(view *hostedView) string {
	lease, current := r.sessions.Entitlement()
	if current {
		if !slices.Contains(lease.AllowedRoutes, remoteproto.RouteCloud) {
			return remoteproto.HostedTunnelEntitlementOnly
		}
		if view != nil && view.mode == remoteproto.HostedTunnelEntitlementOnly {
			return remoteproto.HostedTunnelEntitlementOnly
		}
		return remoteproto.HostedTunnelActive
	}
	if view != nil && view.mode == remoteproto.HostedTunnelEntitlementOnly {
		return remoteproto.HostedTunnelEntitlementOnly
	}
	return "unavailable"
}

func (h listenerHandler) guardHosted(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		view := h.run.hostedView()
		hostname := hostnameValue(view)
		if hostname == "" || !isHostedRequest(req.Host, hostname) {
			http.NotFound(w, req)
			return
		}
		w.Header().Set("Referrer-Policy", "no-referrer")
		if err := RejectClaimToken(req); err != nil {
			writeFailure(w, err)
			return
		}
		mode := h.run.hostedEntitlement(view)
		if req.Method == http.MethodGet && req.URL.Path == remoteproto.BrowserStatusPath {
			state := view.status.State
			if mode == remoteproto.HostedTunnelEntitlementOnly {
				state = planRequiredCode
			}
			writeJSON(w, http.StatusOK, remoteproto.BrowserStatus{State: state})
			return
		}
		if h.hostedApp != nil && hostedStaticRequest(req) {
			h.hostedApp.ServeHTTP(w, req)
			return
		}
		if mode == remoteproto.HostedTunnelEntitlementOnly {
			writeFailure(w, ErrPlanRequired)
			return
		}
		if err := OperatorRouteRejection(req.URL.Path, true); err != nil {
			writeFailure(w, err)
			return
		}
		if mode != remoteproto.HostedTunnelActive && req.URL.Path != DescriptorPath {
			writeFailure(w, ErrEntitlement)
			return
		}
		next.ServeHTTP(w, req)
	})
}

func hostedStaticRequest(req *http.Request) bool {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}
	root, _, _ := strings.Cut(strings.TrimPrefix(path.Clean(req.URL.Path), "/"), "/")
	switch root {
	case "api", "c", "remote", "ws":
		return false
	default:
		return true
	}
}

func isHostedRequest(authority, hostname string) bool {
	if host, port, err := net.SplitHostPort(authority); err == nil && port == "443" {
		authority = host
	}
	return strings.EqualFold(authority, hostname)
}
