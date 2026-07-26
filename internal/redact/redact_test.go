package redact

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const redactionSnapshotHelperEnv = "AGH_TEST_REDACTION_SNAPSHOT_HELPER"

func TestStringRedactsCanonicalSecretTaxonomy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		leaks []string
	}{
		{
			name: "Should redact provider token shapes",
			input: strings.Join([]string{
				"Bearer bearer-token-value",
				"sk-openai-secret-value",
				"xoxb-slack-secret-value",
				"xapp-slack-app-secret",
				"ghp_githubsecretvalue",
			}, " "),
			leaks: []string{
				"bearer-token-value",
				"sk-openai-secret-value",
				"xoxb-slack-secret-value",
				"xapp-slack-app-secret",
				"ghp_githubsecretvalue",
			},
		},
		{
			name:  "Should redact one-character bearer credentials without consuming delimiters",
			input: "Bearer z, visible",
			want:  "Bearer [REDACTED], visible",
			leaks: []string{"Bearer z"},
		},
		{
			name: "Should redact AGH MCP OAuth PKCE and binding secrets",
			input: strings.Join([]string{
				"agh_claim_raw-claim-value",
				"mcp_auth_token=mcp-token-value",
				"authorization_code=oauth-code-value",
				"code_verifier=pkce-verifier-value",
				"secret_ref=vault:bridges/slack/token",
				"client_secret_ref=env:CLIENT_SECRET",
				"webhook_secret_ref=vault:webhook-secret",
				"workspace_secret=workspace-secret-value",
			}, " "),
			leaks: []string{
				"agh_claim_raw-claim-value",
				"mcp-token-value",
				"oauth-code-value",
				"pkce-verifier-value",
				"vault:bridges/slack/token",
				"env:CLIENT_SECRET",
				"vault:webhook-secret",
				"workspace-secret-value",
			},
		},
		{
			name:  "Should redact quoted generic secret assignments",
			input: `{"api_key":"api-value","auth_token":"auth-value","private_key":"private-value","safe":"visible"}`,
			leaks: []string{"api-value", "auth-value", "private-value"},
		},
		{
			name: "Should redact camel case secret assignments",
			input: `{"apiKey":"api-camel-value","accessToken":"access-camel-value",` +
				`"privateKey":"private-camel-value","codeVerifier":"verifier-camel-value",` +
				`"secretRef":"reference-camel-value"}`,
			leaks: []string{
				"api-camel-value",
				"access-camel-value",
				"private-camel-value",
				"verifier-camel-value",
				"reference-camel-value",
			},
		},
		{
			name:  "Should redact complete authorization headers",
			input: "Proxy-Authorization: Basic dXNlcjpwYXNz",
			leaks: []string{"dXNlcjpwYXNz"},
		},
		{
			name:  "Should redact URL userinfo credentials",
			input: "curl https://admin:s3cr3t@example.com/private",
			want:  "curl https://[REDACTED]@example.com/private",
			leaks: []string{"admin", "s3cr3t"},
		},
		{
			name:  "Should redact generic token assignments",
			input: "connect: token=cause-secret",
			leaks: []string{"cause-secret"},
		},
		{
			name: "Should redact shell flag values and bare secret binding references",
			input: "deploy --password hunter2 --secret-ref vault:bridges/slack/token " +
				"--api-key='api-flag-value' env:OPENAI_API_KEY",
			leaks: []string{
				"hunter2",
				"vault:bridges/slack/token",
				"api-flag-value",
				"env:OPENAI_API_KEY",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := String(tc.input)
			if tc.want != "" && got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
			for _, leak := range tc.leaks {
				if strings.Contains(got, leak) {
					t.Fatalf("String() = %q leaked %q", got, leak)
				}
			}
			if !strings.Contains(got, Marker) {
				t.Fatalf("String() = %q, want marker %q", got, Marker)
			}
		})
	}
}

