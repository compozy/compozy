package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/vault"
)

func TestExtensionSecretsValidationIsMutationFree(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		req  contract.SetExtensionSecretsRequest
		want error
	}{
		{
			name: "Should reject undeclared environment names",
			req: contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "OTHER_KEY", Value: new("value")},
			}},
			want: extensionpkg.ErrExtensionEnvBindingUndeclared,
		},
		{
			name: "Should reject an MCP Vault namespace",
			req: contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "API_KEY", VaultRef: new("vault:mcp/global/linear/env/API_KEY")},
			}},
			want: extensionpkg.ErrExtensionEnvBindingInvalid,
		},
		{
			name: "Should reject a dangling extension Vault ref",
			req: contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "API_KEY", VaultRef: new("vault:extensions/global/kit/env/missing")},
			}},
			want: extensionpkg.ErrExtensionEnvBindingDangling,
		},
		{
			name: "Should reject value and ref together",
			req: contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "API_KEY",
					Value: new("value"), VaultRef: new("vault:extensions/global/kit/env/API_KEY"),
				},
			}},
			want: extensionpkg.ErrExtensionEnvBindingInvalid,
		},
		{
			name: "Should reject a partial remote header mapping",
			req: contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "API_KEY", Value: new("value"), MCPServer: "deployment-api"},
			}},
			want: extensionpkg.ErrExtensionEnvBindingInvalid,
		},
		{
			name: "Should reject an undeclared remote MCP server",
			req: contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{
					EnvName: "API_KEY", Value: new("value"),
					MCPServer: "deployment-api", HeaderName: "X-Deployment-Key",
				},
			}},
			want: extensionpkg.ErrExtensionEnvBindingInvalid,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			secretVault := newExtensionSecretVaultFake()
			service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
			_, err := service.setExtensionSecretsForProfileInstance(
				testutil.Context(t),
				extensionpkg.GlobalInstanceKey("kit"),
				store.DefaultProfileID,
				testCase.req,
			)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("setExtensionSecretsForProfileInstance() error = %v, want %v", err, testCase.want)
			}
			rows, listErr := bindings.ListEnvBindings(testutil.Context(t), "kit", store.DefaultProfileID, "")
			if listErr != nil {
				t.Fatalf("ListEnvBindings() error = %v", listErr)
			}
			if len(rows) != 0 || len(secretVault.operations()) != 0 {
				t.Fatalf("validation mutated rows=%#v vault operations=%#v", rows, secretVault.operations())
			}
		})
	}
}

func TestExtensionSecretsRemoteHeaderBinding(t *testing.T) {
	t.Parallel()

	t.Run("Should persist a profile-scoped remote header binding without exposing secret material", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"PROCESS_SECRET"}, secretVault)
		runtime, ok := service.runtime.(*fakeExtensionRuntime)
		if !ok {
			t.Fatal("extension runtime is not a fakeExtensionRuntime")
		}
		runtime.getExt.Manifest.Resources.MCPServers = map[string]extensionpkg.MCPServerConfig{
			"deployment-api": {
				Transport: "http", URL: "https://deployment.example.test/mcp",
				Headers: map[string]string{"X-Tenant": "acme"},
			},
		}

		payload, err := service.setExtensionSecretsForProfileInstance(
			testutil.Context(t),
			extensionpkg.GlobalInstanceKey("kit"),
			store.DefaultProfileID,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{{
				EnvName: "DEPLOYMENT_KEY", Value: new("secret-value"),
				MCPServer: "deployment-api", HeaderName: "X-Deployment-Key",
			}}},
		)
		if err != nil {
			t.Fatalf("setExtensionSecretsForProfileInstance() error = %v", err)
		}
		if len(payload.Bindings) != 1 || payload.Bindings[0].EnvName != "DEPLOYMENT_KEY" ||
			payload.Bindings[0].MCPServer != "deployment-api" ||
			payload.Bindings[0].HeaderName != "X-Deployment-Key" || payload.Bindings[0].Stale {
			t.Fatalf("remote binding payload = %#v, want presence-only current mapping", payload)
		}
		rows, err := bindings.ListEnvBindings(testutil.Context(t), "kit", store.DefaultProfileID, "")
		if err != nil {
			t.Fatalf("ListEnvBindings() error = %v", err)
		}
		if len(rows) != 1 || rows[0].MCPServer != "deployment-api" ||
			rows[0].HeaderName != "X-Deployment-Key" || rows[0].SecretRef == "" {
			t.Fatalf("remote binding rows = %#v, want persisted mapping and opaque ref", rows)
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(payload) error = %v", err)
		}
		if bytes.Contains(encoded, []byte("secret-value")) || bytes.Contains(encoded, []byte(rows[0].SecretRef)) {
			t.Fatalf("remote binding payload leaked secret material: %s", encoded)
		}
	})
}

