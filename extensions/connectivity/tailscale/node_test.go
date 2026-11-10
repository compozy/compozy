package tailscale

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"tailscale.com/net/netmon"
	"tailscale.com/net/netns"
)

func TestTSNetNodeRouting(t *testing.T) {
	t.Setenv("TS_AUTHKEY", "tskey-test")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	for _, tt := range []struct {
		name     string
		proxyURL string
	}{
		{name: "ShouldUseHostRoutingWithoutProxy"},
		{name: "ShouldKeepNetnsWhenOutboundProxyIsConfigured", proxyURL: "socks5://127.0.0.1:1080"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ALL_PROXY", tt.proxyURL)
			t.Setenv("all_proxy", "")
			// netns is process-global; start each case from the default and
			// leave the default enabled for sibling tests.
			netns.SetEnabled(true)
			t.Cleanup(func() { netns.SetEnabled(true) })

			node, err := newTSNetNode(t.TempDir(), logger)
			if err != nil {
				t.Fatalf("newTSNetNode() error = %v", err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 5*time.Second)
				defer cancel()
				if closeErr := node.Close(ctx); closeErr != nil {
					t.Logf("node.Close(cleanup) error = %v", closeErr)
				}
			})

			// tsnet takes its control-plane and DERP dialers from netns.FromDialer.
			dialer := netns.FromDialer(func(string, ...any) {}, netmon.NewStatic(), &net.Dialer{})
			if tt.proxyURL != "" {
				if !netns.IsSOCKSDialer(dialer) {
					t.Fatal("configured outbound proxy was dropped from the embedded node dialer")
				}
				return
			}
			plain, ok := dialer.(*net.Dialer)
			if !ok {
				t.Fatalf("netns.FromDialer() type = %T, want *net.Dialer", dialer)
			}
			if plain.Control != nil {
				t.Fatal("embedded node dialer pins sockets to a host device")
			}
		})
	}
}