func TestStringUsesDynamicSecretsAndRemainsIdempotent(t *testing.T) {
	t.Parallel()

	t.Run("Should redact registered dynamic secrets until cleanup", func(t *testing.T) {
		secret := "runtime-secret-material-123456"
		cleanup := RegisterDynamicSecret(secret)
		t.Cleanup(cleanup)

		got := String("provider leaked " + secret)
		if strings.Contains(got, secret) {
			t.Fatalf("String(dynamic) = %q leaked registered secret", got)
		}
		if twice := String(got); twice != got {
			t.Fatalf("String(String(value)) = %q, want idempotent %q", twice, got)
		}
	})

	t.Run("Should expose the shared sensitive key classifier", func(t *testing.T) {
		t.Parallel()

		for _, key := range []string{
			"api-key",
			"apiKey",
			"mcp_auth_token",
			"accessToken",
			"authorization_code",
			"codeVerifier",
			"PKCE",
			"pkce_verifier",
			"Bearer",
			"client_secret_ref",
			"secretRef",
			"privateKey",
			"workspace_secret",
		} {
			if !IsSensitiveKey(key) {
				t.Fatalf("IsSensitiveKey(%q) = false, want true", key)
			}
		}
		for _, key := range []string{"token_present", "tokenPresent", "maxInputTokens", "maxOutputTokens"} {
			if IsSensitiveKey(key) {
				t.Fatalf("IsSensitiveKey(%q) = true, want public diagnostic key", key)
			}
		}
	})

	t.Run("Should own task claim detection and claim-only redaction", func(t *testing.T) {
		t.Parallel()

		if !ContainsRawClaimToken("prefix AGH_CLAIM_secret-value suffix") {
			t.Fatal("ContainsRawClaimToken() = false, want true")
		}
		if ContainsRawClaimToken("field name agh_claim_token") {
			t.Fatal("ContainsRawClaimToken(placeholder) = true, want false")
		}
		if got, want := ClaimTokens("AGH_CLAIM_secret-value api_key=visible"),
			"agh_claim_[REDACTED] api_key=visible"; got != want {
			t.Fatalf("ClaimTokens() = %q, want %q", got, want)
		}
	})
}

func TestEngineRedactsSeededProviderPrefixes(t *testing.T) {
	t.Parallel()

	engine := New(Options{})
	for _, secret := range seededProviderSecretsForTest() {
		t.Run("Should redact "+secret.name+" in text and named JSON content", func(t *testing.T) {
			t.Parallel()

			text := "credential=" + secret.value
			redactedText := engine.RedactString(text)
			assertRedactionRemovedValue(t, redactedText, secret.value)

			raw, err := json.Marshal(map[string]string{
				redactionMessageFieldKey:   text,
				redactionSessionIDFieldKey: "session-visible",
			})
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			redactedJSON := engine.RedactJSON(raw, []string{redactionMessageFieldKey})
			assertRedactionRemovedValue(t, string(redactedJSON), secret.value)
		})
	}
}

func TestEngineComposesExactAndHeuristicRedaction(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve exact claim token protection", func(t *testing.T) {
		t.Parallel()

		got := New(Options{}).RedactString("claim=agh_claim_super-secret-lease")
		assertRedactionRemovedValue(t, got, "agh_claim_super-secret-lease")
	})

	t.Run("Should redact a generic high entropy token", func(t *testing.T) {
		t.Parallel()

		secret := "Q7mV2pL9xR4nK8sT6wY3cF5hJ1dB0zAq"
		got := New(Options{}).RedactString("opaque " + secret)
		assertRedactionRemovedValue(t, got, secret)
	})

	t.Run("Should leave non-secret code-heavy content byte identical", func(t *testing.T) {
		t.Parallel()

		fixture := strings.Join([]string{
			`func correlate(sessionID, runID string) string {`,
			`  claimTokenHash := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"`,
			`  requestID := "550e8400-e29b-41d4-a716-446655440000"`,
			`  return sessionID + ":" + runID + ":" + requestID + ":" + claimTokenHash`,
			`}`,
		}, "\n")
		if got := New(Options{}).RedactString(fixture); got != fixture {
			t.Fatalf("RedactString(code fixture) = %q, want byte-identical %q", got, fixture)
		}
	})

	t.Run("Should leave filesystem paths byte identical", func(t *testing.T) {
		t.Parallel()

		for _, path := range []string{
			"/private/var/tmp/TestExecuteContextLocalDatabaseMigrationErrorsJ20270725/001/agh.db",
			`C:\Users\runner\AppData\Local\TestExecuteContextLocalDatabaseMigrationErrorsJ20270725\agh.db`,
		} {
			if got := New(Options{}).RedactString(path); got != path {
				t.Fatalf("RedactString(path) = %q, want byte-identical %q", got, path)
			}
		}
	})
}

