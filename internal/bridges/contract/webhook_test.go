// Suite: bridge public webhook configuration
// Invariant: provider callbacks use complete public HTTPS URLs and reject non-public IP destinations.
// Boundary IN: provider_config webhook.public_url and resolved destination addresses.
// Boundary OUT: DNS resolution and HTTP reachability, owned by bridgesdk transport suites.
package contract

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
)

func TestWebhookPublicURLContract(t *testing.T) {
	t.Parallel()

	t.Run("Should accept complete public HTTPS callbacks", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name   string
			config string
			want   string
		}{
			{
				name:   "Should accept a public hostname callback",
				config: `{"webhook":{"public_url":"https://bridge.example.com/slack/support"}}`,
				want:   "https://bridge.example.com/slack/support",
			},
			{
				name:   "Should accept a public IPv4 callback",
				config: `{"webhook":{"public_url":"https://8.8.8.8/slack/support"}}`,
				want:   "https://8.8.8.8/slack/support",
			},
			{
				name:   "Should trim a configured callback",
				config: `{"webhook":{"public_url":"  https://bridge.example.com/hooks  "}}`,
				want:   "https://bridge.example.com/hooks",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				got, err := WebhookPublicURL(BridgeInstance{ProviderConfig: json.RawMessage(test.config)})
				if err != nil {
					t.Fatalf("WebhookPublicURL() error = %v", err)
				}
				if got != test.want {
					t.Fatalf("WebhookPublicURL() = %q, want %q", got, test.want)
				}
			})
		}
	})

	t.Run("Should reject incomplete unsafe or non-public callbacks", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			config     string
			wantErrSub string
		}{
			{name: "Should reject empty provider config", config: ``, wantErrSub: "public_url is required"},
			{
				name:       "Should reject malformed provider config",
				config:     `{"webhook":`,
				wantErrSub: "decode provider config",
			},
			{
				name:       "Should reject a missing public URL",
				config:     `{"webhook":{"path":"/hooks"}}`,
				wantErrSub: "public_url is required",
			},
			{
				name:       "Should reject a malformed public URL",
				config:     `{"webhook":{"public_url":"https://%zz"}}`,
				wantErrSub: "parse webhook public_url",
			},
			{
				name:       "Should reject insecure HTTP",
				config:     `{"webhook":{"public_url":"http://bridge.example.com/hooks"}}`,
				wantErrSub: "must use HTTPS",
			},
			{
				name:       "Should reject a URL without a host",
				config:     `{"webhook":{"public_url":"https:///hooks"}}`,
				wantErrSub: "must include a host",
			},
			{
				name:       "Should reject URL credentials",
				config:     `{"webhook":{"public_url":"https://user@bridge.example.com/hooks"}}`,
				wantErrSub: "userinfo or fragment",
			},
			{
				name:       "Should reject URL fragments",
				config:     `{"webhook":{"public_url":"https://bridge.example.com/hooks#token"}}`,
				wantErrSub: "userinfo or fragment",
			},
			{
				name:       "Should reject localhost",
				config:     `{"webhook":{"public_url":"https://localhost/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject localhost subdomains",
				config:     `{"webhook":{"public_url":"https://bridge.localhost/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject loopback IPv4",
				config:     `{"webhook":{"public_url":"https://127.0.0.1/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject private IPv4",
				config:     `{"webhook":{"public_url":"https://10.20.30.40/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject link-local metadata IPv4",
				config:     `{"webhook":{"public_url":"https://169.254.169.254/latest/meta-data"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject multicast IPv4",
				config:     `{"webhook":{"public_url":"https://224.0.0.1/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject NAT64",
				config:     `{"webhook":{"public_url":"https://[64:ff9b::a00:1]/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject local-use NAT64",
				config:     `{"webhook":{"public_url":"https://[64:ff9b:1::a00:1]/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject 6to4",
				config:     `{"webhook":{"public_url":"https://[2002:a00:1::]/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject Teredo",
				config:     `{"webhook":{"public_url":"https://[2001::1]/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject benchmark IPv4",
				config:     `{"webhook":{"public_url":"https://198.18.0.1/hooks"}}`,
				wantErrSub: "publicly routable",
			},
			{
				name:       "Should reject documentation IPv4",
				config:     `{"webhook":{"public_url":"https://192.0.2.1/hooks"}}`,
				wantErrSub: "publicly routable",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				got, err := WebhookPublicURL(BridgeInstance{ProviderConfig: json.RawMessage(test.config)})
				if err == nil || !strings.Contains(err.Error(), test.wantErrSub) {
					t.Fatalf("WebhookPublicURL() = (%q, %v), want error containing %q", got, err, test.wantErrSub)
				}
			})
		}
	})
}

func TestValidateWebhookDestinationIPContract(t *testing.T) {
	t.Parallel()

	t.Run("Should accept public unicast destinations", func(t *testing.T) {
		t.Parallel()

		for _, raw := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
			t.Run("Should accept "+raw, func(t *testing.T) {
				t.Parallel()

				if err := ValidateWebhookDestinationIP(netip.MustParseAddr(raw)); err != nil {
					t.Fatalf("ValidateWebhookDestinationIP(%q) error = %v", raw, err)
				}
			})
		}
	})

	t.Run("Should reject invalid and non-public destinations", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			ip   netip.Addr
		}{
			{name: "Should reject an invalid address", ip: netip.Addr{}},
			{name: "Should reject an unspecified address", ip: netip.MustParseAddr("0.0.0.0")},
			{name: "Should reject IPv6 loopback", ip: netip.MustParseAddr("::1")},
			{name: "Should reject IPv6 private space", ip: netip.MustParseAddr("fd00::1")},
			{name: "Should reject IPv6 link-local space", ip: netip.MustParseAddr("fe80::1")},
			{name: "Should reject a zoned IPv6 address", ip: netip.MustParseAddr("fe80::1%eth0")},
			{name: "Should reject carrier-grade NAT", ip: netip.MustParseAddr("100.64.0.1")},
			{name: "Should reject IPv4 future-use space", ip: netip.MustParseAddr("240.0.0.1")},
			{name: "Should reject IPv4-mapped private space", ip: netip.MustParseAddr("::ffff:10.0.0.1")},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				if err := ValidateWebhookDestinationIP(test.ip); err == nil {
					t.Fatalf("ValidateWebhookDestinationIP(%q) error = nil", test.ip)
				}
			})
		}
	})
}