func TestExtensionSecretsRollbackUsesSortedForwardAndReverseOrder(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should restore prior values and bindings in reverse order after the third sorted write fails",
		func(t *testing.T) {
			t.Parallel()

			secretVault := newExtensionSecretVaultFake()
			service, bindings := newExtensionSecretsTestService(t, []string{"A_KEY", "B_KEY", "C_KEY"}, secretVault)
			key := extensionpkg.GlobalInstanceKey("kit")
			createdAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
			previous := make([]extensionpkg.EnvBinding, 0, 2)
			for _, envName := range []string{"A_KEY", "B_KEY"} {
				ref := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", envName)
				secretVault.seed(ref, extensionpkg.ExtensionEnvBindingKind, "old-"+envName)
				binding := extensionpkg.EnvBinding{
					ExtensionName: "kit", ProfileID: store.DefaultProfileID, EnvName: envName, SecretRef: ref,
					Kind: extensionpkg.ExtensionEnvBindingKind, CreatedAt: createdAt, UpdatedAt: createdAt,
				}
				if err := bindings.PutEnvBinding(testutil.Context(t), binding); err != nil {
					t.Fatalf("PutEnvBinding(%s) error = %v", envName, err)
				}
				previous = append(previous, binding)
			}
			secretVault.failPutAt = 3
			_, err := service.setExtensionSecretsForProfileInstance(
				testutil.Context(t),
				key,
				store.DefaultProfileID,
				contract.SetExtensionSecretsRequest{
					Bindings: []contract.ExtensionSecretBindingInput{
						{EnvName: "C_KEY", Value: new("new-c")},
						{EnvName: "A_KEY", Value: new("new-a")},
						{EnvName: "B_KEY", Value: new("new-b")},
					},
				},
			)
			if err == nil || !strings.Contains(err.Error(), "injected put failure") {
				t.Fatalf("setExtensionSecretsForProfileInstance() error = %v, want injected failure", err)
			}
			for _, envName := range []string{"A_KEY", "B_KEY"} {
				ref := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", envName)
				if got := secretVault.value(ref); got != "old-"+envName {
					t.Fatalf("restored %s value = %q, want %q", envName, got, "old-"+envName)
				}
			}
			rows, listErr := bindings.ListEnvBindings(testutil.Context(t), "kit", store.DefaultProfileID, "")
			if listErr != nil {
				t.Fatalf("ListEnvBindings() error = %v", listErr)
			}
			if len(rows) != len(previous) {
				t.Fatalf("bindings after rollback = %#v, want %#v", rows, previous)
			}
			for index, binding := range rows {
				want := previous[index]
				if binding.EnvName != want.EnvName || binding.SecretRef != want.SecretRef ||
					!binding.CreatedAt.Equal(want.CreatedAt) || !binding.UpdatedAt.Equal(want.UpdatedAt) {
					t.Fatalf("binding[%d] after rollback = %#v, want %#v", index, binding, want)
				}
			}
			refA := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "A_KEY")
			refB := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "B_KEY")
			refC := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "C_KEY")
			wantOperations := []string{
				"put:" + refA,
				"put:" + refB,
				"put:" + refC,
				"delete:" + refC,
				"put:" + refB,
				"put:" + refA,
			}
			if operations := secretVault.operations(); !slices.Equal(operations, wantOperations) {
				t.Fatalf("vault operations = %#v, want %#v", operations, wantOperations)
			}
		},
	)

	t.Run("Should restore the previous value when a Vault write persists before reporting failure", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
		key := extensionpkg.GlobalInstanceKey("kit")
		ref := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "API_KEY")
		secretVault.seed(ref, extensionpkg.ExtensionEnvBindingKind, "old-value")
		previous := extensionpkg.EnvBinding{
			ExtensionName: "kit", ProfileID: store.DefaultProfileID, EnvName: "API_KEY", SecretRef: ref,
			Kind: extensionpkg.ExtensionEnvBindingKind,
		}
		if err := bindings.PutEnvBinding(testutil.Context(t), previous); err != nil {
			t.Fatalf("PutEnvBinding(previous) error = %v", err)
		}
		secretVault.failAfterPersistAt = 1
		_, err := service.setExtensionSecretsForProfileInstance(
			testutil.Context(t), key, store.DefaultProfileID, contract.SetExtensionSecretsRequest{
				Bindings: []contract.ExtensionSecretBindingInput{{EnvName: "API_KEY", Value: new("new-value")}},
			})
		if err == nil || !strings.Contains(err.Error(), "injected post-persist put failure") {
			t.Fatalf("setExtensionSecretsForProfileInstance() error = %v, want post-persist failure", err)
		}
		if got := secretVault.value(ref); got != "old-value" {
			t.Fatalf("secret after failed write = %q, want restored old-value", got)
		}
		rows, listErr := bindings.ListEnvBindings(testutil.Context(t), "kit", store.DefaultProfileID, "")
		if listErr != nil {
			t.Fatalf("ListEnvBindings() error = %v", listErr)
		}
		if len(rows) != 1 || rows[0].SecretRef != ref {
			t.Fatalf("bindings after failed write = %#v, want previous binding", rows)
		}
	})
}