func TestEngineRedactJSONPreservesStructuredEnvelope(t *testing.T) {
	t.Parallel()

	secret := "Q7mV2pL9xR4nK8sT6wY3cF5hJ1dB0zAq"
	wantEnvelope := map[string]string{
		"claim_token_hash":         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		redactionSessionIDFieldKey: "550e8400-e29b-41d4-a716-446655440000",
		"run_id":                   "62f82910-18ca-4f2e-aa4a-54dcde9fe761",
		"fingerprint":              "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"idempotency_key":          "idem_550e8400e29b41d4a716446655440000",
	}
	payload := map[string]string{redactionMessageFieldKey: "leaked " + secret}
	maps.Copy(payload, wantEnvelope)
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	redacted := New(Options{}).RedactJSON(raw, []string{redactionMessageFieldKey})
	var got map[string]string
	if err := json.Unmarshal(redacted, &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v payload=%s", err, redacted)
	}
	assertRedactionRemovedValue(t, got[redactionMessageFieldKey], secret)
	for key, want := range wantEnvelope {
		if got[key] != want {
			t.Fatalf("RedactJSON()[%q] = %q, want intact %q", key, got[key], want)
		}
	}
}

func TestEngineRedactionRecursesIntoProtectedCompositeEnvelopes(t *testing.T) {
	t.Parallel()

	secret := "agh_claim_nested-envelope-secret"
	engine := New(Options{})

	t.Run("Should redact sensitive JSON children under a protected envelope key", func(t *testing.T) {
		t.Parallel()

		raw := json.RawMessage(`{"session_id":{"claim_token":"` + secret + `","request_id":"req-1"}}`)
		redacted := engine.RedactJSON(raw, nil)
		var got map[string]map[string]string
		if err := json.Unmarshal(redacted, &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v payload=%s", err, redacted)
		}
		if got[redactionSessionIDFieldKey]["claim_token"] != Marker {
			t.Fatalf("claim_token = %q, want %q", got[redactionSessionIDFieldKey]["claim_token"], Marker)
		}
		if got[redactionSessionIDFieldKey]["request_id"] != "req-1" {
			t.Fatalf("request_id = %q, want req-1", got[redactionSessionIDFieldKey]["request_id"])
		}
	})

	t.Run("Should redact sensitive log group children under a protected envelope key", func(t *testing.T) {
		t.Parallel()

		attrs := engine.RedactLogAttrs([]slog.Attr{slog.Group(
			redactionSessionIDFieldKey,
			slog.String("claim_token", secret),
			slog.String("request_id", "req-1"),
		)})
		group := attrs[0].Value.Group()
		if group[0].Value.String() != Marker {
			t.Fatalf("claim_token = %q, want %q", group[0].Value.String(), Marker)
		}
		if group[1].Value.String() != "req-1" {
			t.Fatalf("request_id = %q, want req-1", group[1].Value.String())
		}
	})
}

func TestProcessEnabledSnapshot(t *testing.T) {
	if os.Getenv(redactionSnapshotHelperEnv) == "1" {
		SnapshotEnabled(false)
		if Enabled() {
			t.Fatal("Enabled() = true after disabled boot snapshot")
		}
		SnapshotEnabled(true)
		if Enabled() {
			t.Fatal("Enabled() = true after runtime re-snapshot, want immutable false")
		}
		providerSecret := "AIza1234567890abcdefghijklmnopqrstuvwxyzABCD"
		if got := String(providerSecret); got != providerSecret {
			t.Fatalf("String(provider secret) = %q, want heuristic disabled", got)
		}
		claimSecret := "agh_claim_runtime-snapshot-secret"
		assertRedactionRemovedValue(t, String(claimSecret), claimSecret)
		return
	}

	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessEnabledSnapshot$")
	cmd.Env = append(os.Environ(), redactionSnapshotHelperEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("redaction snapshot helper error = %v output=%s", err, output)
	}
}

func BenchmarkEngineRedactString(b *testing.B) {
	engine := New(Options{})
	payload := "assistant output with sk-ant-api03-abcdefghijklmnopqrstuv and visible context"
	b.ReportAllocs()
	for b.Loop() {
		engine.RedactString(payload)
	}
}

type seededProviderSecret struct {
	name  string
	value string
}

