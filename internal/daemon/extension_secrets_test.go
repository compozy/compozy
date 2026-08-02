package daemon

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
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
			req: contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"OTHER_KEY": {Value: new("value")},
			}},
			want: extensionpkg.ErrExtensionEnvBindingUndeclared,
		},
		{
			name: "Should reject an MCP Vault namespace",
			req: contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"API_KEY": {VaultRef: new("vault:mcp/global/linear/env/API_KEY")},
			}},
			want: extensionpkg.ErrExtensionEnvBindingInvalid,
		},
		{
			name: "Should reject a dangling extension Vault ref",
			req: contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"API_KEY": {VaultRef: new("vault:extensions/global/kit/env/missing")},
			}},
			want: extensionpkg.ErrExtensionEnvBindingDangling,
		},
		{
			name: "Should reject value and ref together",
			req: contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"API_KEY": {
					Value: new("value"), VaultRef: new("vault:extensions/global/kit/env/API_KEY"),
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
			_, err := service.SetSecrets(
				testutil.Context(t),
				extensionpkg.GlobalInstanceKey("kit"),
				testCase.req,
			)
			if !errors.Is(err, testCase.want) {
				t.Fatalf("SetSecrets() error = %v, want %v", err, testCase.want)
			}
			rows, listErr := bindings.ListEnvBindings(testutil.Context(t), "kit", "")
			if listErr != nil {
				t.Fatalf("ListEnvBindings() error = %v", listErr)
			}
			if len(rows) != 0 || len(secretVault.operations()) != 0 {
				t.Fatalf("validation mutated rows=%#v vault operations=%#v", rows, secretVault.operations())
			}
		})
	}
}

