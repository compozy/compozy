package control

import "testing"

func TestHostedHostname(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, computer, want string }{
		{"Should filter a brand case-insensitively (UT-143)", "PayPal", "computer"},
		{"Should filter a reserved slug part (UT-143)", "login box", "computer"},
		{"Should filter the service brand (UT-143)", "compozy", "computer"},
		{"Should keep an ordinary computer name (UT-143)", "devbox", "devbox"},
		{"Should match whole parts rather than substrings (UT-143)", "blogin", "blogin"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if hostname := hostedHostname(tc.computer, "7f3k", "compozy.run"); hostname != tc.want+"-7f3k.compozy.run" {
				t.Fatalf("hostname = %q, want slug %q", hostname, tc.want)
			}
		})
	}
}