func TestExtensionSecretsGarbageCollectionAndStaleProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve unrelated bindings during a partial update", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"A_KEY", "B_KEY"}, secretVault)
		for _, envName := range []string{"A_KEY", "B_KEY"} {
			ref := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", envName)
			secretVault.seed(ref, extensionpkg.ExtensionEnvBindingKind, "old-"+envName)
			if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
				ExtensionName: "kit", ProfileID: store.DefaultProfileID, EnvName: envName, SecretRef: ref,
				Kind: extensionpkg.ExtensionEnvBindingKind,
			}); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", envName, err)
			}
		}
		_, err := service.setExtensionSecretsForProfileInstance(
			testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"), store.DefaultProfileID,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "A_KEY", Value: new("new-a")},
			}})
		if err != nil {
			t.Fatalf("setExtensionSecretsForProfileInstance() error = %v", err)
		}
		if got := secretVault.value(
			vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "B_KEY"),
		); got != "old-B_KEY" {
			t.Fatalf("unrelated B_KEY value = %q, want preserved", got)
		}
	})

	t.Run("Should delete only unreferenced extension-owned superseded refs", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"A_KEY", "B_KEY"}, secretVault)
		oldRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "shared-old")
		newRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "new-a")
		secretVault.seed(oldRef, extensionpkg.ExtensionEnvBindingKind, "old")
		secretVault.seed(newRef, extensionpkg.ExtensionEnvBindingKind, "new")
		for _, envName := range []string{"A_KEY", "B_KEY"} {
			if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
				ExtensionName: "kit", ProfileID: store.DefaultProfileID, EnvName: envName, SecretRef: oldRef,
				Kind: extensionpkg.ExtensionEnvBindingKind,
			}); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", envName, err)
			}
		}
		_, err := service.setExtensionSecretsForProfileInstance(
			testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"), store.DefaultProfileID,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "A_KEY", VaultRef: &newRef},
			}})
		if err != nil {
			t.Fatalf("setExtensionSecretsForProfileInstance(first ref change) error = %v", err)
		}
		if got := secretVault.value(oldRef); got != "old" {
			t.Fatalf("shared old ref value = %q, want preserved", got)
		}
		_, err = service.setExtensionSecretsForProfileInstance(
			testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"), store.DefaultProfileID,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "B_KEY", VaultRef: &newRef},
			}})
		if err != nil {
			t.Fatalf("setExtensionSecretsForProfileInstance(second ref change) error = %v", err)
		}
		if got := secretVault.value(oldRef); got != "" {
			t.Fatalf("unreferenced old ref value = %q, want deleted", got)
		}
	})

	t.Run("Should preserve an unreferenced superseded ref owned by another subsystem", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
		foreignRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "foreign-owned")
		nextRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "next")
		secretVault.seed(foreignRef, "foreign_token", "foreign-value")
		secretVault.seed(nextRef, extensionpkg.ExtensionEnvBindingKind, "next-value")
		if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
			ExtensionName: "kit",
			ProfileID:     store.DefaultProfileID,
			EnvName:       "API_KEY",
			SecretRef:     foreignRef,
			Kind:          extensionpkg.ExtensionEnvBindingKind,
		}); err != nil {
			t.Fatalf("PutEnvBinding(foreign ref) error = %v", err)
		}
		_, err := service.setExtensionSecretsForProfileInstance(
			testutil.Context(t),
			extensionpkg.GlobalInstanceKey("kit"),
			store.DefaultProfileID,
			contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
				{EnvName: "API_KEY", VaultRef: &nextRef},
			}},
		)
		if err != nil {
			t.Fatalf("setExtensionSecretsForProfileInstance(ref change) error = %v", err)
		}
		if got := secretVault.value(foreignRef); got != "foreign-value" {
			t.Fatalf("foreign ref value = %q, want preserved", got)
		}
	})

	t.Run("Should delete an unreferenced owned secret without resolving its plaintext", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
		ref := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "API_KEY")
		secretVault.seed(ref, extensionpkg.ExtensionEnvBindingKind, "secret-value")
		if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
			ExtensionName: "kit", ProfileID: store.DefaultProfileID, EnvName: "API_KEY", SecretRef: ref,
			Kind: extensionpkg.ExtensionEnvBindingKind,
		}); err != nil {
			t.Fatalf("PutEnvBinding() error = %v", err)
		}
		secretVault.failResolveAt = 1

		if err := service.deleteExtensionSecretForProfileInstance(
			testutil.Context(t),
			extensionpkg.GlobalInstanceKey("kit"),
			store.DefaultProfileID,
			"API_KEY",
		); err != nil {
			t.Fatalf("deleteExtensionSecretForInstance() error = %v", err)
		}
		if got := secretVault.resolveCallCount(); got != 0 {
			t.Fatalf("ResolveRef() calls = %d, want 0", got)
		}
		if got := secretVault.value(ref); got != "" {
			t.Fatalf("deleted secret value = %q, want absent", got)
		}
		rows, err := bindings.ListEnvBindings(testutil.Context(t), "kit", store.DefaultProfileID, "")
		if err != nil {
			t.Fatalf("ListEnvBindings() error = %v", err)
		}
		if len(rows) != 0 {
			t.Fatalf("bindings after delete = %#v, want none", rows)
		}
	})

	t.Run("Should restore prior deletions and preserve primary and rollback failures", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			name       string
			primaryErr error
			configure  func(*extensionSecretVaultFake)
		}{
			{
				name:       "Should restore when metadata inspection fails after one delete",
				primaryErr: errInjectedSecretMetadata,
				configure:  func(vault *extensionSecretVaultFake) { vault.failMetadataAt = 4 },
			},
			{
				name:       "Should restore when plaintext snapshot fails after one delete",
				primaryErr: errInjectedSecretResolve,
				configure:  func(vault *extensionSecretVaultFake) { vault.failResolveAt = 2 },
			},
			{
				name:       "Should restore when the second delete fails",
				primaryErr: errInjectedSecretDelete,
				configure:  func(vault *extensionSecretVaultFake) { vault.failDeleteAt = 2 },
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				secretVault := newExtensionSecretVaultFake()
				service, bindings := newExtensionSecretsTestService(
					t,
					[]string{"A_KEY", "B_KEY"},
					secretVault,
				)
				oldARef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "a-old")
				oldBRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "b-old")
				newARef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "a-new")
				newBRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "b-new")
				for _, secret := range []struct {
					envName  string
					oldRef   string
					oldValue string
					newRef   string
					newValue string
				}{
					{envName: "A_KEY", oldRef: oldARef, oldValue: "old-a", newRef: newARef, newValue: "new-a"},
					{envName: "B_KEY", oldRef: oldBRef, oldValue: "old-b", newRef: newBRef, newValue: "new-b"},
				} {
					secretVault.seed(secret.oldRef, extensionpkg.ExtensionEnvBindingKind, secret.oldValue)
					secretVault.seed(secret.newRef, extensionpkg.ExtensionEnvBindingKind, secret.newValue)
					if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
						ExtensionName: "kit", ProfileID: store.DefaultProfileID,
						EnvName: secret.envName, SecretRef: secret.oldRef,
						Kind: extensionpkg.ExtensionEnvBindingKind,
					}); err != nil {
						t.Fatalf("PutEnvBinding(%s) error = %v", secret.envName, err)
					}
				}
				testCase.configure(secretVault)
				secretVault.failAfterPersistAt = 1

				_, err := service.setExtensionSecretsForProfileInstance(
					testutil.Context(t),
					extensionpkg.GlobalInstanceKey("kit"),
					store.DefaultProfileID,
					contract.SetExtensionSecretsRequest{Bindings: []contract.ExtensionSecretBindingInput{
						{EnvName: "A_KEY", VaultRef: &newARef},
						{EnvName: "B_KEY", VaultRef: &newBRef},
					}},
				)
				if !errors.Is(err, testCase.primaryErr) {
					t.Fatalf(
						"setExtensionSecretsForProfileInstance() error = %v, want primary %v",
						err,
						testCase.primaryErr,
					)
				}
				if !errors.Is(err, errInjectedSecretPostPersistPut) {
					t.Fatalf(
						"setExtensionSecretsForProfileInstance() error = %v, want rollback %v",
						err,
						errInjectedSecretPostPersistPut,
					)
				}
				if got := secretVault.value(oldARef); got != "old-a" {
					t.Fatalf("restored first secret = %q, want old-a", got)
				}
				if got := secretVault.value(oldBRef); got != "old-b" {
					t.Fatalf("preserved second secret = %q, want old-b", got)
				}
				rows, listErr := bindings.ListEnvBindings(testutil.Context(t), "kit", store.DefaultProfileID, "")
				if listErr != nil {
					t.Fatalf("ListEnvBindings() error = %v", listErr)
				}
				if len(rows) != 2 || rows[0].SecretRef != oldARef || rows[1].SecretRef != oldBRef {
					t.Fatalf("bindings after failed GC = %#v, want prior refs", rows)
				}
			})
		}
	})

	t.Run("Should report stale rows without refs or values", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
		staleRef := vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "OLD_KEY")
		const secretSentinel = "extension-secret-leak-sentinel"
		secretVault.seed(staleRef, extensionpkg.ExtensionEnvBindingKind, secretSentinel)
		if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
			ExtensionName: "kit", ProfileID: store.DefaultProfileID, EnvName: "OLD_KEY",
			SecretRef: staleRef, Kind: extensionpkg.ExtensionEnvBindingKind,
		}); err != nil {
			t.Fatalf("PutEnvBinding(stale) error = %v", err)
		}
		payload, err := service.extensionSecretsForProfileInstance(
			testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"), store.DefaultProfileID,
		)
		if err != nil {
			t.Fatalf("extensionSecretsForInstance() error = %v", err)
		}
		if !slices.Equal(payload.DeclaredEnv, []string{"API_KEY"}) ||
			!slices.Equal(payload.BoundEnvKeys, []string{"OLD_KEY"}) || len(payload.Bindings) != 1 ||
			payload.Bindings[0].EnvName != "OLD_KEY" || !payload.Bindings[0].Stale {
			t.Fatalf("extensionSecretsForInstance() = %#v, want stale OLD_KEY", payload)
		}
		encoded, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			t.Fatalf("json.Marshal(payload) error = %v", marshalErr)
		}
		if bytes.Contains(encoded, []byte(staleRef)) || bytes.Contains(encoded, []byte(secretSentinel)) {
			t.Fatalf("presence-only payload leaked secret material: %s", encoded)
		}
	})

	t.Run("Should exclude active bindings but not stale rows from missing environment math", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(
			t,
			[]string{"API_KEY", "PROCESS_MISSING"},
			secretVault,
		)
		for _, binding := range []extensionpkg.EnvBinding{
			{
				ExtensionName: "kit", EnvName: "API_KEY",
				ProfileID: store.DefaultProfileID,
				SecretRef: vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "API_KEY"),
				Kind:      extensionpkg.ExtensionEnvBindingKind,
			},
			{
				ExtensionName: "kit", EnvName: "STALE_KEY",
				ProfileID: store.DefaultProfileID,
				SecretRef: vault.ExtensionProfileSecretRef("kit", store.DefaultProfileID, "", "STALE_KEY"),
				Kind:      extensionpkg.ExtensionEnvBindingKind,
			},
		} {
			if err := bindings.PutEnvBinding(testutil.Context(t), binding); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", binding.EnvName, err)
			}
		}
		ext := &extensionpkg.Extension{
			Info: extensionpkg.ExtensionInfo{Name: "kit", Version: "1.0.0", Enabled: true},
			Manifest: &extensionpkg.Manifest{
				Name: "kit", Version: "1.0.0", RequiresEnv: []string{"API_KEY", "PROCESS_MISSING"},
			},
			Status: extensionpkg.ExtensionStatus{
				Name: "kit", Version: "1.0.0", Enabled: true, Registered: true,
				MissingEnvChecked: true, MissingEnv: []string{"API_KEY", "PROCESS_MISSING"},
			},
		}

		payload, err := service.payloadFromExtension(testutil.Context(t), ext, extensionDefaultProfileLens())
		if err != nil {
			t.Fatalf("payloadFromExtension() error = %v", err)
		}
		if !slices.Equal(payload.BoundEnvKeys, []string{"API_KEY", "STALE_KEY"}) {
			t.Fatalf("BoundEnvKeys = %#v, want active and stale rows", payload.BoundEnvKeys)
		}
		if !slices.Equal(payload.MissingEnv, []string{"PROCESS_MISSING"}) {
			t.Fatalf("MissingEnv = %#v, want only unbound declared env", payload.MissingEnv)
		}
	})
}

