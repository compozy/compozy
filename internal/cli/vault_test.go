package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestVaultCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list vault metadata with filters and jsonl output", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{
			listVaultSecretsFn: func(_ context.Context, query VaultListQuery) ([]VaultRecord, error) {
				if query.Prefix != "vault:sessions/sess-1/" || query.Namespace != "sessions" {
					t.Fatalf("ListVaultSecrets() query = %#v, want session prefix and namespace", query)
				}
				return []VaultRecord{
					{
						Ref:       "vault:sessions/sess-1/github-token",
						Namespace: "sessions",
						Kind:      "token",
						Present:   true,
						UpdatedAt: fixedTestNow,
					},
					{
						Ref:       "vault:sessions/sess-1/slack-token",
						Namespace: "sessions",
						Kind:      "token",
						Present:   true,
						UpdatedAt: fixedTestNow,
					},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"vault",
			"list",
			"--prefix",
			"vault:sessions/sess-1/",
			"--namespace",
			"sessions",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("vault list jsonl error = %v", err)
		}

		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 2 {
			t.Fatalf("vault list jsonl lines = %d, want 2: %q", len(lines), stdout)
		}
		var decoded VaultRecord
		if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(first vault line) error = %v", err)
		}
		if decoded.Ref != "vault:sessions/sess-1/github-token" || decoded.Kind != "token" || !decoded.Present {
			t.Fatalf("decoded first vault line = %#v", decoded)
		}
	})

	t.Run("Should put vault secret from stdin without printing plaintext", func(t *testing.T) {
		t.Parallel()

		var captured PutVaultSecretRequest
		deps := newTestDeps(t, &stubClient{
			putVaultSecretFn: func(_ context.Context, request PutVaultSecretRequest) (VaultRecord, error) {
				captured = request
				return VaultRecord{
					Ref:       request.Ref,
					Namespace: "sessions",
					Kind:      request.Kind,
					Present:   true,
					CreatedAt: fixedTestNow,
					UpdatedAt: fixedTestNow,
				}, nil
			},
		})

		cmd := newRootCommand(deps)
		var stdout strings.Builder
		cmd.SetOut(&stdout)
		cmd.SetIn(strings.NewReader("super-secret-token\n"))
		cmd.SetArgs([]string{
			"vault",
			"put",
			"vault:sessions/sess-1/github-token",
			"--kind",
			"token",
			"--value-stdin",
			"-o",
			"json",
		})

		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("vault put error = %v", err)
		}
		if captured.Ref != "vault:sessions/sess-1/github-token" ||
			captured.Kind != "token" ||
			captured.SecretValue != "super-secret-token" {
			t.Fatalf("captured vault put request = %#v", captured)
		}
		if strings.Contains(stdout.String(), "super-secret-token") {
			t.Fatalf("vault put output leaked plaintext: %s", stdout.String())
		}
		var decoded VaultRecord
		if err := json.Unmarshal([]byte(stdout.String()), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(vault put) error = %v", err)
		}
		if decoded.Ref != captured.Ref || !decoded.Present {
			t.Fatalf("decoded vault put = %#v, want stored metadata", decoded)
		}
	})

	t.Run("Should delete vault secret and render deleted status", func(t *testing.T) {
		t.Parallel()

		var deletedRef string
		deps := newTestDeps(t, &stubClient{
			deleteVaultSecretFn: func(_ context.Context, ref string) error {
				deletedRef = ref
				return nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"vault",
			"delete",
			"vault:sessions/sess-1/github-token",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("vault delete error = %v", err)
		}
		if deletedRef != "vault:sessions/sess-1/github-token" {
			t.Fatalf("DeleteVaultSecret() ref = %q, want session ref", deletedRef)
		}
		var decoded vaultDeleteRecord
		if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
			t.Fatalf("json.Unmarshal(vault delete) error = %v", err)
		}
		if decoded.Ref != deletedRef || decoded.Status != "deleted" {
			t.Fatalf("decoded vault delete = %#v", decoded)
		}
	})
}
