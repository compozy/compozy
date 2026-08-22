package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
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

func TestProfileSecretCommandContract(t *testing.T) {
	t.Parallel()

	t.Run("Should complete profile secret set without exposing plaintext [E2E-007]", func(t *testing.T) {
		t.Parallel()

		var captured PutVaultSecretRequest
		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{putVaultSecretFn: func(
				_ context.Context,
				request PutVaultSecretRequest,
			) (VaultRecord, error) {
				captured = request
				return VaultRecord{Ref: request.Ref, Kind: request.Kind, Present: true}, nil
			}}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		cmd := newRootCommand(newTestDeps(t, client))
		var stdout strings.Builder
		cmd.SetOut(&stdout)
		cmd.SetIn(strings.NewReader("profile-secret\n"))
		cmd.SetArgs([]string{
			"--profile", "marketing", "secret", "set", "providers/openai/api_key",
			"--value-stdin", "-o", "json",
		})
		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("secret set profile override error = %v", err)
		}
		if captured.Ref != "vault:profiles/marketing/providers/openai/api_key" ||
			captured.SecretValue != "profile-secret" {
			t.Fatalf("profile secret set request = %#v", captured)
		}
		if strings.Contains(stdout.String(), "profile-secret") {
			t.Fatalf("secret set output leaked plaintext: %s", stdout.String())
		}
		var record secretMutationRecord
		if err := json.Unmarshal([]byte(stdout.String()), &record); err != nil {
			t.Fatalf("decode secret set profile output: %v", err)
		}
		if record.Ref != captured.Ref || record.Profile != "marketing" || record.Status != "saved" {
			t.Fatalf("secret set profile output = %#v", record)
		}
	})

	t.Run("Should bind non-default secrets to the active profile and default secrets to user scope", func(t *testing.T) {
		t.Parallel()

		marketingRef, err := secretRefForProfile("marketing", "providers/openai/api_key")
		if err != nil {
			t.Fatalf("secretRefForProfile(marketing) error = %v", err)
		}
		if got, want := marketingRef, "vault:profiles/marketing/providers/openai/api_key"; got != want {
			t.Fatalf("marketing ref = %q, want %q", got, want)
		}
		defaultRef, err := secretRefForProfile("default", "providers/openai/api_key")
		if err != nil {
			t.Fatalf("secretRefForProfile(default) error = %v", err)
		}
		if got, want := defaultRef, "vault:providers/openai/api_key"; got != want {
			t.Fatalf("default ref = %q, want %q", got, want)
		}
	})

	t.Run("Should refuse process environment imports for profile scope", func(t *testing.T) {
		t.Parallel()

		cmd := &cobra.Command{}
		_, err := readProfileSecretValue(
			cmd,
			commandDeps{getenv: func(string) string { return "secret" }},
			"marketing",
			"OPENAI_API_KEY",
			false,
		)
		if err == nil || !strings.Contains(err.Error(), "process environment is shared") {
			t.Fatalf("readProfileSecretValue(profile env) error = %v, want profile_secret_env_forbidden", err)
		}
	})

	t.Run("Should render profile environment refusal as a structured error [E2E-007]", func(t *testing.T) {
		t.Parallel()

		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active"},
			}},
		}
		deps := newTestDeps(t, client)
		deps.getenv = func(key string) string {
			if key == "OPENAI_API_KEY" {
				return "profile-secret"
			}
			return ""
		}
		exitCode, _, stderr := executeRootCommandWithExit(
			t, deps,
			"--profile", "marketing", "secret", "set", "providers/openai/api_key",
			"--from-env", "OPENAI_API_KEY", "-o", "json",
		)
		if exitCode != 1 {
			t.Fatalf("secret set --from-env exit = %d, want 1", exitCode)
		}
		var payload contract.ProfileErrorPayload
		if err := json.Unmarshal([]byte(stderr), &payload); err != nil {
			t.Fatalf("decode structured profile env refusal: %v; stderr=%q", err, stderr)
		}
		if payload.Error.Code != "profile_secret_env_forbidden" ||
			!strings.Contains(payload.Error.Action, "--value-stdin") {
			t.Fatalf("profile env refusal = %#v", payload)
		}
	})

	t.Run("Should return the owned-work fallback warning after confirmed removal [IT-048][E2E-007]", func(t *testing.T) {
		t.Parallel()

		deletedRef := ""
		client := &profileAwareStubClient{
			stubClient: withWorkspaceResolution(&stubClient{deleteVaultSecretFn: func(
				_ context.Context,
				ref string,
			) error {
				deletedRef = ref
				return nil
			}}),
			profileClientStub: &profileClientStub{profiles: []contract.Profile{
				{Name: "default", State: "active"},
				{Name: "marketing", State: "active", WorkItems: 3},
			}},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"--profile", "marketing",
			"secret", "rm", "providers/openai/api_key",
			"--yes",
			"-o", "json",
		)
		if err != nil {
			t.Fatalf("secret rm profile override error = %v", err)
		}
		if deletedRef != "vault:profiles/marketing/providers/openai/api_key" {
			t.Fatalf("DeleteVaultSecret() ref = %q", deletedRef)
		}
		var record secretMutationRecord
		if err := json.Unmarshal([]byte(stdout), &record); err != nil {
			t.Fatalf("decode secret rm profile output: %v", err)
		}
		if record.Status != "removed" || record.Profile != "marketing" || len(record.Warnings) != 1 ||
			!strings.Contains(record.Warnings[0], "future runs fall back to the user key") {
			t.Fatalf("secret rm profile output = %#v, want owned-work fallback warning", record)
		}
	})
}