func newExtensionSecretsTestService(
	t *testing.T,
	declared []string,
	secretVault *extensionSecretVaultFake,
) (*daemonExtensionService, *globaldb.ExtensionEnvRepo) {
	t.Helper()

	db := openDaemonTestGlobalDB(t)
	registry := extensionpkg.NewRegistry(db.DB())
	runtime := &fakeExtensionRuntime{getExt: &extensionpkg.Extension{
		Info:     extensionpkg.ExtensionInfo{Name: "kit", Version: "1.0.0", Enabled: true},
		Manifest: &extensionpkg.Manifest{Name: "kit", Version: "1.0.0", RequiresEnv: slices.Clone(declared)},
		Status:   extensionpkg.ExtensionStatus{Name: "kit", Version: "1.0.0", Enabled: true, Registered: true},
	}}
	service, ok := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry:  registry,
		Runtime:   runtime,
		HomePaths: testHomePaths(t),
		Logger:    discardLogger(),
		Now:       func() time.Time { return time.Date(2026, 8, 2, 12, 30, 0, 0, time.UTC) },
	},
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, secretVault),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	return service, db.ExtensionEnvRepo
}

var (
	errInjectedSecretMetadata       = errors.New("injected metadata failure")
	errInjectedSecretResolve        = errors.New("injected resolve failure")
	errInjectedSecretDelete         = errors.New("injected delete failure")
	errInjectedSecretPostPersistPut = errors.New("injected post-persist put failure")
)

