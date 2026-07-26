package diagnostics

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRedactHandlesQuotedJSONSecretsAndBounds(t *testing.T) {
	t.Parallel()

	t.Run("Should redact quoted JSON secret values", func(t *testing.T) {
		t.Parallel()

		redacted := Redact(`{"access_token":"abc","refresh_token":"def","safe":"ok"}`)
		if strings.Contains(redacted, "abc") || strings.Contains(redacted, "def") {
			t.Fatalf("Redact(JSON secrets) = %q, want token material removed", redacted)
		}
		if !strings.Contains(redacted, `"access_token":"[REDACTED]"`) ||
			!strings.Contains(redacted, `"refresh_token":"[REDACTED]"`) ||
			!strings.Contains(redacted, `"safe":"ok"`) {
			t.Fatalf("Redact(JSON secrets) = %q, want quoted redacted values and safe fields preserved", redacted)
		}
	})

	t.Run("Should preserve non JSON secret redaction shape", func(t *testing.T) {
		t.Parallel()

		if got, want := Redact("token=super-secret"), "token=[REDACTED]"; got != want {
			t.Fatalf("Redact(token=) = %q, want %q", got, want)
		}
	})

	t.Run("Should redact complete authorization header values", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			input string
			leak  string
			want  string
		}{
			{
				name:  "Should redact basic authorization header",
				input: "Authorization: Basic dXNlcjpwYXNz",
				leak:  "dXNlcjpwYXNz",
				want:  "Authorization: [REDACTED]",
			},
			{
				name:  "Should redact proxy authorization header",
				input: "Proxy-Authorization: Basic dXNlcjpwYXNz",
				leak:  "dXNlcjpwYXNz",
				want:  "Proxy-Authorization: [REDACTED]",
			},
			{
				name:  "Should redact bearer authorization token",
				input: "Authorization: Bearer token-value",
				leak:  "token-value",
				want:  "Authorization: [REDACTED]",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := Redact(tc.input)
				if strings.Contains(got, tc.leak) {
					t.Fatalf("Redact(%q) = %q leaked %q", tc.input, got, tc.leak)
				}
				if got != tc.want {
					t.Fatalf("Redact(%q) = %q, want %q", tc.input, got, tc.want)
				}
			})
		}
	})

	t.Run("Should redact MCP OAuth PKCE and secret binding values", func(t *testing.T) {
		t.Parallel()

		redacted := Redact(
			`{"mcp_auth_token":"mcp-raw","authorization_code":"code-raw","code_verifier":"verifier-raw","secret_binding":"binding-raw","safe":"ok"} oauth_code=oauth-raw pkce_verifier=pkce-raw`,
		)
		for _, leaked := range []string{
			"mcp-raw",
			"code-raw",
			"verifier-raw",
			"binding-raw",
			"oauth-raw",
			"pkce-raw",
		} {
			if strings.Contains(redacted, leaked) {
				t.Fatalf("Redact(MCP/OAuth secrets) = %q leaked %q", redacted, leaked)
			}
		}
		if !strings.Contains(redacted, `"safe":"ok"`) {
			t.Fatalf("Redact(MCP/OAuth secrets) = %q, want safe field preserved", redacted)
		}
	})

	t.Run("Should redact AGH composite secret keys and token colon assignments", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name  string
			input string
			leak  string
		}{
			{
				name:  "Should redact claim token assignment",
				input: "claim_token=agh_claim_secret_123",
				leak:  "agh_claim_secret_123",
			},
			{
				name:  "Should redact claim token colon assignment",
				input: "claim_token: agh_claim_secret_456",
				leak:  "agh_claim_secret_456",
			},
			{
				name:  "Should redact quoted claim token",
				input: `{"claim_token":"agh_claim_secret_789","safe":"ok"}`,
				leak:  "agh_claim_secret_789",
			},
			{
				name:  "Should redact lease token assignment",
				input: "lease_token=lease-raw-secret",
				leak:  "lease-raw-secret",
			},
			{
				name:  "Should redact client secret assignment",
				input: "client_secret=client-raw-secret",
				leak:  "client-raw-secret",
			},
			{
				name:  "Should redact OAuth client secret assignment",
				input: "oauth_client_secret: oauth-client-raw",
				leak:  "oauth-client-raw",
			},
			{
				name:  "Should redact webhook secret assignment",
				input: "webhook_secret=webhook-raw-secret",
				leak:  "webhook-raw-secret",
			},
			{
				name:  "Should redact bot token assignment",
				input: "bot_token=bot-raw-token",
				leak:  "bot-raw-token",
			},
			{
				name:  "Should redact hyphenated API key assignment",
				input: "api-key=api-raw-secret",
				leak:  "api-raw-secret",
			},
			{
				name:  "Should redact token colon assignment",
				input: `token : "token-colon-secret"`,
				leak:  "token-colon-secret",
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				redacted := Redact(tc.input)
				if strings.Contains(redacted, tc.leak) {
					t.Fatalf("Redact(%q) = %q leaked %q", tc.input, redacted, tc.leak)
				}
				if !strings.Contains(redacted, redactedValue) {
					t.Fatalf("Redact(%q) = %q, want redacted placeholder", tc.input, redacted)
				}
			})
		}
	})

	t.Run("Should preserve benign token text without assignment delimiter", func(t *testing.T) {
		t.Parallel()

		const input = "next token should remain visible"
		if got := Redact(input); got != input {
			t.Fatalf("Redact(benign token text) = %q, want %q", got, input)
		}
	})

	t.Run("Should preserve already redacted claim token markers", func(t *testing.T) {
		t.Parallel()

		const input = "task: invalid claim token: agh_claim_[REDACTED]"
		if got := Redact(input); got != input {
			t.Fatalf("Redact(redacted claim token marker) = %q, want %q", got, input)
		}
	})

	t.Run("Should redact bare AGH claim tokens in display text", func(t *testing.T) {
		t.Parallel()

		const leaked = "agh_claim_display_secret_123"
		redacted := Redact("stdout returned " + leaked)
		if strings.Contains(redacted, leaked) {
			t.Fatalf("Redact(bare claim token) = %q leaked %q", redacted, leaked)
		}
		if !strings.Contains(redacted, "agh_claim_[REDACTED]") {
			t.Fatalf("Redact(bare claim token) = %q, want claim-token placeholder", redacted)
		}
	})

	t.Run("Should keep non positive byte budgets bounded", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name     string
			maxBytes int
		}{
			{name: "Should return empty bounded result for zero bytes", maxBytes: 0},
			{name: "Should return empty bounded result for negative bytes", maxBytes: -1},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if got := RedactAndBound("token=super-secret", tc.maxBytes); got != "" {
					t.Fatalf("RedactAndBound(maxBytes=%d) = %q, want empty bounded result", tc.maxBytes, got)
				}
			})
		}
	})

	t.Run("Should append truncation marker after redacting bounded text", func(t *testing.T) {
		t.Parallel()

		redacted := RedactAndBound("prefix token=super-secret suffix", 24)
		if strings.Contains(redacted, "super-secret") {
			t.Fatalf("RedactAndBound() = %q, want token material removed", redacted)
		}
		if !strings.HasSuffix(redacted, truncationSuffix) {
			t.Fatalf("RedactAndBound() = %q, want truncation suffix", redacted)
		}
		if got, want := len(redacted), 24; got != want {
			t.Fatalf("len(RedactAndBound()) = %d, want %d", got, want)
		}
	})

	t.Run("Should keep bounded UTF8 text valid at truncation boundaries", func(t *testing.T) {
		t.Parallel()

		withoutSuffix := RedactAndBound("ééé", 3)
		if !utf8.ValidString(withoutSuffix) {
			t.Fatalf("RedactAndBound(multibyte small bound) = %q, want valid UTF-8", withoutSuffix)
		}
		if got, want := withoutSuffix, "é"; got != want {
			t.Fatalf("RedactAndBound(multibyte small bound) = %q, want %q", got, want)
		}

		withSuffix := RedactAndBound("éééééééééé", len(truncationSuffix)+3)
		if !utf8.ValidString(withSuffix) {
			t.Fatalf("RedactAndBound(multibyte suffix bound) = %q, want valid UTF-8", withSuffix)
		}
		if !strings.HasSuffix(withSuffix, truncationSuffix) {
			t.Fatalf("RedactAndBound(multibyte suffix bound) = %q, want truncation suffix", withSuffix)
		}
		if len(withSuffix) > len(truncationSuffix)+3 {
			t.Fatalf(
				"len(RedactAndBound(multibyte suffix bound)) = %d, want <= %d",
				len(withSuffix),
				len(truncationSuffix)+3,
			)
		}
	})
}

