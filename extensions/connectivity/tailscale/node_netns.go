package tailscale

import "tailscale.com/net/netns"

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
// Disabling netns also drops the ALL_PROXY support it provides, and tsnet
// exposes no dialer hook that could restore proxying without the pinning.
func useHostRouting() {
	netns.SetEnabled(false)
}