type extensionSecretVaultFake struct {
	mu                 sync.Mutex
	values             map[string]string
	kinds              map[string]string
	ops                []string
	putCalls           int
	metadataCalls      int
	resolveCalls       int
	deleteCalls        int
	failPutAt          int
	failAfterPersistAt int
	failMetadataAt     int
	failResolveAt      int
	failDeleteAt       int
}

func newExtensionSecretVaultFake() *extensionSecretVaultFake {
	return &extensionSecretVaultFake{values: make(map[string]string), kinds: make(map[string]string)}
}

func (f *extensionSecretVaultFake) seed(ref, kind, value string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[ref] = value
	f.kinds[ref] = kind
}

func (f *extensionSecretVaultFake) PutSecret(
	_ context.Context,
	ref string,
	kind string,
	value string,
) (vault.Metadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.putCalls++
	f.ops = append(f.ops, "put:"+ref)
	if f.failPutAt > 0 && f.putCalls == f.failPutAt {
		return vault.Metadata{}, errors.New("injected put failure")
	}
	f.values[ref] = value
	f.kinds[ref] = kind
	if f.failAfterPersistAt > 0 && f.putCalls == f.failAfterPersistAt {
		return vault.Metadata{}, errInjectedSecretPostPersistPut
	}
	return vault.Metadata{Ref: ref, Kind: kind, Present: true}, nil
}

