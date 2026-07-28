package auth

import (
	"strings"
	"testing"
)

func TestResolveMetadataURLValidatesTransportAndIssuerPath(t *testing.T) {
	t.Parallel()

	t.Run("Should build RFC8414 path-aware metadata URL", func(t *testing.T) {
		t.Parallel()

		got, err := resolveMetadataURL(ServerConfig{IssuerURL: "https://issuer.example/tenant/v1"})
		if err != nil {
			t.Fatalf("resolveMetadataURL(path issuer) error = %v", err)
		}
		want := "https://issuer.example/.well-known/oauth-authorization-server/tenant/v1"
		if got != want {
			t.Fatalf("resolveMetadataURL(path issuer) = %q, want %q", got, want)
		}
	})

	t.Run("Should reject plaintext non loopback metadata URLs", func(t *testing.T) {
		t.Parallel()

		for _, cfg := range []ServerConfig{
			{MetadataURL: "http://issuer.example/.well-known/oauth-authorization-server"},
			{IssuerURL: "http://issuer.example"},
		} {
			if _, err := resolveMetadataURL(cfg); err == nil || !strings.Contains(err.Error(), "https") {
				t.Fatalf("resolveMetadataURL(%#v) error = %v, want https enforcement", cfg, err)
			}
		}
	})

	t.Run("Should allow plaintext loopback metadata URLs", func(t *testing.T) {
		t.Parallel()

		got, err := resolveMetadataURL(ServerConfig{IssuerURL: "http://127.0.0.1:8080/oauth"})
		if err != nil {
			t.Fatalf("resolveMetadataURL(loopback) error = %v", err)
		}
		want := "http://127.0.0.1:8080/.well-known/oauth-authorization-server/oauth"
		if got != want {
			t.Fatalf("resolveMetadataURL(loopback) = %q, want %q", got, want)
		}
	})
}

func TestMetadataValidateRequiresSecureCredentialEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should reject plaintext non loopback OAuth endpoints", func(t *testing.T) {
		t.Parallel()

		metadata := Metadata{
			AuthorizationEndpoint: "http://issuer.example/authorize",
			TokenEndpoint:         "https://issuer.example/token",
		}
		if err := metadata.Validate(); err == nil || !strings.Contains(err.Error(), "https") {
			t.Fatalf("Metadata.Validate(non-loopback http) error = %v, want https enforcement", err)
		}
	})

	t.Run("Should allow plaintext loopback OAuth endpoints", func(t *testing.T) {
		t.Parallel()

		metadata := Metadata{
			AuthorizationEndpoint: "http://localhost:8080/authorize",
			TokenEndpoint:         "http://127.0.0.1:8080/token",
			RevocationEndpoint:    "http://[::1]:8080/revoke",
		}
		if err := metadata.Validate(); err != nil {
			t.Fatalf("Metadata.Validate(loopback http) error = %v", err)
		}
	})
}
