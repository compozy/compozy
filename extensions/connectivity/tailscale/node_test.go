package tailscale

import (
	"context"
	"net"
	"testing"

	"tailscale.com/net/netmon"
	"tailscale.com/net/netns"
)

func TestTSNetNodeUsesHostRouting(t *testing.T) {
	t.Setenv("ALL_PROXY", "")
	t.Setenv("all_proxy", "")
	t.Setenv("TS_AUTHKEY", "tskey-test")
	// netns is process-global; leave the default enabled for sibling tests.
	t.Cleanup(func() { netns.SetEnabled(true) })

	node, err := newTSNetNode(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("newTSNetNode() error = %v", err)
	}
	t.Cleanup(func() {
		if err := node.Close(context.Background()); err != nil {
			t.Logf("node.Close(cleanup) error = %v", err)
		}
	})

	// tsnet takes its control-plane and DERP dialers from netns.FromDialer. With
	// the logical netns enabled, that dialer carries a Control hook that sets
	// SO_BINDTODEVICE to the main-table default-route device, which is what
	// stranded establishes on hosts whose egress is policy-routed elsewhere.
	dialer := netns.FromDialer(func(string, ...any) {}, netmon.NewStatic(), &net.Dialer{})
	plain, ok := dialer.(*net.Dialer)
	if !ok {
		t.Fatalf("netns.FromDialer() type = %T, want *net.Dialer", dialer)
	}
	if plain.Control != nil {
		t.Fatal("embedded node dialer pins sockets to a host device")
	}
}