func TestRedactHandlesRuntimeRegisteredSecrets(t *testing.T) {
	t.Parallel()

	t.Run("Should redact dynamic provider secret values", func(t *testing.T) {
		t.Parallel()

		secret := "runtime-dynamic-provider-secret-123456"
		cleanup := RegisterDynamicSecret(secret)
		t.Cleanup(cleanup)

		redacted := Redact("provider stderr leaked " + secret)
		if strings.Contains(redacted, secret) {
			t.Fatalf("Redact(dynamic secret) = %q leaked registered value", redacted)
		}
		if !strings.Contains(redacted, "[REDACTED]") {
			t.Fatalf("Redact(dynamic secret) = %q, want redacted placeholder", redacted)
		}
	})

	t.Run("Should unregister dynamic provider secret values", func(t *testing.T) {
		t.Parallel()

		secret := "runtime-dynamic-provider-secret-cleanup-123456"
		cleanup := RegisterDynamicSecret(secret)
		cleanup()

		redacted := Redact("provider stderr contains " + secret)
		if !strings.Contains(redacted, secret) {
			t.Fatalf("Redact(after cleanup) = %q, want unregistered value unchanged", redacted)
		}
	})

	t.Run("Should keep duplicate dynamic secrets registered until final cleanup", func(t *testing.T) {
		t.Parallel()

		secret := "runtime-dynamic-provider-secret-refcount-123456"
		firstCleanup := RegisterDynamicSecret(secret)
		secondCleanup := RegisterDynamicSecret(secret)
		firstCleanup()

		redacted := Redact("provider stderr contains " + secret)
		if strings.Contains(redacted, secret) {
			t.Fatalf("Redact(after first cleanup) = %q leaked registered value", redacted)
		}

		secondCleanup()
		redacted = Redact("provider stderr contains " + secret)
		if !strings.Contains(redacted, secret) {
			t.Fatalf("Redact(after final cleanup) = %q, want unregistered value unchanged", redacted)
		}
	})

	t.Run("Should redact longer dynamic secrets before prefix secrets", func(t *testing.T) {
		t.Parallel()

		shortSecret := "runtime-dynamic-prefix-material"
		longSecret := shortSecret + "-with-long-tail"
		shortCleanup := RegisterDynamicSecret(shortSecret)
		longCleanup := RegisterDynamicSecret(longSecret)
		t.Cleanup(shortCleanup)
		t.Cleanup(longCleanup)

		if got, want := Redact("leaked "+longSecret), "leaked [REDACTED]"; got != want {
			t.Fatalf("Redact(prefix dynamic secret) = %q, want %q", got, want)
		}
	})

	t.Run("Should ignore blank and short dynamic secrets", func(t *testing.T) {
		t.Parallel()

		blankCleanup := RegisterDynamicSecret("   ")
		shortCleanup := RegisterDynamicSecret("short")
		blankCleanup()
		shortCleanup()

		if got, want := Redact("short"), "short"; got != want {
			t.Fatalf("Redact(short dynamic secret) = %q, want %q", got, want)
		}
	})
}