func (f *extensionSecretVaultFake) ResolveRef(_ context.Context, ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolveCalls++
	if f.failResolveAt > 0 && f.resolveCalls == f.failResolveAt {
		return "", errInjectedSecretResolve
	}
	value, ok := f.values[ref]
	if !ok {
		return "", vault.ErrSecretNotFound
	}
	return value, nil
}

func (f *extensionSecretVaultFake) GetMetadata(_ context.Context, ref string) (vault.Metadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.metadataCalls++
	if f.failMetadataAt > 0 && f.metadataCalls == f.failMetadataAt {
		return vault.Metadata{}, errInjectedSecretMetadata
	}
	_, ok := f.values[ref]
	if !ok {
		return vault.Metadata{}, vault.ErrSecretNotFound
	}
	return vault.Metadata{Ref: ref, Kind: f.kinds[ref], Present: true}, nil
}

func (f *extensionSecretVaultFake) DeleteSecret(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls++
	f.ops = append(f.ops, "delete:"+ref)
	if f.failDeleteAt > 0 && f.deleteCalls == f.failDeleteAt {
		return errInjectedSecretDelete
	}
	if _, ok := f.values[ref]; !ok {
		return vault.ErrSecretNotFound
	}
	delete(f.values, ref)
	delete(f.kinds, ref)
	return nil
}

func (f *extensionSecretVaultFake) operations() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.ops)
}

func (f *extensionSecretVaultFake) value(ref string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.values[ref]
}

func (f *extensionSecretVaultFake) resolveCallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resolveCalls
}
