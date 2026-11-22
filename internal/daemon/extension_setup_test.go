package daemon

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	builtintools "github.com/compozy/compozy/internal/tools/builtin"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	setupKitName       = "setup-kit"
	setupWorkspaceID   = "ws-setup"
	setupProjectValue  = "acme-project-41"
	setupTokenValue    = "setup-token-secret"
	setupDaemonRegion  = "eu-daemon"
	setupManifestInput = `name = "setup-kit"
version = "1.0.0"
description = "Setup fixture"
min_compozy_version = "0.7.9"
requires_env = ["PROJECT_ID", "REGION", "API_TOKEN"]

[[inputs]]
id = "project"
prompt = "Project"
type = "identifier"
required = true
binding = { type = "env", name = "PROJECT_ID" }

[[inputs]]
id = "region"
prompt = "Region"
type = "string"
required = true
binding = { type = "env", name = "REGION" }

[[inputs]]
id = "token"
prompt = "Token"
type = "secret"
required = false
binding = { type = "env", name = "API_TOKEN" }

[resources.mcp_servers.server]
command = "server"
env = { REGION = "region" }
`
)

type extensionSetupFixture struct {
	service  *daemonExtensionService
	bindings *globaldb.ExtensionEnvRepo
	runtime  *fakeExtensionRuntime
}

// newExtensionSetupFixture installs setup-kit globally and enabled, with REGION only in the daemon
// environment so the dual-consumer region input reads daemon_env.
func newExtensionSetupFixture(t *testing.T) extensionSetupFixture {
	t.Helper()
	service, bindings := newExtensionSecretsTestService(t, nil, newExtensionSecretVaultFake())
	service.getenv = func(name string) string {
		if name == "REGION" {
			return setupDaemonRegion
		}
		return ""
	}
	withDaemonExtensionWorkspaceResolver(&daemonExtensionWorkspaceResolverStub{
		resolved: workspacepkg.ResolvedWorkspace{
			Workspace: workspacepkg.Workspace{ID: setupWorkspaceID}, WorkspaceID: setupWorkspaceID,
		},
	})(service)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(setupManifestInput), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.registry.Install(
		manifest, dir, checksum, extensionpkg.WithInstallScope(extensionpkg.InstallationScope{}),
	); err != nil {
		t.Fatalf("Registry.Install() error = %v", err)
	}
	runtime, ok := service.runtime.(*fakeExtensionRuntime)
	if !ok {
		t.Fatal("setup fixture runtime is not the fake runtime")
	}
	return extensionSetupFixture{service: service, bindings: bindings, runtime: runtime}
}

func extensionSetupOperator(t *testing.T) taskpkg.ActorContext {
	t.Helper()
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindHTTP, "extension setup test")
	if err != nil {
		t.Fatal(err)
	}
	actor.ReadScope = store.ReadScope{ProfileID: store.DefaultProfileID}
	return actor
}

func setupInputStates(payload contract.ExtensionInputsPayload) map[string]string {
	states := make(map[string]string, len(payload.Inputs))
	for _, input := range payload.Inputs {
		states[input.ID] = input.State
	}
	return states
}

func setupValues(pairs ...string) map[string]extensioninput.Value {
	values := make(map[string]extensioninput.Value, len(pairs)/2)
	for index := 0; index+1 < len(pairs); index += 2 {
		encoded, _ := json.Marshal(pairs[index+1])
		values[pairs[index]] = extensioninput.Value{Value: encoded}
	}
	return values
}

