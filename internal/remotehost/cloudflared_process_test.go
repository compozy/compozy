package remotehost

import (
	"slices"
	"testing"
)

func TestHostedConnectorDiagnosis(t *testing.T) {
	t.Parallel()
	t.Run("Should recognize blocked Cloudflare edge port across stderr writes (UT-070)", func(t *testing.T) {
		t.Parallel()
		var diagnostics cloudflaredDiagnostics
		for _, chunk := range []string{"unrelated error\nfailed to dial to edge with quic: ", "203.0.113.1:7844\n"} {
			n, err := diagnostics.Write([]byte(chunk))
			if n != len(chunk) || err != nil {
				t.Fatalf("stderr write = %d, %v", n, err)
			}
		}
		const want = "Hosted route can't reach Cloudflare (port 7844 blocked)"
		if got := diagnostics.diagnosis(); got != want {
			t.Fatalf("diagnosis = %q, want %q", got, want)
		}
	})
}

func TestHostedConnectorProxyEnvironment(t *testing.T) {
	t.Run("Should pass standard proxy variables to cloudflared (UT-133)", func(t *testing.T) {
		expected := map[string]string{
			"HTTP_PROXY":  "http://proxy.test:8080",
			"HTTPS_PROXY": "http://proxy.test:8443",
			"NO_PROXY":    "internal.test",
		}
		for name, value := range expected {
			t.Setenv(name, value)
		}
		env := cloudflaredEnv(t.TempDir(), "tunnel-credential")
		for name, value := range expected {
			if !slices.Contains(env, name+"="+value) {
				t.Errorf("cloudflared did not receive %s", name)
			}
		}
	})
}
