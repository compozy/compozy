package remotehost

import "testing"

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