func TestExtensionSecretsRollbackUsesSortedForwardAndReverseOrder(t *testing.T) {
	t.Parallel()

	t.Run("Should restore prior values and bindings after the second sorted write fails", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"A_KEY", "B_KEY", "C_KEY"}, secretVault)
		key := extensionpkg.GlobalInstanceKey("kit")
		oldARef := vault.ExtensionSecretRef("kit", "", "A_KEY")
		secretVault.seed(oldARef, extensionpkg.ExtensionEnvBindingKind, "old-a")
		createdAt := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
		previous := extensionpkg.EnvBinding{
			ExtensionName: "kit", EnvName: "A_KEY", SecretRef: oldARef,
			Kind: extensionpkg.ExtensionEnvBindingKind, CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		if err := bindings.PutEnvBinding(testutil.Context(t), previous); err != nil {
			t.Fatalf("PutEnvBinding(previous) error = %v", err)
		}
		secretVault.failPutAt = 2
		_, err := service.SetSecrets(testutil.Context(t), key, contract.SetExtensionSecretsRequest{
			Secrets: map[string]contract.ExtensionSecretInput{
				"C_KEY": {Value: new("new-c")},
				"A_KEY": {Value: new("new-a")},
				"B_KEY": {Value: new("new-b")},
			},
		})
		if err == nil || !strings.Contains(err.Error(), "injected put failure") {
			t.Fatalf("SetSecrets() error = %v, want injected failure", err)
		}
		if got := secretVault.value(oldARef); got != "old-a" {
			t.Fatalf("restored A_KEY value = %q, want old-a", got)
		}
		rows, listErr := bindings.ListEnvBindings(testutil.Context(t), "kit", "")
		if listErr != nil {
			t.Fatalf("ListEnvBindings() error = %v", listErr)
		}
		if len(rows) != 1 || rows[0].EnvName != previous.EnvName || rows[0].SecretRef != previous.SecretRef ||
			!rows[0].CreatedAt.Equal(previous.CreatedAt) || !rows[0].UpdatedAt.Equal(previous.UpdatedAt) {
			t.Fatalf("bindings after rollback = %#v, want previous row", rows)
		}
		operations := secretVault.operations()
		if len(operations) < 3 || operations[0] != "put:"+oldARef ||
			operations[1] != "put:"+vault.ExtensionSecretRef("kit", "", "B_KEY") ||
			operations[len(operations)-1] != "put:"+oldARef {
			t.Fatalf("vault operations = %#v, want A then B failure then reverse A restore", operations)
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
			ref := vault.ExtensionSecretRef("kit", "", envName)
			secretVault.seed(ref, extensionpkg.ExtensionEnvBindingKind, "old-"+envName)
			if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
				ExtensionName: "kit", EnvName: envName, SecretRef: ref, Kind: extensionpkg.ExtensionEnvBindingKind,
			}); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", envName, err)
			}
		}
		_, err := service.SetSecrets(testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"),
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"A_KEY": {Value: new("new-a")},
			}})
		if err != nil {
			t.Fatalf("SetSecrets() error = %v", err)
		}
		if got := secretVault.value(vault.ExtensionSecretRef("kit", "", "B_KEY")); got != "old-B_KEY" {
			t.Fatalf("unrelated B_KEY value = %q, want preserved", got)
		}
	})

	t.Run("Should delete only unreferenced extension-owned superseded refs", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"A_KEY", "B_KEY"}, secretVault)
		oldRef := "vault:extensions/global/kit/env/shared-old"
		newRef := "vault:extensions/global/kit/env/new-a"
		secretVault.seed(oldRef, extensionpkg.ExtensionEnvBindingKind, "old")
		secretVault.seed(newRef, extensionpkg.ExtensionEnvBindingKind, "new")
		for _, envName := range []string{"A_KEY", "B_KEY"} {
			if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
				ExtensionName: "kit", EnvName: envName, SecretRef: oldRef, Kind: extensionpkg.ExtensionEnvBindingKind,
			}); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", envName, err)
			}
		}
		_, err := service.SetSecrets(testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"),
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"A_KEY": {VaultRef: &newRef},
			}})
		if err != nil {
			t.Fatalf("SetSecrets(first ref change) error = %v", err)
		}
		if got := secretVault.value(oldRef); got != "old" {
			t.Fatalf("shared old ref value = %q, want preserved", got)
		}
		_, err = service.SetSecrets(testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"),
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"B_KEY": {VaultRef: &newRef},
			}})
		if err != nil {
			t.Fatalf("SetSecrets(second ref change) error = %v", err)
		}
		if got := secretVault.value(oldRef); got != "" {
			t.Fatalf("unreferenced old ref value = %q, want deleted", got)
		}
	})

	t.Run("Should preserve an unreferenced superseded ref owned by another subsystem", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
		foreignRef := "vault:extensions/global/kit/env/foreign-owned"
		nextRef := "vault:extensions/global/kit/env/next"
		secretVault.seed(foreignRef, "foreign_token", "foreign-value")
		secretVault.seed(nextRef, extensionpkg.ExtensionEnvBindingKind, "next-value")
		if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
			ExtensionName: "kit",
			EnvName:       "API_KEY",
			SecretRef:     foreignRef,
			Kind:          extensionpkg.ExtensionEnvBindingKind,
		}); err != nil {
			t.Fatalf("PutEnvBinding(foreign ref) error = %v", err)
		}
		_, err := service.SetSecrets(
			testutil.Context(t),
			extensionpkg.GlobalInstanceKey("kit"),
			contract.SetExtensionSecretsRequest{Secrets: map[string]contract.ExtensionSecretInput{
				"API_KEY": {VaultRef: &nextRef},
			}},
		)
		if err != nil {
			t.Fatalf("SetSecrets(ref change) error = %v", err)
		}
		if got := secretVault.value(foreignRef); got != "foreign-value" {
			t.Fatalf("foreign ref value = %q, want preserved", got)
		}
	})

	t.Run("Should report stale rows without refs or values", func(t *testing.T) {
		t.Parallel()

		secretVault := newExtensionSecretVaultFake()
		service, bindings := newExtensionSecretsTestService(t, []string{"API_KEY"}, secretVault)
		if err := bindings.PutEnvBinding(testutil.Context(t), extensionpkg.EnvBinding{
			ExtensionName: "kit", EnvName: "OLD_KEY",
			SecretRef: "vault:extensions/global/kit/env/OLD_KEY", Kind: extensionpkg.ExtensionEnvBindingKind,
		}); err != nil {
			t.Fatalf("PutEnvBinding(stale) error = %v", err)
		}
		payload, err := service.ExtensionSecrets(testutil.Context(t), extensionpkg.GlobalInstanceKey("kit"))
		if err != nil {
			t.Fatalf("ExtensionSecrets() error = %v", err)
		}
		if !reflect.DeepEqual(payload.DeclaredEnv, []string{"API_KEY"}) ||
			!reflect.DeepEqual(payload.BoundEnvKeys, []string{"OLD_KEY"}) || len(payload.Bindings) != 1 ||
			payload.Bindings[0].EnvName != "OLD_KEY" || !payload.Bindings[0].Stale {
			t.Fatalf("ExtensionSecrets() = %#v, want stale OLD_KEY", payload)
		}
		encoded := fmt.Sprintf("%#v", payload)
		if strings.Contains(encoded, "vault:") || strings.Contains(encoded, "old-secret-value") {
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
				SecretRef: vault.ExtensionSecretRef("kit", "", "API_KEY"),
				Kind:      extensionpkg.ExtensionEnvBindingKind,
			},
			{
				ExtensionName: "kit", EnvName: "STALE_KEY",
				SecretRef: vault.ExtensionSecretRef("kit", "", "STALE_KEY"),
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

		payload, err := service.payloadFromExtension(testutil.Context(t), ext)
		if err != nil {
			t.Fatalf("payloadFromExtension() error = %v", err)
		}
		if !reflect.DeepEqual(payload.BoundEnvKeys, []string{"API_KEY", "STALE_KEY"}) {
			t.Fatalf("BoundEnvKeys = %#v, want active and stale rows", payload.BoundEnvKeys)
		}
		if !reflect.DeepEqual(payload.MissingEnv, []string{"PROCESS_MISSING"}) {
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
	service, ok := newDaemonExtensionService(
		registry,
		runtime,
		nil,
		nil,
		nil,
		nil,
		nil,
		testHomePaths(t),
		discardLogger(),
		func() time.Time { return time.Date(2026, 8, 2, 12, 30, 0, 0, time.UTC) },
		withDaemonExtensionSecrets(db.ExtensionEnvRepo, secretVault),
	).(*daemonExtensionService)
	if !ok {
		t.Fatal("newDaemonExtensionService() did not return daemonExtensionService")
	}
	return service, db.ExtensionEnvRepo
}

type extensionSecretVaultFake struct {
	mu        sync.Mutex
	values    map[string]string
	kinds     map[string]string
	ops       []string
	putCalls  int
	failPutAt int
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
	return vault.Metadata{Ref: ref, Kind: kind, Present: true}, nil
}

func (f *extensionSecretVaultFake) ResolveRef(_ context.Context, ref string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	value, ok := f.values[ref]
	if !ok {
		return "", vault.ErrSecretNotFound
	}
	return value, nil
}

func (f *extensionSecretVaultFake) GetMetadata(_ context.Context, ref string) (vault.Metadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.values[ref]
	if !ok {
		return vault.Metadata{}, vault.ErrSecretNotFound
	}
	return vault.Metadata{Ref: ref, Kind: f.kinds[ref], Present: true}, nil
}

func (f *extensionSecretVaultFake) DeleteSecret(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ops = append(f.ops, "delete:"+ref)
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
