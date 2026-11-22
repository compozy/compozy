package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/store"
)

func TestManagerResolveInstanceEnvMap(t *testing.T) {
	t.Parallel()

	t.Run("Should merge sources in precedence order and isolate instance bindings", func(t *testing.T) {
		t.Parallel()

		bindingStore := &envResolutionBindingStore{bindings: map[string][]EnvBinding{
			envResolutionBindingKey("demo", "", ""): {
				{ExtensionName: "demo", EnvName: "BOUND_ONLY", SecretRef: "vault:global-only"},
				{ExtensionName: "demo", EnvName: "SHARED", SecretRef: "vault:global-shared"},
			},
			envResolutionBindingKey("demo", "", "ws-1"): {
				{ExtensionName: "demo", WorkspaceID: "ws-1", EnvName: "BOUND_ONLY", SecretRef: "vault:workspace-only"},
				{ExtensionName: "demo", WorkspaceID: "ws-1", EnvName: "SHARED", SecretRef: "vault:workspace-shared"},
			},
			envResolutionBindingKey("demo", "profile-marketing", ""): {
				{
					ExtensionName: "demo",
					ProfileID:     "profile-marketing",
					EnvName:       "BOUND_ONLY",
					SecretRef:     "vault:marketing-only",
				},
				{
					ExtensionName: "demo",
					ProfileID:     "profile-marketing",
					EnvName:       "SHARED",
					SecretRef:     "vault:marketing-shared",
				},
			},
		}}
		manager := NewManager(nil,
			WithGetenv(func(key string) string {
				if key == "PATH" {
					return "/baseline/bin"
				}
				return ""
			}),
			WithEnvBindingStore(bindingStore),
			WithSecretResolver(envResolutionSecretResolver{values: map[string]string{
				"vault:authored":         "authored-secret",
				"vault:global-only":      "global-bound",
				"vault:global-shared":    "global-wins",
				"vault:workspace-only":   "workspace-bound",
				"vault:workspace-shared": "workspace-wins",
				"vault:marketing-only":   "marketing-bound",
				"vault:marketing-shared": "marketing-wins",
			}}),
		)

		resolve := func(t *testing.T, key InstanceKey) []string {
			t.Helper()
			env, cleanups, err := manager.resolveInstanceEnvMap(
				t.Context(),
				key,
				[]string{"SHARED", "BOUND_ONLY"},
				nil,
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

		wantEnv := func(shared, bound string) []string {
			return []string{
				"PATH=/manifest/bin",
				"HOME=",
				"USER=",
				"LOGNAME=",
				"TMPDIR=",
				"TMP=",
				"TEMP=",
				"LANG=",
				"LC_ALL=",
				"SHELL=",
				"SystemRoot=",
				"ComSpec=",
				"PATHEXT=",
				"USERPROFILE=",
				"SHARED=" + shared,
				"AUTHORED_ONLY=authored-secret",
				"BOUND_ONLY=" + bound,
			}
		}

		global := resolve(t, InstanceKey{Name: " demo "})
		wantGlobal := wantEnv("global-wins", "global-bound")
		if !slices.Equal(global, wantGlobal) {
			t.Fatalf("resolveInstanceEnvMap(global) = %#v, want %#v", global, wantGlobal)
		}

		workspace := resolve(t, InstanceKey{Name: "demo", WorkspaceID: " ws-1 "})
		wantWorkspace := wantEnv("workspace-wins", "workspace-bound")
		if !slices.Equal(workspace, wantWorkspace) {
			t.Fatalf("resolveInstanceEnvMap(workspace) = %#v, want %#v", workspace, wantWorkspace)
		}

		marketing := resolve(t, InstanceKey{
			Name: "demo", ProfileID: " profile-marketing ", WorkspaceID: " ws-1 ",
		})
		wantMarketing := wantEnv("marketing-wins", "marketing-bound")
		if !slices.Equal(marketing, wantMarketing) {
			t.Fatalf("resolveInstanceEnvMap(marketing) = %#v, want %#v", marketing, wantMarketing)
		}

		wantCalls := []string{
			envResolutionBindingKey("demo", store.DefaultProfileID, ""),
			envResolutionBindingKey("demo", store.DefaultProfileID, "ws-1"),
			envResolutionBindingKey("demo", "profile-marketing", "ws-1"),
		}
		if !slices.Equal(bindingStore.calls, wantCalls) {
			t.Fatalf("ResolveEnvBindings() calls = %#v, want %#v", bindingStore.calls, wantCalls)
		}
	})

	t.Run("Should pass set CA trust bundles and let the manifest override them", func(t *testing.T) {
		t.Parallel()

		manager := NewManager(nil, WithGetenv(func(key string) string {
			switch key {
			case "NODE_EXTRA_CA_CERTS":
				return "/etc/ssl/certs/ca-certificates.crt"
			case "SSL_CERT_FILE":
				return "/etc/ssl/daemon-ca.crt"
			default:
				return ""
			}
		}))
		env, cleanups, err := manager.resolveInstanceEnvMap(
			t.Context(),
			InstanceKey{Name: "demo"},
			nil,
			nil,
			t.TempDir(),
			map[string]string{"SSL_CERT_FILE": "/manifest/ca.crt"},
			nil,
		)
		if err != nil {
			t.Fatalf("resolveInstanceEnvMap() error = %v", err)
		}
		t.Cleanup(func() { runExtensionRedactionCleanups(cleanups) })
		for _, want := range []string{
			"NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt",
			"SSL_CERT_FILE=/manifest/ca.crt",
		} {
			if !slices.Contains(env, want) {
				t.Fatalf("resolveInstanceEnvMap() = %#v, missing %q", env, want)
			}
		}
		for _, entry := range env {
			if strings.HasPrefix(entry, "REQUESTS_CA_BUNDLE=") || strings.HasPrefix(entry, "SSL_CERT_DIR=") {
				t.Fatalf("resolveInstanceEnvMap() passed an unset CA variable as %q", entry)
			}
		}
	})

	t.Run("Should reject a dangling binding and unwind registered redactions", func(t *testing.T) {
		t.Parallel()

		secret := "dynamic-material-for-cleanup-839174"
		resolverErr := errors.New("secret material missing")
		manager := NewManager(nil,
			WithEnvBindingStore(&envResolutionBindingStore{bindings: map[string][]EnvBinding{
				envResolutionBindingKey("demo", "", ""): {
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
			t.Context(),
			InstanceKey{Name: "demo"},
			[]string{"B_BROKEN", "A_OK"},
			nil,
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
		if !strings.Contains(err.Error(), "B_BROKEN") || strings.Contains(err.Error(), "vault:missing") {
			t.Fatalf("resolveInstanceEnvMap() error = %q, want env name without Vault ref", err)
		}
		if got := diagnostics.Redact("resolved value " + secret); !strings.Contains(got, secret) {
			t.Fatalf("dynamic redaction remained registered after failure: %q", got)
		}
	})

	t.Run("Should exclude bindings dropped from the current declaration", func(t *testing.T) {
		t.Parallel()

		manager := NewManager(nil,
			WithEnvBindingStore(&envResolutionBindingStore{bindings: map[string][]EnvBinding{
				envResolutionBindingKey("demo", "", ""): {
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
			t.Context(),
			InstanceKey{Name: "demo"},
			[]string{"ACTIVE_KEY"},
			nil,
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

	t.Run("Should deliver subprocess inputs from bindings or defaults and never from the daemon", func(t *testing.T) {
		t.Parallel()

		const boundValue = "subprocess-bound-value-7f3a"
		manager := NewManager(nil,
			WithGetenv(func(key string) string {
				if key == "DAEMON_ONLY" {
					return "daemon-value"
				}
				return ""
			}),
			WithEnvBindingStore(&envResolutionBindingStore{bindings: map[string][]EnvBinding{
				envResolutionBindingKey("demo", "", ""): {
					{ExtensionName: "demo", EnvName: "BOUND", SecretRef: "vault:bound", InputID: "bound"},
					{ExtensionName: "demo", EnvName: "STALE_TYPE", SecretRef: "vault:stale", InputID: "stale"},
					{ExtensionName: "demo", EnvName: "SERVER_ONLY", SecretRef: "vault:server", MCPServer: "acme"},
				},
			}}),
			WithSecretResolver(envResolutionSecretResolver{values: map[string]string{
				"vault:bound":  boundValue,
				"vault:stale":  "not-a-boolean",
				"vault:server": "server-value",
			}}),
		)
		env, cleanups, err := manager.resolveInstanceEnvMap(
			t.Context(),
			InstanceKey{Name: "demo"},
			[]string{"BOUND", "DEFAULTED", "STALE_TYPE", "SERVER_ONLY", "DAEMON_ONLY"},
			[]launchInput{
				envLaunchInput("bound", "string", "BOUND", "", false),
				envLaunchInput("defaulted", "boolean", "DEFAULTED", "true", false),
				envLaunchInput("stale", "boolean", "STALE_TYPE", "", false),
				envLaunchInput("server", "string", "SERVER_ONLY", "", false),
				envLaunchInput("daemon", "string", "DAEMON_ONLY", "", false),
			},
			t.TempDir(),
			nil,
			nil,
		)
		if err != nil {
			t.Fatalf("resolveInstanceEnvMap() error = %v", err)
		}
		t.Cleanup(func() { runExtensionRedactionCleanups(cleanups) })
		decoded := envListToMap(t, env)
		if decoded["BOUND"] != boundValue || decoded["DEFAULTED"] != "true" {
			t.Fatalf("subprocess env = %#v, want bound value and default", decoded)
		}
		for _, name := range []string{"STALE_TYPE", "SERVER_ONLY", "DAEMON_ONLY"} {
			if _, exists := decoded[name]; exists {
				t.Fatalf("subprocess env = %#v, want no %s", decoded, name)
			}
		}
		if got := diagnostics.Redact("value " + boundValue); strings.Contains(got, boundValue) {
			t.Fatalf("bound value was not registered for redaction: %q", got)
		}
	})
}

func TestManagerResolveLaunchInputGate(t *testing.T) {
	t.Parallel()
	newManager := func(bindings []EnvBinding, values map[string]string) *Manager {
		return NewManager(nil,
			WithGetenv(func(string) string { return "" }),
			WithEnvBindingStore(&envResolutionBindingStore{bindings: map[string][]EnvBinding{
				envResolutionBindingKey("demo", "", ""): bindings,
			}}),
			WithSecretResolver(envResolutionSecretResolver{values: values}),
		)
	}
	staleBinding := []EnvBinding{{ExtensionName: "demo", EnvName: "FLAG", SecretRef: "vault:flag", InputID: "flag"}}
	staleValue := map[string]string{"vault:flag": "eu-west"}

	t.Run("Should omit an optional invalid binding instead of injecting its default [R-4]", func(t *testing.T) {
		t.Parallel()
		manager := newManager(staleBinding, staleValue)
		env, cleanups, err := manager.resolveInstanceEnvMap(t.Context(), InstanceKey{Name: "demo"},
			[]string{"FLAG"}, []launchInput{envLaunchInput("flag", "boolean", "FLAG", "true", false)},
			t.TempDir(), nil, nil)
		if err != nil {
			t.Fatalf("resolveInstanceEnvMap() error = %v", err)
		}
		t.Cleanup(func() { runExtensionRedactionCleanups(cleanups) })
		if value, exists := envListToMap(t, env)["FLAG"]; exists {
			t.Fatalf("FLAG = %q, want the optional invalid input omitted", value)
		}
	})

	t.Run("Should refuse start for a required invalid binding even with a default [R-7]", func(t *testing.T) {
		t.Parallel()
		manager := newManager(staleBinding, staleValue)
		required := envLaunchInput("flag", "boolean", "FLAG", "true", true)
		required.Required = true
		_, cleanups, err := manager.resolveInstanceEnvMap(t.Context(), InstanceKey{Name: "demo"},
			[]string{"FLAG"}, []launchInput{required}, t.TempDir(), nil, nil)
		refused, ok := errors.AsType[*InputsRequiredError](err)
		if !ok || !slices.Equal(refused.MissingInputs, []string{"flag"}) ||
			!slices.Equal(refused.InvalidInputs, []string{"flag"}) || cleanups != nil {
			t.Fatalf("resolveInstanceEnvMap() = %#v cleanups %d, want flag refused as invalid", err, len(cleanups))
		}
		if strings.Contains(err.Error(), "eu-west") {
			t.Fatal("launch refusal leaked the stored value")
		}
	})

	t.Run("Should refuse a missing required subprocess-only or server-consumed input [R-7]", func(t *testing.T) {
		t.Parallel()
		manager := newManager(nil, nil)
		missing := envLaunchInput("project", "identifier", "PROJECT_ID", "", false)
		missing.Required = true
		_, _, err := manager.resolveInstanceEnvMap(t.Context(), InstanceKey{Name: "demo"},
			[]string{"PROJECT_ID"}, []launchInput{missing}, t.TempDir(), nil, nil)
		refused, ok := errors.AsType[*InputsRequiredError](err)
		if !ok || !slices.Equal(refused.MissingInputs, []string{"project"}) || len(refused.InvalidInputs) != 0 {
			t.Fatalf("resolveInstanceEnvMap(missing) error = %#v, want project required", err)
		}
		served := envLaunchInput("region", "string", "REGION", "", true)
		served.Required = true
		_, _, err = manager.resolveInstanceEnvMap(t.Context(), InstanceKey{Name: "demo"},
			[]string{"REGION"}, []launchInput{served}, t.TempDir(), nil, nil)
		refused, ok = errors.AsType[*InputsRequiredError](err)
		if !ok || !slices.Equal(refused.MissingInputs, []string{"region"}) || len(refused.InvalidInputs) != 0 {
			t.Fatalf("resolveInstanceEnvMap(server-consumed) error = %#v, want region required", err)
		}
	})

	t.Run("Should start a required server-consumed input satisfied by daemon_env or its typed row", func(t *testing.T) {
		t.Parallel()
		served := envLaunchInput("region", "string", "REGION", "", true)
		served.Required = true
		daemonEnv := NewManager(nil,
			WithGetenv(func(name string) string {
				if name == "REGION" {
					return "eu-west"
				}
				return ""
			}),
			WithEnvBindingStore(&envResolutionBindingStore{}),
		)
		typedRow := NewManager(nil,
			WithGetenv(func(string) string { return "" }),
			WithEnvBindingStore(&envResolutionBindingStore{}),
			WithInputRowReader(envResolutionInputRows{
				"region": {Type: "string", Value: json.RawMessage(`"eu-west"`), Active: true},
			}),
		)
		for name, manager := range map[string]*Manager{"daemon_env": daemonEnv, "typed row": typedRow} {
			env, cleanups, err := manager.resolveInstanceEnvMap(t.Context(), InstanceKey{Name: "demo"},
				[]string{"REGION"}, []launchInput{served}, t.TempDir(), nil, nil)
			if err != nil {
				t.Fatalf("resolveInstanceEnvMap(%s) error = %v, want start", name, err)
			}
			runExtensionRedactionCleanups(cleanups)
			if value, exists := envListToMap(t, env)["REGION"]; exists {
				t.Fatalf("REGION = %q via %s, want the daemon-side value withheld from the subprocess", value, name)
			}
		}
	})

	t.Run("Should decide a dual-consumer input with the readiness precedence of setup", func(t *testing.T) {
		t.Parallel()
		manifest := &Manifest{
			Inputs: []ManifestInput{{
				ID: "region", Prompt: "Region", Type: "identifier", Required: true,
				Binding: marketplace.InputBinding{Type: "env", Name: "REGION"},
			}},
			DeclaredRequiresEnv: []string{"REGION"},
			Resources: ResourcesConfig{MCPServers: map[string]MCPServerConfig{
				"geo": {Command: "geo", Env: map[string]string{"REGION": "region"}},
			}},
		}
		manifest.RequiresEnv = manifestRequiredEnv(manifest.DeclaredRequiresEnv, manifest.Inputs)
		cases := []struct {
			name        string
			row         string
			daemonValue string
			wantState   string
		}{
			{name: "valid typed row over an invalid daemon value", row: `"eu-west"`, daemonValue: "eu west",
				wantState: "set"},
			{name: "invalid typed row over a valid daemon value", row: `"eu west"`, daemonValue: "eu-west",
				wantState: "invalid"},
			{name: "invalid daemon value without a row", daemonValue: "eu west", wantState: "invalid"},
		}
		for _, tc := range cases {
			t.Run("Should agree with setup on a "+tc.name, func(t *testing.T) {
				t.Parallel()
				rows := envResolutionInputRows{}
				state := InputState{Values: map[string]InputValueRecord{}}
				if tc.row != "" {
					rows["region"] = extensioninput.Record{
						Type:   "identifier",
						Value:  json.RawMessage(tc.row),
						Active: true,
					}
					state.Values["region"] = InputValueRecord{
						Type:   "identifier",
						Value:  json.RawMessage(tc.row),
						Active: true,
					}
				}
				getenv := func(name string) string {
					if name == "REGION" {
						return tc.daemonValue
					}
					return ""
				}
				if got := InputSetupState(manifest, manifest.Inputs[0], state, getenv); got != tc.wantState {
					t.Fatalf("InputSetupState(%s) = %q, want %q", tc.name, got, tc.wantState)
				}
				manager := NewManager(nil, WithGetenv(getenv), WithEnvBindingStore(&envResolutionBindingStore{}),
					WithInputRowReader(rows))
				env, cleanups, err := manager.resolveInstanceEnvMap(t.Context(), InstanceKey{Name: "demo"},
					manifest.RequiresEnv, manifest.launchInputs(), t.TempDir(), nil, nil)
				runExtensionRedactionCleanups(cleanups)
				if tc.wantState == "set" {
					if err != nil {
						t.Fatalf("resolveInstanceEnvMap(%s) error = %v, want start", tc.name, err)
					}
					if value, exists := envListToMap(t, env)["REGION"]; exists {
						t.Fatalf("REGION = %q via %s, want the daemon-side value withheld", value, tc.name)
					}
					return
				}
				refused, ok := errors.AsType[*InputsRequiredError](err)
				if !ok || !slices.Equal(refused.MissingInputs, []string{"region"}) ||
					!slices.Equal(refused.InvalidInputs, []string{"region"}) {
					t.Fatalf("resolveInstanceEnvMap(%s) error = %#v, want region invalid", tc.name, err)
				}
			})
		}
	})
}

type envResolutionInputRows map[string]extensioninput.Record

func (r envResolutionInputRows) List(
	context.Context,
	extensioninput.Instance,
) (map[string]extensioninput.Record, error) {
	return maps.Clone(r), nil
}

func envLaunchInput(id, inputType, name, inputDefault string, serverConsumed bool) launchInput {
	value := ManifestInput{ID: id, Type: inputType, Binding: marketplace.InputBinding{Type: "env", Name: name}}
	if inputDefault != "" {
		value.Default = json.RawMessage(inputDefault)
	}
	return launchInput{ManifestInput: value, serverConsumed: serverConsumed}
}

type envResolutionBindingStore struct {
	bindings map[string][]EnvBinding
	calls    []string
}

func (s *envResolutionBindingStore) ListEnvBindings(
	_ context.Context,
	extension string,
	profileID string,
	workspaceID string,
) ([]EnvBinding, error) {
	key := envResolutionBindingKey(extension, profileID, workspaceID)
	return slices.Clone(s.bindings[key]), nil
}

func (s *envResolutionBindingStore) ResolveEnvBindings(
	ctx context.Context,
	extension string,
	profileID string,
	workspaceID string,
) ([]EnvBinding, error) {
	s.calls = append(s.calls, envResolutionBindingKey(extension, profileID, workspaceID))
	precedence := [][2]string{
		{profileID, workspaceID},
		{profileID, ""},
		{"", workspaceID},
		{"", ""},
	}
	resolved := make(map[string]EnvBinding)
	for _, scope := range precedence {
		bindings, err := s.ListEnvBindings(ctx, extension, scope[0], scope[1])
		if err != nil {
			return nil, err
		}
		for _, binding := range bindings {
			if _, exists := resolved[binding.EnvName]; !exists {
				resolved[binding.EnvName] = binding
			}
		}
	}
	result := make([]EnvBinding, 0, len(resolved))
	for _, binding := range resolved {
		result = append(result, binding)
	}
	return result, nil
}

func envResolutionBindingKey(extension, profileID, workspaceID string) string {
	return strings.TrimSpace(extension) + "\x00" + strings.TrimSpace(profileID) + "\x00" +
		strings.TrimSpace(workspaceID)
}

func (*envResolutionBindingStore) PutEnvBinding(context.Context, EnvBinding) error { return nil }

func (*envResolutionBindingStore) DeleteEnvBinding(context.Context, string, string, string, string) error {
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
