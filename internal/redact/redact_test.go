package redact

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const redactionSnapshotHelperEnv = "COMPOZY_TEST_REDACTION_SNAPSHOT_HELPER"

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
			name: "Should redact Compozy MCP OAuth PKCE and binding secrets",
			input: strings.Join([]string{
				"compozy_claim_raw-claim-value",
				"mcp_auth_token=mcp-token-value",
				"authorization_code=oauth-code-value",
				"code_verifier=pkce-verifier-value",
				"secret_ref=vault:bridges/slack/token",
				"client_secret_ref=env:CLIENT_SECRET",
				"webhook_secret_ref=vault:webhook-secret",
				"workspace_secret=workspace-secret-value",
			}, " "),
			leaks: []string{
				"compozy_claim_raw-claim-value",
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

	t.Run("Should retain a short required secret until every registration is released", func(t *testing.T) {
		t.Parallel()

		const secret = "ACT"
		firstCleanup := RegisterRequiredSecret(secret)
		secondCleanup := RegisterRequiredSecret(secret)
		t.Cleanup(firstCleanup)
		t.Cleanup(secondCleanup)

		got := String("opaque " + secret)
		if want := "opaque " + Marker; got != want {
			t.Fatalf("String(required secret) = %q, want %q", got, want)
		}
		if twice := String(got); twice != got {
			t.Fatalf("String(String(required secret)) = %q, want idempotent %q", twice, got)
		}

		firstCleanup()
		firstCleanup()
		if got, want := String("opaque "+secret), "opaque "+Marker; got != want {
			t.Fatalf("String(after first required cleanup) = %q, want %q", got, want)
		}

		secondCleanup()
		secondCleanup()
		if got, want := String("opaque "+secret), "opaque "+secret; got != want {
			t.Fatalf("String(after final required cleanup) = %q, want %q", got, want)
		}
	})

	t.Run("Should ignore a blank required secret", func(t *testing.T) {
		t.Parallel()

		cleanup := RegisterRequiredSecret(" \t ")
		cleanup()
		cleanup()
		if got, want := String("visible"), "visible"; got != want {
			t.Fatalf("String(after blank required registration) = %q, want %q", got, want)
		}
	})

	t.Run("Should redact a required secret that contains the canonical marker", func(t *testing.T) {
		t.Parallel()

		secret := "prefix-" + Marker + "-suffix"
		cleanup := RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)

		got := String("opaque " + secret)
		if want := "opaque " + Marker; got != want {
			t.Fatalf("String(marker-bearing secret) = %q, want %q", got, want)
		}
		if twice := String(got); twice != got {
			t.Fatalf("String(String(marker-bearing secret)) = %q, want idempotent %q", twice, got)
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

		if !ContainsRawClaimToken("prefix COMPOZY_CLAIM_secret-value suffix") {
			t.Fatal("ContainsRawClaimToken() = false, want true")
		}
		if ContainsRawClaimToken("field name compozy_claim_token") {
			t.Fatal("ContainsRawClaimToken(placeholder) = true, want false")
		}
		if got, want := ClaimTokens("COMPOZY_CLAIM_secret-value api_key=visible"),
			"compozy_claim_[REDACTED] api_key=visible"; got != want {
			t.Fatalf("ClaimTokens() = %q, want %q", got, want)
		}
	})

	t.Run("Should redact claim tokens without relying on word boundaries", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			input string
			leak  string
			want  string
		}{
			{
				name:  "Should redact a token immediately preceded by a word character",
				input: "xcompozy_claim_prefixed-secret",
				leak:  "compozy_claim_prefixed-secret",
				want:  "xcompozy_claim_[REDACTED]",
			},
			{
				name:  "Should redact a URL-safe token ending in a hyphen",
				input: "compozy_claim_trailing- suffix",
				leak:  "compozy_claim_trailing-",
				want:  "compozy_claim_[REDACTED] suffix",
			},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				if !ContainsRawClaimToken(tc.input) {
					t.Fatalf("ContainsRawClaimToken(%q) = false, want true", tc.input)
				}
				if got := ClaimTokens(tc.input); got != tc.want {
					t.Fatalf("ClaimTokens(%q) = %q, want %q", tc.input, got, tc.want)
				} else if strings.Contains(got, tc.leak) {
					t.Fatalf("ClaimTokens(%q) = %q, leaked %q", tc.input, got, tc.leak)
				}
			})
		}
	})
}

