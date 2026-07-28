package diagnostics

import (
	"fmt"
	"strings"
	"testing"
)

func BenchmarkRedactStaticSecrets(b *testing.B) {
	input := `{"access_token":"abc","refresh_token":"def","safe":"ok"} token=super-secret Bearer token-value`

	b.ReportAllocs()
	for b.Loop() {
		if got := Redact(input); strings.Contains(got, "super-secret") {
			b.Fatalf("Redact() = %q, want token material removed", got)
		}
	}
}

func BenchmarkRedactDynamicSecrets(b *testing.B) {
	cleanups := make([]func(), 0, 32)
	for idx := range 32 {
		secret := fmt.Sprintf("sk-dynamic-secret-%02d-abcdefghijklmnopqrstuvwxyz", idx)
		cleanups = append(cleanups, RegisterDynamicSecret(secret))
	}
	b.Cleanup(func() {
		for _, cleanup := range cleanups {
			cleanup()
		}
	})

	input := "stderr leaked sk-dynamic-secret-00-abcdefghijklmnopqrstuvwxyz and sk-dynamic-secret-31-abcdefghijklmnopqrstuvwxyz"

	b.ReportAllocs()
	for b.Loop() {
		if got := Redact(input); strings.Contains(got, "abcdefghijklmnopqrstuvwxyz") {
			b.Fatalf("Redact() = %q, want dynamic secrets removed", got)
		}
	}
}
