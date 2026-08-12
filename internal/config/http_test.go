package config

import "testing"

func TestHTTPConfigValidatesAllowedIPs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		allowed   []string
		wantError bool
	}{
		{name: "Should accept IPv4 CIDR", allowed: []string{"192.168.1.0/24"}},
		{name: "Should accept IPv6 address", allowed: []string{"fd00::42"}},
		{name: "Should accept an empty allowlist", allowed: nil},
		{name: "Should reject malformed address", allowed: []string{"not-an-ip"}, wantError: true},
		{name: "Should reject empty entry", allowed: []string{" "}, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := HTTPConfig{Host: "0.0.0.0", Port: 2123, AllowRemoteAccess: true, AllowedIPs: tt.allowed}
			err := config.Validate()
			if (err != nil) != tt.wantError {
				t.Fatalf("Validate() error = %v, want error = %v", err, tt.wantError)
			}
		})
	}
}