func TestClaimTokensJSON(t *testing.T) {
	t.Parallel()

	t.Run("Should remove raw claim token fields and values while preserving safe JSON", func(t *testing.T) {
		t.Parallel()

		const rawToken = "compozy_claim_json-secret"
		const largeInteger = "9007199254740993123456789"
		redacted := ClaimTokensJSON(json.RawMessage(
			`{"count":` + largeInteger + `,"claim_TOKEN":"` + rawToken + `",` +
				`"note":"provider returned ` + rawToken + `","nested":{"proof":"` + rawToken + `"},` +
				`"compozy_claim_json-key":"discarded"}`,
		))
		if strings.Contains(string(redacted), rawToken) {
			t.Fatalf("ClaimTokensJSON() = %s, leaked raw bearer", redacted)
		}
		if !json.Valid(redacted) {
			t.Fatalf("ClaimTokensJSON() = %s, want valid JSON", redacted)
		}
		if !strings.Contains(string(redacted), `"count":`+largeInteger) {
			t.Fatalf("ClaimTokensJSON() = %s, want exact number %s", redacted, largeInteger)
		}
		if strings.Contains(string(redacted), `"claim_TOKEN"`) {
			t.Fatalf("ClaimTokensJSON() = %s, want claim_token field removed", redacted)
		}
	})

	t.Run("Should encode malformed input as a redacted JSON string", func(t *testing.T) {
		t.Parallel()

		const rawToken = "compozy_claim_malformed-json-secret"
		redacted := ClaimTokensJSON(json.RawMessage(`{"note":"` + rawToken + `"`))
		if strings.Contains(string(redacted), rawToken) {
			t.Fatalf("ClaimTokensJSON() = %s, leaked raw bearer", redacted)
		}
		if !json.Valid(redacted) {
			t.Fatalf("ClaimTokensJSON() = %s, want valid JSON", redacted)
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

	t.Run("Should redact registered secrets from bytes and reversible JSON binary envelopes", func(t *testing.T) {
		t.Parallel()

		const secret = "b3?"
		cleanup := RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)

		binary := []byte("prefix " + secret + " suffix")
		redactedBytes := ExactBytes(binary)
		if bytes.Contains(redactedBytes, []byte(secret)) {
			t.Fatalf("ExactBytes() = %q, leaked registered secret", redactedBytes)
		}
		if !bytes.Contains(redactedBytes, []byte(Marker)) {
			t.Fatalf("ExactBytes() = %q, want redaction marker", redactedBytes)
		}
		if !bytes.Contains(binary, []byte(secret)) {
			t.Fatalf("ExactBytes() mutated input = %q", binary)
		}
		encodedText := strings.Join([]string{
			"prefix",
			base64.StdEncoding.EncodeToString([]byte(secret)),
			strings.ToUpper(hex.EncodeToString([]byte(secret))),
			"credential=b3%3f",
			"data:text/plain,b3%3F",
			"credential=b3%3F trailing=%",
			"suffix",
		}, " ")
		redactedText := ExactBinaryString(encodedText)
		for _, leaked := range []string{
			base64.StdEncoding.EncodeToString([]byte(secret)),
			strings.ToUpper(hex.EncodeToString([]byte(secret))),
			"b3%3f",
			"b3%3F",
		} {
			if strings.Contains(redactedText, leaked) {
				t.Fatalf("ExactBinaryString() = %q, retained reversible secret %q", redactedText, leaked)
			}
		}
		if !strings.Contains(redactedText, Marker) {
			t.Fatalf("ExactBinaryString() = %q, want redaction marker", redactedText)
		}

		raw, err := json.Marshal(map[string]string{
			"base64":                      base64.StdEncoding.EncodeToString(binary),
			"embedded_data":               "prefix data:text/plain,b3%3F suffix",
			"percent_encoded":             "credential=b3%3f",
			"percent_with_invalid_suffix": "credential=b3%3F trailing=%",
			"data_uri": "data:application/octet-stream;base64," +
				base64.StdEncoding.EncodeToString(binary),
			"hex":                  hex.EncodeToString(binary),
			"safe":                 base64.StdEncoding.EncodeToString([]byte("unrelated bytes")),
			"safe_percent":         "progress=100%25 complete",
			"safe_invalid_percent": "progress=100% complete",
		})
		if err != nil {
			t.Fatalf("json.Marshal(binary envelope) error = %v", err)
		}
		got, err := ExactBinaryJSON(raw)
		if err != nil {
			t.Fatalf("ExactBinaryJSON() error = %v", err)
		}
		for _, leaked := range []string{
			base64.StdEncoding.EncodeToString(binary),
			hex.EncodeToString(binary),
		} {
			if strings.Contains(string(got), leaked) {
				t.Fatalf("ExactBinaryJSON() = %s, retained reversible secret envelope %q", got, leaked)
			}
		}
		if safe := base64.StdEncoding.EncodeToString([]byte("unrelated bytes")); !strings.Contains(string(got), safe) {
			t.Fatalf("ExactBinaryJSON() = %s, want unrelated binary envelope %q", got, safe)
		}
		if !strings.Contains(string(got), "progress=100%25 complete") {
			t.Fatalf("ExactBinaryJSON() = %s, want unrelated percent encoding preserved", got)
		}
		if !strings.Contains(string(got), "progress=100% complete") {
			t.Fatalf("ExactBinaryJSON() = %s, want unrelated invalid percent text preserved", got)
		}
		if twice, exactErr := ExactBinaryJSON(got); exactErr != nil || !bytes.Equal(twice, got) {
			t.Fatalf("ExactBinaryJSON(ExactBinaryJSON(value)) = %s, %v; want idempotent %s", twice, exactErr, got)
		}
		for name, encodedKey := range map[string]string{
			"base64":            base64.StdEncoding.EncodeToString([]byte(secret)),
			"data URI":          "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString([]byte(secret)),
			"embedded data URI": "prefix data:text/plain,b3%3F suffix",
			"hex":               hex.EncodeToString([]byte(secret)),
			"percent encoded":   "credential=b3%3f",
		} {
			keyEnvelope, marshalErr := json.Marshal(map[string]string{encodedKey: "visible"})
			if marshalErr != nil {
				t.Fatalf("json.Marshal(%s key) error = %v", name, marshalErr)
			}
			redactedKey, redactErr := ExactBinaryJSON(keyEnvelope)
			if redactErr != nil {
				t.Fatalf("ExactBinaryJSON(%s key) error = %v", name, redactErr)
			}
			if strings.Contains(string(redactedKey), encodedKey) {
				t.Fatalf("ExactBinaryJSON(%s key) = %s, retained reversible secret key", name, redactedKey)
			}
		}

		collision, err := json.Marshal(map[string]bool{
			base64.StdEncoding.EncodeToString([]byte(secret)): true,
			Marker: false,
		})
		if err != nil {
			t.Fatalf("json.Marshal(binary key collision) error = %v", err)
		}
		_, err = ExactBinaryJSON(collision)
		if !errors.Is(err, errExactJSONKeyCollision) {
			t.Fatalf("ExactBinaryJSON(binary key collision) error = %v, want collision failure", err)
		}
		percentCollision, err := json.Marshal(map[string]bool{
			"credential=b3%3F": true,
			Marker:             false,
		})
		if err != nil {
			t.Fatalf("json.Marshal(percent key collision) error = %v", err)
		}
		_, err = ExactBinaryJSON(percentCollision)
		if !errors.Is(err, errExactJSONKeyCollision) {
			t.Fatalf("ExactBinaryJSON(percent key collision) error = %v, want collision failure", err)
		}

		const encodedMarker = "%5BREDACTED%5D"
		if idempotentMarker := ExactBinaryString(encodedMarker); idempotentMarker != encodedMarker {
			t.Fatalf("ExactBinaryString(encoded marker) = %q, want byte-identical %q", idempotentMarker, encodedMarker)
		}

		cleanup()
		const markerEncodingSecret = "W1J"
		markerCleanup := RegisterRequiredSecret(markerEncodingSecret)
		t.Cleanup(markerCleanup)
		redactedBinary := ExactBytes([]byte(markerEncodingSecret))
		encodedRedactedBinary, err := json.Marshal(redactedBinary)
		if err != nil {
			t.Fatalf("json.Marshal(redacted binary) error = %v", err)
		}
		idempotent, err := ExactBinaryJSON(encodedRedactedBinary)
		if err != nil {
			t.Fatalf("ExactBinaryJSON(redacted binary) error = %v", err)
		}
		if !bytes.Equal(idempotent, encodedRedactedBinary) {
			t.Fatalf("ExactBinaryJSON(redacted binary) = %s, want byte-identical %s", idempotent, encodedRedactedBinary)
		}
	})

	t.Run("Should redact registered secrets from exact JSON keys and values only", func(t *testing.T) {
		t.Parallel()

		const secret = "j4?"
		const highEntropyValue = "Q7mV2pL9xR4nK8sT6wY3cF5hJ1dB0zAq"
		cleanup := RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)

		raw := json.RawMessage(
			`{"j4?":"prefix j4?","entropy":"` + highEntropyValue + `","number":9007199254740993}`,
		)
		got, err := ExactJSON(raw)
		if err != nil {
			t.Fatalf("ExactJSON() error = %v", err)
		}
		if strings.Contains(string(got), secret) {
			t.Fatalf("ExactJSON() = %s, leaked registered secret", got)
		}
		for _, preserved := range []string{highEntropyValue, "9007199254740993"} {
			if !strings.Contains(string(got), preserved) {
				t.Fatalf("ExactJSON() = %s, want preserved %q", got, preserved)
			}
		}
		if twice, exactErr := ExactJSON(got); exactErr != nil || !bytes.Equal(twice, got) {
			t.Fatalf("ExactJSON(ExactJSON(value)) = %s, %v; want idempotent %s", twice, exactErr, got)
		}

		shadowed := json.RawMessage(`{"value":"j4?","value":"visible"}`)
		shadowedResult, err := ExactJSON(shadowed)
		if err != nil {
			t.Fatalf("ExactJSON(shadowed key) error = %v", err)
		}
		if strings.Contains(string(shadowedResult), secret) {
			t.Fatalf("ExactJSON(shadowed key) = %s, leaked overwritten secret", shadowedResult)
		}
	})

	t.Run("Should reject exact JSON key collisions after redaction", func(t *testing.T) {
		t.Parallel()

		const secret = "k5!"
		cleanup := RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)

		_, err := ExactJSON(json.RawMessage(`{"k5!":true,"[REDACTED]":false}`))
		if !errors.Is(err, errExactJSONKeyCollision) {
			t.Fatalf("ExactJSON() error = %v, want key-collision failure", err)
		}
	})

	t.Run("Should preserve exact claim token protection", func(t *testing.T) {
		t.Parallel()

		got := New(Options{}).RedactString("claim=compozy_claim_super-secret-lease")
		assertRedactionRemovedValue(t, got, "compozy_claim_super-secret-lease")
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
			"/private/var/tmp/TestExecuteContextLocalDatabaseMigrationErrorsJ20270725/001/compozy.db",
			`C:\Users\runner\AppData\Local\TestExecuteContextLocalDatabaseMigrationErrorsJ20270725\compozy.db`,
		} {
			if got := New(Options{}).RedactString(path); got != path {
				t.Fatalf("RedactString(path) = %q, want byte-identical %q", got, path)
			}
		}
	})

	t.Run("Should leave whitespace-only streaming chunks byte identical", func(t *testing.T) {
		t.Parallel()

		for _, chunk := range []string{" ", "\n", "\n\n", "\t\r\n"} {
			if got := New(Options{}).RedactString(chunk); got != chunk {
				t.Fatalf("RedactString(%q) = %q, want byte-identical chunk", chunk, got)
			}
		}
	})
}