func seededProviderSecretsForTest() []seededProviderSecret {
	return []seededProviderSecret{
		{name: "OpenAI and Anthropic", value: "sk-ant-api03-abcdefghijklmnopqrstuv"},
		{name: "GitHub classic PAT", value: "ghp_abcdefghijklmnopqrstuv"},
		{name: "GitHub fine-grained PAT", value: "github_pat_abcdefghijklmnopqrstuv"},
		{name: "GitHub OAuth", value: "gho_abcdefghijklmnopqrstuv"},
		{name: "GitHub user token", value: "ghu_abcdefghijklmnopqrstuv"},
		{name: "GitHub server token", value: "ghs_abcdefghijklmnopqrstuv"},
		{name: "GitHub refresh token", value: "ghr_abcdefghijklmnopqrstuv"},
		{name: "Slack app token", value: "xapp-1-abcdefghijklmnopqrstuv"},
		{name: "Slack bot token", value: "xoxb-abcdefghijklmnopqrstuv"},
		{name: "Google API key", value: "AIza1234567890abcdefghijklmnopqrstuvwxyzABCD"},
		{name: "Perplexity", value: "pplx-abcdefghijklmnopqrstuv"},
		{name: "Fal", value: "fal_abcdefghijklmnopqrstuv"},
		{name: "Firecrawl", value: "fc-abcdefghijklmnopqrstuv"},
		{name: "BrowserBase", value: "bb_live_abcdefghijklmnopqrstuv"},
		{name: "Codex encrypted token", value: "gAAAAabcdefghijklmnopqrstuv1234567890="},
		{name: "AWS access key", value: "AKIA1234567890ABCDEF"},
		{name: "Stripe live", value: "sk_live_abcdefghijklmnopqrstuv"},
		{name: "Stripe test", value: "sk_test_abcdefghijklmnopqrstuv"},
		{name: "Stripe restricted", value: "rk_live_abcdefghijklmnopqrstuv"},
		{name: "SendGrid", value: "SG.abcdefghijklmnopqrstuv"},
		{name: "Hugging Face", value: "hf_abcdefghijklmnopqrstuv"},
		{name: "Replicate", value: "r8_abcdefghijklmnopqrstuv"},
		{name: "npm", value: "npm_abcdefghijklmnopqrstuv"},
		{name: "PyPI", value: "pypi-abcdefghijklmnopqrstuv"},
		{name: "DigitalOcean PAT", value: "dop_v1_abcdefghijklmnopqrstuv"},
		{name: "DigitalOcean OAuth", value: "doo_v1_abcdefghijklmnopqrstuv"},
		{name: "AgentMail", value: "am_abcdefghijklmnopqrstuv"},
		{name: "ElevenLabs", value: "sk_abcdefghijklmnopqrstuv"},
		{name: "Tavily", value: "tvly-abcdefghijklmnopqrstuv"},
		{name: "Exa", value: "exa_abcdefghijklmnopqrstuv"},
		{name: "Groq", value: "gsk_abcdefghijklmnopqrstuv"},
		{name: "Matrix", value: "syt_abcdefghijklmnopqrstuv"},
		{name: "RetainDB", value: "retaindb_abcdefghijklmnopqrstuv"},
		{name: "Hindsight", value: "hsk-abcdefghijklmnopqrstuv"},
		{name: "Mem0", value: "mem0_abcdefghijklmnopqrstuv"},
		{name: "ByteRover", value: "brv_abcdefghijklmnopqrstuv"},
		{name: "xAI", value: "xai-abcdefghijklmnopqrstuvwxyz1234567890"},
		{name: "Notion", value: "ntn_abcdefghijklmnopqrstuv"},
		{name: "Fireworks API", value: "fw-abcdefghijklmnopqrstuvwxyz1234567890"},
		{name: "Fireworks underscore", value: "fw_abcdefghijklmnopqrstuvwxyz1234567890"},
		{name: "Fireworks project", value: "fpk_abcdefghijklmnopqrstuvwxyz1234567890"},
	}
}

func assertRedactionRemovedValue(t testing.TB, got string, secret string) {
	t.Helper()
	if strings.Contains(got, secret) {
		t.Fatalf("redacted value = %q, leaked %q", got, secret)
	}
	if !strings.Contains(got, Marker) {
		t.Fatalf("redacted value = %q, want marker %q", got, Marker)
	}
}
