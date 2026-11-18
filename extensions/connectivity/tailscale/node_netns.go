package tailscale

import (
	"log/slog"
	"os"
	"strings"

	"tailscale.com/net/netns"
)

// useHostRouting keeps the embedded node on the host's own routing decision
// instead of letting Tailscale's logical network namespace pin sockets to the
// main-table default-route device.
//
// tailscale.com/net/netns exists so tailscaled's traffic cannot loop back
// through the routes tailscaled installs. When SO_MARK is unavailable, which
// includes every non-root process because the mark needs CAP_NET_ADMIN, netns
// falls back to SO_BINDTODEVICE on netmon.DefaultRouteInterface(). That device
// is not necessarily the internet path: hosts behind policy routing (Cloudflare
// WARP, split-tunnel VPNs) egress through another interface, so the pinned
// sockets to the control plane and DERP servers stay in SYN-SENT until the
// establish deadline expires. A tsnet node runs a userspace netstack and
// installs no host routes, so there is no route to loop through; upstream's own
// tsnet tests disable netns for the same reason (tsnet/tsnet_test.go,
// "Corp#4520: don't use netns for tests").
//
// netns is also what wires ALL_PROXY into Tailscale's internal dialers. A host
// that depends on an outbound proxy keeps netns enabled, because losing the
// proxy would be worse than the pinning it brings; the warning makes that
// trade-off visible in the provider log.
func useHostRouting(logger *slog.Logger) {
	if proxyConfigured() {
		logger.Warn(
			"tailscale: outbound proxy configured; keeping Tailscale netns so the embedded node can use it, " +
				"socket pinning to the main-table default-route device may remain",
		)
		return
	}
	netns.SetEnabled(false)
}

// proxyConfigured reports whether Tailscale's netns SOCKS wrapping would pick up
// an outbound proxy. x/net/proxy reads both spellings of the variable.
func proxyConfigured() bool {
	for _, name := range []string{"ALL_PROXY", "all_proxy"} {
		if strings.TrimSpace(os.Getenv(name)) != "" {
			return true
		}
	}
	return false
}