func TestEngineRedactJSONPreservesStructuredEnvelope(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve protected structured envelope fields", func(t *testing.T) {
		t.Parallel()

		secret := "Q7mV2pL9xR4nK8sT6wY3cF5hJ1dB0zAq"
		wantEnvelope := map[string]string{
			"claim_token_hash":         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"cursor":                   "eyJ2IjoyLCJraW5kIjoiYnVuZGxlIiwib2Zmc2V0IjowfQ",
			redactionSessionIDFieldKey: "550e8400-e29b-41d4-a716-446655440000",
			"run_id":                   "62f82910-18ca-4f2e-aa4a-54dcde9fe761",
			"fingerprint":              "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			"idempotency_key":          "idem_550e8400e29b41d4a716446655440000",
			"next_cursor":              "eyJ2IjoyLCJraW5kIjoiYnVuZGxlIiwib2Zmc2V0IjoxfQ",
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
	})
}

func TestEngineRedactionRecursesIntoProtectedCompositeEnvelopes(t *testing.T) {
	t.Parallel()

	secret := "compozy_claim_nested-envelope-secret"
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
	t.Run("Should freeze the process heuristic setting at boot", testProcessEnabledSnapshot)
}

func testProcessEnabledSnapshot(t *testing.T) {
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
		claimSecret := "compozy_claim_runtime-snapshot-secret"
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

func BenchmarkEngineRedactStructuredLogValue(b *testing.B) {
	engine := New(Options{})
	attrs := []slog.Attr{slog.Any("request", map[string]any{
		"claim_token": "compozy_claim_benchmark-secret",
		"metadata":    map[string]any{"request_id": "req-1", "attempt": 2},
	})}
	b.ReportAllocs()
	for b.Loop() {
		engine.RedactLogAttrs(attrs)
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
