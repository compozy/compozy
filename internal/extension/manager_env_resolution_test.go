package extensionpkg

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/diagnostics"
)

func TestManagerResolveInstanceEnvMap(t *testing.T) {
	t.Parallel()

	t.Run("Should merge sources in precedence order and isolate instance bindings", func(t *testing.T) {
		t.Parallel()

		store := &envResolutionBindingStore{bindings: map[string][]EnvBinding{
			"demo\x00": {
				{ExtensionName: "demo", EnvName: "BOUND_ONLY", SecretRef: "vault:global-only"},
				{ExtensionName: "demo", EnvName: "SHARED", SecretRef: "vault:global-shared"},
			},
			"demo\x00ws-1": {
				{ExtensionName: "demo", WorkspaceID: "ws-1", EnvName: "BOUND_ONLY", SecretRef: "vault:workspace-only"},
				{ExtensionName: "demo", WorkspaceID: "ws-1", EnvName: "SHARED", SecretRef: "vault:workspace-shared"},
			},
		}}
		manager := NewManager(nil,
			WithGetenv(func(key string) string {
				if key == "PATH" {
					return "/baseline/bin"
				}
				return ""
			}),
			WithEnvBindingStore(store),
			WithSecretResolver(envResolutionSecretResolver{values: map[string]string{
				"vault:authored":         "authored-secret",
				"vault:global-only":      "global-bound",
				"vault:global-shared":    "global-wins",
				"vault:workspace-only":   "workspace-bound",
				"vault:workspace-shared": "workspace-wins",
			}}),
		)

		resolve := func(t *testing.T, key InstanceKey) []string {
			t.Helper()
			env, cleanups, err := manager.resolveInstanceEnvMap(
				context.Background(),
				key,
				[]string{"SHARED", "BOUND_ONLY"},
				t.TempDir(),
				map[string]string{"PATH": "/manifest/bin", "SHARED": "manifest"},
				map[string]string{"AUTHORED_ONLY": "vault:authored", "SHARED": "vault:authored"},
			)
			if err != nil {
				t.Fatalf("resolveInstanceEnvMap() error = %v", err)
			}
			t.Cleanup(func() { runExtensionRedactionCleanups(cleanups) })
			return env
		}

		global := resolve(t, InstanceKey{Name: " demo "})
		globalAgain := resolve(t, InstanceKey{Name: "demo"})
		if !reflect.DeepEqual(global, globalAgain) {
			t.Fatalf("resolveInstanceEnvMap() ordering changed: first=%#v second=%#v", global, globalAgain)
		}
		globalMap := envListToMap(t, global)
		assertEnvResolutionValues(t, globalMap, "/manifest/bin", "authored-secret", "global-bound", "global-wins")

		workspace := resolve(t, InstanceKey{Name: "demo", WorkspaceID: " ws-1 "})
		workspaceMap := envListToMap(t, workspace)
		assertEnvResolutionValues(
			t,
			workspaceMap,
			"/manifest/bin",
			"authored-secret",
			"workspace-bound",
			"workspace-wins",
		)

		wantCalls := []string{"demo\x00", "demo\x00", "demo\x00ws-1"}
		if !reflect.DeepEqual(store.calls, wantCalls) {
			t.Fatalf("ListEnvBindings() calls = %#v, want %#v", store.calls, wantCalls)
		}
	})

	t.Run("Should reject a dangling binding and unwind registered redactions", func(t *testing.T) {
		t.Parallel()

		secret := "dynamic-material-for-cleanup-839174"
		resolverErr := errors.New("secret material missing")
		manager := NewManager(nil,
			WithEnvBindingStore(&envResolutionBindingStore{bindings: map[string][]EnvBinding{
				"demo\x00": {
					{ExtensionName: "demo", EnvName: "A_OK", SecretRef: "vault:first"},
					{ExtensionName: "demo", EnvName: "B_BROKEN", SecretRef: "vault:missing"},
				},
			}}),
			WithSecretResolver(envResolutionSecretResolver{
				values: map[string]string{"vault:first": secret},
				errors: map[string]error{"vault:missing": resolverErr},
			}),
		)

		_, cleanups, err := manager.resolveInstanceEnvMap(
			context.Background(),
			InstanceKey{Name: "demo"},
			[]string{"B_BROKEN", "A_OK"},
			t.TempDir(),
			nil,
			nil,
		)
		if !errors.Is(err, resolverErr) {
			t.Fatalf("resolveInstanceEnvMap() error = %v, want %v", err, resolverErr)
		}
		if cleanups != nil {
			t.Fatalf("resolveInstanceEnvMap() cleanups = %#v, want nil after unwind", cleanups)
		}
		if !strings.Contains(err.Error(), "B_BROKEN") || !strings.Contains(err.Error(), "vault:missing") {
			t.Fatalf("resolveInstanceEnvMap() error = %q, want env name and ref", err)
		}
		if got := diagnostics.Redact("resolved value " + secret); !strings.Contains(got, secret) {
			t.Fatalf("dynamic redaction remained registered after failure: %q", got)
		}
	})

	t.Run("Should exclude bindings dropped from the current declaration", func(t *testing.T) {
		t.Parallel()

		manager := NewManager(nil,
			WithEnvBindingStore(&envResolutionBindingStore{bindings: map[string][]EnvBinding{
				"demo\x00": {
					{ExtensionName: "demo", EnvName: "ACTIVE_KEY", SecretRef: "vault:active"},
					{ExtensionName: "demo", EnvName: "STALE_KEY", SecretRef: "vault:stale"},
				},
			}}),
			WithSecretResolver(envResolutionSecretResolver{values: map[string]string{
				"vault:active": "active-value",
				"vault:stale":  "stale-value",
			}}),
		)

		env, cleanups, err := manager.resolveInstanceEnvMap(
			context.Background(),
			InstanceKey{Name: "demo"},
			[]string{"ACTIVE_KEY"},
			t.TempDir(),
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("resolveInstanceEnvMap() error = %v", err)
		}
		t.Cleanup(func() { runExtensionRedactionCleanups(cleanups) })
		decoded := envListToMap(t, env)
		if got := decoded["ACTIVE_KEY"]; got != "active-value" {
			t.Fatalf("ACTIVE_KEY = %q, want active-value", got)
		}
		if _, exists := decoded["STALE_KEY"]; exists {
			t.Fatalf("resolveInstanceEnvMap() injected stale binding: %#v", decoded)
		}
	})
}