// Invariant: setup reads report per-input state without values and writes merge into one instance,
// reloading it once per changing write. Owner: daemon setup service; suite: TestExtensionInputSetup.
func TestExtensionInputSetup(t *testing.T) {
	t.Parallel()
	target := extensionInputsTarget{Name: setupKitName}

	t.Run("Should report state, targets and daemon_env without values [T-08]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t)
		ctx := testutil.Context(t)
		payload, err := fixture.service.InputsState(ctx, target, extensionSetupOperator(t))
		if err != nil {
			t.Fatalf("InputsState() error = %v", err)
		}
		states := setupInputStates(payload)
		if states["project"] != contract.ExtensionInputStateMissing ||
			states["region"] != contract.ExtensionInputStateDaemonEnv ||
			states["token"] != contract.ExtensionInputStateMissing {
			t.Fatalf("states = %#v", states)
		}
		if !slices.Equal(payload.Missing, []string{"project"}) || !slices.Equal(payload.DaemonEnv, []string{"region"}) ||
			len(payload.Invalid) != 0 || len(payload.MissingEnv) != 0 ||
			!payload.Enabled ||
			payload.Extension != setupKitName ||
			payload.Profile != daemonDefaultProfileName {
			t.Fatalf("payload = %#v", payload)
		}
		for _, input := range payload.Inputs {
			want := []string{"subprocess"}
			if input.ID == "region" {
				want = append(want, "mcp:server")
			}
			if !slices.Equal(input.Targets, want) {
				t.Fatalf("%s targets = %#v, want %#v", input.ID, input.Targets, want)
			}
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), setupDaemonRegion) {
			t.Fatal("setup read leaked the daemon environment value")
		}

		_, err = fixture.service.InputsState(ctx, extensionInputsTarget{Name: "missing-kit"}, extensionSetupOperator(t))
		if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("InputsState(uninstalled) error = %v, want extension not found", err)
		}
	})

	t.Run("Should merge writes, reload once per change and keep the installation intact [T-09]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t)
		ctx := testutil.Context(t)
		operator := extensionSetupOperator(t)
		service := fixture.service

		written, err := service.SetInputs(
			ctx,
			target,
			setupValues("project", setupProjectValue, "token", setupTokenValue),
			operator,
		)
		if err != nil {
			t.Fatalf("SetInputs() error = %v", err)
		}
		if !slices.Equal(written.Changed, []string{"project", "token"}) || !written.Reloaded ||
			fixture.runtime.reloadCount != 1 || len(written.Missing) != 0 {
			t.Fatalf("first write = %#v reloads=%d", written, fixture.runtime.reloadCount)
		}
		encoded, err := json.Marshal(written)
		if err != nil {
			t.Fatal(err)
		}
		tokenRef := vault.ExtensionProfileSecretRef(setupKitName, store.DefaultProfileID, "", "API_TOKEN")
		for _, forbidden := range []string{setupProjectValue, setupTokenValue, tokenRef} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatal("setup write leaked an input value or Vault reference")
			}
		}

		repeated, err := service.SetInputs(ctx, target, setupValues("project", setupProjectValue), operator)
		if err != nil {
			t.Fatalf("SetInputs(no-op) error = %v", err)
		}
		if len(repeated.Changed) != 0 || repeated.Reloaded || fixture.runtime.reloadCount != 1 {
			t.Fatalf("no-op write = %#v reloads=%d", repeated, fixture.runtime.reloadCount)
		}

		// Binding the daemon_env input replaces the daemon fallback; the omitted project stays bound.
		regional, err := service.SetInputs(ctx, target, setupValues("region", "us-east"), operator)
		if err != nil {
			t.Fatalf("SetInputs(region) error = %v", err)
		}
		states := setupInputStates(regional)
		if states["region"] != contract.ExtensionInputStateSet ||
			states["project"] != contract.ExtensionInputStateSet ||
			len(regional.DaemonEnv) != 0 ||
			fixture.runtime.reloadCount != 2 {
			t.Fatalf("region write = %#v reloads=%d", regional, fixture.runtime.reloadCount)
		}
		bound, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, "")
		if err != nil {
			t.Fatal(err)
		}
		project := extensionBindingsByName(bound)["PROJECT_ID"]
		if project.InputID != "project" || project.Inactive {
			t.Fatalf("project binding after another input's setup = %#v, want active", project)
		}
		other, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, "profile-other", "")
		if err != nil || len(other) != 0 {
			t.Fatalf("other profile bindings = %#v error %v, want untouched", other, err)
		}

		if err := service.registry.SetEnabledForProfile(setupKitName, store.DefaultProfileID, false); err != nil {
			t.Fatal(err)
		}
		disabled, err := service.SetInputs(ctx, target, setupValues("token", "rotated-token"), operator)
		if err != nil {
			t.Fatalf("SetInputs(disabled) error = %v", err)
		}
		enabled, err := service.registry.IsEnabledForProfile(setupKitName, store.DefaultProfileID)
		if err != nil || enabled || disabled.Enabled || disabled.Reloaded || fixture.runtime.reloadCount != 2 {
			t.Fatalf("disabled write = %#v enabled=%v reloads=%d error %v",
				disabled, enabled, fixture.runtime.reloadCount, err)
		}
		info, err := service.registry.Get(setupKitName)
		if err != nil || info.Version != "1.0.0" {
			t.Fatalf("installed version after setup = %#v error %v", info, err)
		}
	})

	t.Run("Should leave required inputs missing on a partial write and refuse invalid values", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t)
		ctx := testutil.Context(t)
		partial, err := fixture.service.SetInputs(
			ctx,
			target,
			setupValues("token", setupTokenValue),
			extensionSetupOperator(t),
		)
		if err != nil {
			t.Fatalf("SetInputs(partial) error = %v", err)
		}
		if !slices.Equal(partial.Missing, []string{"project"}) || !slices.Equal(partial.Changed, []string{"token"}) {
			t.Fatalf("partial write = %#v, want project still missing", partial)
		}
		_, err = fixture.service.SetInputs(
			ctx,
			target,
			setupValues("project", "not an identifier!"),
			extensionSetupOperator(t),
		)
		invalid, ok := errors.AsType[*extensionpkg.InputValidationError](err)
		if !ok || invalid.InputID != "project" {
			t.Fatalf("SetInputs(invalid) error = %v, want project input invalid", err)
		}
	})

	t.Run("Should report daemon_env in status while keeping set true [T-09]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t)
		ctx := testutil.Context(t)
		instance, err := fixture.service.resolveInputsInstance(ctx, target, extensionSetupOperator(t))
		if err != nil {
			t.Fatal(err)
		}
		ext, err := fixture.service.inputsInstanceExtension(ctx, instance)
		if err != nil {
			t.Fatal(err)
		}
		var payload contract.ExtensionPayload
		if err := fixture.service.populateExtensionInputStatus(ctx, ext, store.DefaultProfileID, &payload); err != nil {
			t.Fatal(err)
		}
		for _, input := range payload.Inputs {
			if input.ID == "region" && (!input.Set || input.State != contract.ExtensionInputStateDaemonEnv) {
				t.Fatalf("region status = %#v, want set with daemon_env state", input)
			}
		}
		if slices.Contains(payload.MissingInputs, "region") || slices.Contains(payload.MissingEnv, "REGION") {
			t.Fatalf("daemon_env input blocked readiness: %#v", payload)
		}

		// The compact info and list results carry the new state and must stay valid for their schemas.
		status, err := fixture.service.payloadFromExtension(ctx, ext, extensionDefaultProfileLens())
		if err != nil {
			t.Fatal(err)
		}
		results := map[toolspkg.ToolID]any{
			toolspkg.ToolIDExtensionsInfo: compactExtensionRecord(
				&contract.InstallExtensionResponse{Extension: status},
			),
			toolspkg.ToolIDExtensionsList: compactExtensionList([]contract.ExtensionPayload{status}),
		}
		for _, descriptor := range builtintools.NativeDescriptors() {
			result, found := results[descriptor.ID]
			if !found {
				continue
			}
			encoded, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), `"state":"daemon_env"`) {
				t.Fatalf("%s compact result lacks the daemon_env state: %s", descriptor.ID, encoded)
			}
			requireNativeOutputSchemaAccepts(t, descriptor, encoded)
			delete(results, descriptor.ID)
		}
		if len(results) != 0 {
			t.Fatalf("descriptors not found for %v", results)
		}
	})
}

