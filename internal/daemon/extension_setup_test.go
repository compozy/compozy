package daemon

import (
	"context"
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

// newExtensionSetupFixture installs setup-kit enabled in scope, with REGION only in the daemon
// environment so the dual-consumer region input reads daemon_env.
func newExtensionSetupFixture(t *testing.T, scope extensionpkg.InstallationScope) extensionSetupFixture {
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
	if scope.WorkspaceID != "" {
		stamp := store.FormatTimestamp(service.now())
		if _, err := service.registry.DB().ExecContext(testutil.Context(t), `INSERT INTO workspaces
			(id, root_dir, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
			scope.WorkspaceID, t.TempDir(), "Setup workspace", stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.registry.Install(
		manifest, dir, checksum, extensionpkg.WithInstallScope(scope),
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
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
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
		if !slices.Equal(payload.Missing, []string{"project"}) ||
			!slices.Equal(payload.DaemonEnv, []string{"region"}) ||
			len(payload.Invalid) != 0 ||
			len(payload.MissingEnv) != 0 ||
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
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
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
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
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
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
		ctx := testutil.Context(t)
		instance, err := fixture.service.resolveInputsInstance(ctx, target, extensionSetupOperator(t), false)
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
		// A required subprocess input is reported by id so the Web setup notice shows (R-6).
		if !slices.Equal(payload.MissingInputs, []string{"project"}) {
			t.Fatalf("status missing_inputs = %#v, want the required subprocess input", payload.MissingInputs)
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

// restartRecordingRuntime records instance-scoped restarts and fails if the manager-wide reload runs.
type restartRecordingRuntime struct {
	*fakeExtensionRuntime
	restarted []extensionpkg.InstanceKey
}

func (r *restartRecordingRuntime) RestartInstance(_ context.Context, key extensionpkg.InstanceKey) error {
	r.restarted = append(r.restarted, key)
	return nil
}

// Invariant: a setup write restarts only the resolved instance's runtime (rule 7, Safety Invariant 3).
// Owner: daemon setup service; suite: TestExtensionInputSetupInstanceRestart.
func TestExtensionInputSetupInstanceRestart(t *testing.T) {
	t.Parallel()
	t.Run("Should restart only the resolved instance and skip a reload that cannot start [R-2]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
		ctx := testutil.Context(t)
		recorder := &restartRecordingRuntime{fakeExtensionRuntime: fixture.runtime}
		fixture.service.runtime = recorder
		target := extensionInputsTarget{Name: setupKitName}

		// project stays missing, so the instance cannot start and the partial write is not reloaded.
		partial, err := fixture.service.SetInputs(
			ctx, target, setupValues("token", setupTokenValue), extensionSetupOperator(t),
		)
		if err != nil || partial.Reloaded || len(recorder.restarted) != 0 {
			t.Fatalf("partial write = %#v restarts %#v error %v, want no reload", partial, recorder.restarted, err)
		}
		written, err := fixture.service.SetInputs(
			ctx, target, setupValues("project", setupProjectValue), extensionSetupOperator(t),
		)
		if err != nil {
			t.Fatalf("SetInputs() error = %v", err)
		}
		want := []extensionpkg.InstanceKey{{Name: setupKitName, ProfileID: store.DefaultProfileID}}
		if !written.Reloaded || !slices.Equal(recorder.restarted, want) || fixture.runtime.reloadCount != 0 {
			t.Fatalf("restarts = %#v manager reloads = %d, want only %#v",
				recorder.restarted, fixture.runtime.reloadCount, want)
		}
	})

	t.Run("Should skip the reload while a required dual-consumer input is missing [R-7]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
		fixture.service.getenv = func(string) string { return "" } // REGION is not daemon_env
		ctx := testutil.Context(t)
		recorder := &restartRecordingRuntime{fakeExtensionRuntime: fixture.runtime}
		fixture.service.runtime = recorder
		target := extensionInputsTarget{Name: setupKitName}

		written, err := fixture.service.SetInputs(
			ctx, target, setupValues("project", setupProjectValue), extensionSetupOperator(t),
		)
		if err != nil || written.Reloaded || len(recorder.restarted) != 0 {
			t.Fatalf("write = %#v restarts %#v error %v, want no reload while region is missing",
				written, recorder.restarted, err)
		}
		if states := setupInputStates(written); states["region"] !=
			contract.ExtensionInputStateMissing {
			t.Fatalf("region state = %q, want missing", states["region"])
		}
	})

	// A save that only mirrors an unchanged binding into its typed row changes no launched value.
	t.Run("Should persist a mirror-only save without restarting the instance", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
		fixture.service.getenv = func(string) string { return "" }
		ctx := testutil.Context(t)
		secretVault, ok := fixture.service.secretVault.(*extensionSecretVaultFake)
		if !ok {
			t.Fatal("setup fixture vault is not the fake vault")
		}
		// `secrets set` bound both values; neither has a typed row yet.
		for _, bound := range []struct{ inputID, envName, value string }{
			{"project", "PROJECT_ID", setupProjectValue},
			{"region", "REGION", "us-east"},
		} {
			ref := vault.ExtensionProfileSecretRef(setupKitName, store.DefaultProfileID, "", bound.envName)
			secretVault.seed(ref, extensionpkg.ExtensionEnvBindingKind, bound.value)
			if err := fixture.bindings.PutEnvBinding(ctx, extensionpkg.EnvBinding{
				ExtensionName: setupKitName, ProfileID: store.DefaultProfileID, EnvName: bound.envName,
				SecretRef: ref, Kind: extensionpkg.ExtensionEnvBindingKind, InputID: bound.inputID,
			}); err != nil {
				t.Fatalf("PutEnvBinding(%s) error = %v", bound.envName, err)
			}
		}
		recorder := &restartRecordingRuntime{fakeExtensionRuntime: fixture.runtime}
		fixture.service.runtime = recorder

		written, err := fixture.service.SetInputs(
			ctx, extensionInputsTarget{Name: setupKitName}, setupValues("region", "us-east"), extensionSetupOperator(t),
		)
		if err != nil || written.Reloaded || len(written.Changed) != 0 || len(recorder.restarted) != 0 {
			t.Fatalf("mirror-only save = %#v restarts %#v error %v, want no change and no restart",
				written, recorder.restarted, err)
		}
		rows, err := fixture.service.inputs.List(
			ctx, extensioninput.Instance{Extension: setupKitName, ProfileID: store.DefaultProfileID},
		)
		if err != nil {
			t.Fatal(err)
		}
		if row := rows["region"]; !row.Active || string(row.Value) != `"us-east"` {
			t.Fatalf("region row = %#v, want the mirrored binding value persisted", row)
		}
	})
}

// failingRestartRuntime fails the next instance restart and records the stored project value each
// restart would launch with.
type failingRestartRuntime struct {
	*fakeExtensionRuntime
	failNext bool
	project  func() string
	launched []string
}

func (r *failingRestartRuntime) RestartInstance(context.Context, extensionpkg.InstanceKey) error {
	r.launched = append(r.launched, r.project())
	if r.failNext {
		r.failNext = false
		return errors.New("injected instance restart failure")
	}
	return nil
}

// Invariant: a setup write whose instance restart fails restores bindings, Vault values and typed rows,
// then restarts the instance with the old values (Safety Invariant 4).
// Owner: daemon setup service; suite: TestExtensionInputSetupInstanceRestart.
func TestExtensionInputSetupReloadRollback(t *testing.T) {
	t.Parallel()
	t.Run("Should restore the prior inputs and restart with them when the reload fails", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
		ctx := testutil.Context(t)
		secretVault, ok := fixture.service.secretVault.(*extensionSecretVaultFake)
		if !ok {
			t.Fatal("setup fixture vault is not the fake vault")
		}
		projectRef := vault.ExtensionProfileSecretRef(setupKitName, store.DefaultProfileID, "", "PROJECT_ID")
		runtime := &failingRestartRuntime{
			fakeExtensionRuntime: fixture.runtime,
			project:              func() string { return secretVault.value(projectRef) },
		}
		fixture.service.runtime = runtime
		target := extensionInputsTarget{Name: setupKitName}
		instance := extensioninput.Instance{Extension: setupKitName, ProfileID: store.DefaultProfileID}
		if _, err := fixture.service.SetInputs(
			ctx, target, setupValues("project", setupProjectValue, "region", "us-east"), extensionSetupOperator(t),
		); err != nil {
			t.Fatalf("SetInputs() error = %v", err)
		}
		bindingsBefore, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, "")
		if err != nil {
			t.Fatal(err)
		}
		rowsBefore, err := fixture.service.inputs.List(ctx, instance)
		if err != nil {
			t.Fatal(err)
		}

		runtime.failNext, runtime.launched = true, nil
		_, err = fixture.service.SetInputs(
			ctx, target, setupValues("project", "acme-project-99", "region", "eu-other"), extensionSetupOperator(t),
		)
		if err == nil || !strings.Contains(err.Error(), "injected instance restart failure") {
			t.Fatalf("SetInputs() error = %v, want the restart failure", err)
		}
		if want := []string{"acme-project-99", setupProjectValue}; !slices.Equal(runtime.launched, want) {
			t.Fatalf("restarts launched with %#v, want %#v", runtime.launched, want)
		}
		if got := secretVault.value(projectRef); got != setupProjectValue {
			t.Fatalf("project Vault value = %q, want the prior value restored", got)
		}
		bindingsAfter, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, "")
		if err != nil {
			t.Fatal(err)
		}
		if !slices.EqualFunc(bindingsBefore, bindingsAfter, func(a, b extensionpkg.EnvBinding) bool {
			return a.EnvName == b.EnvName && a.SecretRef == b.SecretRef && a.InputID == b.InputID &&
				a.Inactive == b.Inactive
		}) {
			t.Fatalf("bindings after rollback = %#v, want %#v", bindingsAfter, bindingsBefore)
		}
		rowsAfter, err := fixture.service.inputs.List(ctx, instance)
		if err != nil {
			t.Fatal(err)
		}
		if string(rowsAfter["region"].Value) != `"us-east"` || !rowsAfter["region"].Active ||
			len(rowsAfter) != len(rowsBefore) {
			t.Fatalf("rows after rollback = %#v, want %#v", rowsAfter, rowsBefore)
		}
	})

	t.Run("Should roll back a committed write when the post-commit read fails", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
		ctx := testutil.Context(t)
		secretVault, ok := fixture.service.secretVault.(*extensionSecretVaultFake)
		if !ok {
			t.Fatal("setup fixture vault is not the fake vault")
		}
		target := extensionInputsTarget{Name: setupKitName}
		instance := extensioninput.Instance{Extension: setupKitName, ProfileID: store.DefaultProfileID}
		if _, err := fixture.service.SetInputs(
			ctx, target, setupValues("project", setupProjectValue, "region", "us-east"), extensionSetupOperator(t),
		); err != nil {
			t.Fatalf("SetInputs() error = %v", err)
		}
		rows := fixture.service.inputs
		injected := errors.New("injected post-commit read failure")
		fixture.service.inputs = &extensionInputListFailureAfterApply{Store: rows, err: injected}

		_, err := fixture.service.SetInputs(ctx, target, setupValues("region", "eu-other"), extensionSetupOperator(t))
		if !errors.Is(err, injected) {
			t.Fatalf("SetInputs() error = %v, want the post-commit read failure", err)
		}
		stored, err := rows.List(ctx, instance)
		if err != nil {
			t.Fatal(err)
		}
		if string(stored["region"].Value) != `"us-east"` || !stored["region"].Active {
			t.Fatalf("region row after failed read = %#v, want the prior value restored", stored["region"])
		}
		regionRef := vault.ExtensionProfileSecretRef(setupKitName, store.DefaultProfileID, "", "REGION")
		if got := secretVault.value(regionRef); got != "us-east" {
			t.Fatalf("region Vault value = %q, want the prior value restored", got)
		}
	})
}

// extensionInputListFailureAfterApply fails every row read once a row write has landed, modeling a
// post-commit read failure; Apply keeps delegating so a rollback can still restore the rows.
type extensionInputListFailureAfterApply struct {
	extensioninput.Store
	applied bool
	err     error
}

func (s *extensionInputListFailureAfterApply) Apply(
	ctx context.Context, instance extensioninput.Instance, mutations []extensioninput.Mutation,
) error {
	if err := s.Store.Apply(ctx, instance, mutations); err != nil {
		return err
	}
	s.applied = true
	return nil
}

func (s *extensionInputListFailureAfterApply) List(
	ctx context.Context, instance extensioninput.Instance,
) (map[string]extensioninput.Record, error) {
	if s.applied {
		return nil, s.err
	}
	return s.Store.List(ctx, instance)
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
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{WorkspaceID: setupWorkspaceID})
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
		ctx := testutil.Context(t)
		own, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, setupWorkspaceID)
		if err != nil || extensionBindingsByName(own)["PROJECT_ID"].InputID != "project" {
			t.Fatalf("agent workspace bindings = %#v error %v, want project bound", own, err)
		}
		for _, workspaceID := range []string{"", "ws-other"} {
			other, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, workspaceID)
			if err != nil || len(other) != 0 {
				t.Fatalf("workspace %q bindings = %#v error %v, want untouched", workspaceID, other, err)
			}
		}
	})

	t.Run("Should refuse an agent write to a global installation and leave the global cell untouched [R-1]",
		func(t *testing.T) {
			t.Parallel()
			fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{})
			ctx := testutil.Context(t)
			read, err := call(t, fixture, `{"name":"setup-kit"}`)
			if err != nil {
				t.Fatalf("extensions_setup read of the visible global installation error = %v", err)
			}
			requireNativeStructuredContains(t, read, []byte(`"missing":["project"]`))
			_, err = call(t, fixture, `{"name":"setup-kit","inputs":{"project":{"value":"acme-project-41"}}}`)
			if _, ok := errors.AsType[*extensionpkg.ManifestValidationError](err); !ok {
				t.Fatalf("extensions_setup global write error = %#v, want a scope refusal", err)
			}
			for _, workspaceID := range []string{"", setupWorkspaceID} {
				bound, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, workspaceID)
				if err != nil || len(bound) != 0 {
					t.Fatalf("workspace %q bindings = %#v error %v, want untouched", workspaceID, bound, err)
				}
			}
			if fixture.runtime.reloadCount != 0 {
				t.Fatal("a refused write reloaded the extension")
			}
		})

	t.Run("Should refuse an agent write that includes a set input and write nothing [T-10]", func(t *testing.T) {
		t.Parallel()
		fixture := newExtensionSetupFixture(t, extensionpkg.InstallationScope{WorkspaceID: setupWorkspaceID})
		ctx := testutil.Context(t)
		if _, err := fixture.service.SetInputs(
			ctx,
			extensionInputsTarget{Name: setupKitName, Scope: "workspace", WorkspaceID: setupWorkspaceID},
			setupValues("project", setupProjectValue),
			extensionSetupOperator(t),
		); err != nil {
			t.Fatal(err)
		}
		before, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, setupWorkspaceID)
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
		after, err := fixture.bindings.ListEnvBindings(ctx, setupKitName, store.DefaultProfileID, setupWorkspaceID)
		if err != nil || !slices.EqualFunc(before, after, func(left, right extensionpkg.EnvBinding) bool {
			return left.EnvName == right.EnvName && left.SecretRef == right.SecretRef && left.Inactive == right.Inactive
		}) {
			t.Fatalf("refused write changed bindings: before %#v after %#v error %v", before, after, err)
		}
	})
}