func assertEnvResolutionValues(
	t *testing.T,
	got map[string]string,
	path string,
	authored string,
	boundOnly string,
	shared string,
) {
	t.Helper()
	want := map[string]string{
		"PATH":          path,
		"AUTHORED_ONLY": authored,
		"BOUND_ONLY":    boundOnly,
		"SHARED":        shared,
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("%s = %q, want %q (env=%#v)", key, got[key], value, got)
		}
	}
	if !slices.IsSortedFunc(envResolutionCustomKeys(got), strings.Compare) {
		t.Fatalf("custom env keys are not deterministic: %#v", got)
	}
}

func envResolutionCustomKeys(env map[string]string) []string {
	baseline := make(map[string]struct{}, len(safeSubprocessEnvKeys))
	for _, key := range safeSubprocessEnvKeys {
		baseline[key] = struct{}{}
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		if _, exists := baseline[key]; !exists {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

type envResolutionBindingStore struct {
	bindings map[string][]EnvBinding
	calls    []string
}

func (s *envResolutionBindingStore) ListEnvBindings(
	_ context.Context,
	extension string,
	workspaceID string,
) ([]EnvBinding, error) {
	key := strings.TrimSpace(extension) + "\x00" + strings.TrimSpace(workspaceID)
	s.calls = append(s.calls, key)
	return slices.Clone(s.bindings[key]), nil
}

func (*envResolutionBindingStore) PutEnvBinding(context.Context, EnvBinding) error { return nil }

func (*envResolutionBindingStore) DeleteEnvBinding(context.Context, string, string, string) error {
	return nil
}

type envResolutionSecretResolver struct {
	values map[string]string
	errors map[string]error
}

func (r envResolutionSecretResolver) ResolveRef(_ context.Context, ref string) (string, error) {
	if err := r.errors[ref]; err != nil {
		return "", err
	}
	return r.values[ref], nil
}