// Invariant: an agent caller writes only missing, invalid or daemon_env inputs of its trusted instance (D7).
// Owner: native extension setup tool; suite: TestExtensionInputSetupAgentAuthority.
func TestExtensionInputSetupAgentAuthority(t *testing.T) {
	t.Parallel()
	agentScope := toolspkg.Scope{
		SessionID:   "sess-setup",
		WorkspaceID: setupWorkspaceID,
		ProfileID:   store.DefaultProfileID,
	}
	call := func(t *testing.T, fixture extensionSetupFixture, input string) (toolspkg.ToolResult, error) {
		t.Helper()
		tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			Extensions: func() core.ExtensionService { return fixture.service },
		}}
		return tools.extensionSetup(testutil.Context(t), agentScope, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsSetup, Input: json.RawMessage(input),
		})
	}

	t.Run("Should let an agent write missing and daemon_env inputs [T-10]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t)
		read, err := call(t, fixture, `{"name":"setup-kit"}`)
		if err != nil {
			t.Fatalf("extensions_setup read error = %v", err)
		}
		requireNativeStructuredContains(t, read, []byte(`"missing":["project"]`))
		if fixture.runtime.reloadCount != 0 {
			t.Fatal("a read reloaded the extension")
		}
		written, err := call(t, fixture,
			`{"name":"setup-kit","inputs":{"project":{"value":"acme-project-41"},"region":{"value":"us-east"}}}`)
		if err != nil {
			t.Fatalf("extensions_setup write error = %v", err)
		}
		requireNativeStructuredContains(t, written, []byte(`"changed":["project","region"]`))
	})

	t.Run("Should refuse an agent write that includes a set input and write nothing [T-10]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t)
		ctx := testutil.Context(t)
		if _, err := fixture.service.SetInputs(
			ctx,
			extensionInputsTarget{Name: setupKitName},
			setupValues("project", setupProjectValue),
			extensionSetupOperator(t),
		); err != nil {
			t.Fatal(err)
		}
		before, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = call(t, fixture,
			`{"name":"setup-kit","inputs":{"project":{"value":"other-project"},"token":{"value":"agent-token"}}}`)
		toolErr, ok := errors.AsType[*toolspkg.ToolError](err)
		if !ok || toolErr.Code != toolspkg.ErrorCode("extension_input_overwrite_denied") ||
			!errors.Is(err, extensionpkg.ErrExtensionInputOverwriteDenied) {
			t.Fatalf("extensions_setup overwrite error = %#v, want extension_input_overwrite_denied", err)
		}
		if toolErr.PartialResult == nil {
			t.Fatal("overwrite refusal carries no structured payload")
		}
		var denied contract.ExtensionOperationErrorPayload
		if err := json.Unmarshal(toolErr.PartialResult.Structured, &denied); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(denied.InputIDs, []string{"project"}) {
			t.Fatalf("denied input_ids = %#v, want project", denied.InputIDs)
		}
		after, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, "")
		if err != nil || !slices.EqualFunc(before, after, func(left, right extensionpkg.EnvBinding) bool {
			return left.EnvName == right.EnvName && left.SecretRef == right.SecretRef && left.Inactive == right.Inactive
		}) {
			t.Fatalf("refused write changed bindings: before %#v after %#v error %v", before, after, err)
		}
	})
}
