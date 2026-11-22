package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/marketplace"
	profilepkg "github.com/compozy/compozy/internal/profile"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// dependencyFixtureMinVersion is the dependencies floor every dependency-declaring fixture must require.
const dependencyFixtureMinVersion = "0.7.9"

type dependencyFixture struct {
	service  *daemonExtensionService
	registry *extensionpkg.Registry
	deps     *daemonNativeToolsDeps
	dirs     map[string]string
}

// newDependencyFixture builds a service over real SQLite whose unverified policy allows consented side-loads.
func newDependencyFixture(t *testing.T) *dependencyFixture {
	t.Helper()
	deps, registry, _, _ := newNativeExtensionToolDeps(t)
	service := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry: registry, Runtime: newLifecycleStateRuntime(registry), HomePaths: deps.HomePaths,
		Profiles: deps.ProfileManager, Logger: discardLogger(),
	}, withDaemonExtensionMarketplace(validNativeExtensionConfig(true), deps.ExtensionSources),
		withDaemonExtensionAutomation(&fakeAutomationManager{})).(*daemonExtensionService)
	return &dependencyFixture{service: service, registry: registry, deps: deps, dirs: make(map[string]string)}
}

// dir reserves one package directory per name so manifests can reference each other before they are written.
func (f *dependencyFixture) dir(t *testing.T, name string) string {
	t.Helper()
	if dir, ok := f.dirs[name]; ok {
		return dir
	}
	f.dirs[name] = t.TempDir()
	return f.dirs[name]
}

// write stores name@version requiring each "dep" or "dep>=min" entry as a local_path dependency.
func (f *dependencyFixture) write(t *testing.T, name, version string, requires ...string) string {
	t.Helper()
	var builder strings.Builder
	fmt.Fprintf(&builder, "name = %q\nversion = %q\nmin_compozy_version = %q\n", name, version,
		dependencyFixtureMinVersion)
	for _, entry := range requires {
		dependency, minVersion, _ := strings.Cut(entry, ">=")
		fmt.Fprintf(&builder, "\n[[requires.extensions]]\nname = %q\nsource = \"local_path\"\nref = %q\n",
			dependency, f.dir(t, dependency))
		if minVersion != "" {
			fmt.Fprintf(&builder, "min_version = %q\n", minVersion)
		}
	}
	dir := f.dir(t, name)
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(builder.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func (f *dependencyFixture) install(t *testing.T, name string, req contract.InstallExtensionRequest) {
	t.Helper()
	req.Source, req.Ref, req.AllowUnverified = contract.InstallExtensionSourceLocalPath, f.dir(t, name), true
	if _, err := f.service.Install(t.Context(), req, dependencyOperator(t)); err != nil {
		t.Fatalf("install %s error = %v", name, err)
	}
}

func (f *dependencyFixture) resolve(t *testing.T, root *extensionpkg.Manifest, target extensionMutationTarget) (
	[]contract.ExtensionDependencyPayload, error,
) {
	t.Helper()
	graph, err := f.service.resolveDependencies(t.Context(), root, "", target, nil, dependencyOperator(t))
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		if closeErr := graph.Close(); closeErr != nil {
			t.Errorf("graph.Close() error = %v", closeErr)
		}
	})
	return graph.payloads(), nil
}

func (f *dependencyFixture) installed(t *testing.T, name string) bool {
	t.Helper()
	_, err := f.registry.Get(name)
	if err != nil && !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		t.Fatalf("registry.Get(%s) error = %v", name, err)
	}
	return err == nil
}

// rootRequest installs the "root" fixture; a non-nil consent map also sets with_dependencies.
func (f *dependencyFixture) rootRequest(
	t *testing.T, consent map[string]contract.ExtensionDependencyConsent,
) contract.InstallExtensionRequest {
	t.Helper()
	return contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: f.dir(t, "root"), AllowUnverified: true,
		WithDependencies: consent != nil, Dependencies: consent,
	}
}

func dependencyOperator(t *testing.T) taskpkg.ActorContext {
	t.Helper()
	actor, err := taskpkg.DeriveHumanActorContext("operator", taskpkg.OriginKindCLI, "extension dependencies")
	if err != nil {
		t.Fatal(err)
	}
	return actor
}

func dependencyRoot(requires ...extensionpkg.ExtensionDependency) *extensionpkg.Manifest {
	return &extensionpkg.Manifest{Name: "root", Requires: extensionpkg.ManifestRequires{Extensions: requires}}
}

func (f *dependencyFixture) localDependency(t *testing.T, name, minVersion string) extensionpkg.ExtensionDependency {
	t.Helper()
	return extensionpkg.ExtensionDependency{
		Name:       name,
		Source:     string(contract.InstallExtensionSourceLocalPath),
		Ref:        f.dir(t, name),
		MinVersion: minVersion,
	}
}

func dependencyStates(items []contract.ExtensionDependencyPayload) map[string]string {
	states := make(map[string]string, len(items))
	for _, item := range items {
		states[item.Name] = item.State
	}
	return states
}

func dependencyPayloadNames(items []contract.ExtensionDependencyPayload) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// Invariant: the resolver classifies each dependency against installations visible to the dependent's instance
// (D11), orders the graph dependencies first, collapses diamonds onto the highest min_version, and refuses
// cycles, depth beyond 5 and conflicting sources before any mutation (rules 11 and 12).
// Owner: daemon dependency resolver. Canonical suite: extension_dependencies_test.go.
func TestExtensionDependencyResolution(t *testing.T) {
	t.Parallel()

	t.Run("Should classify met, missing, disabled and version_unmet dependencies", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-met", "1.0.0")
		fixture.write(t, "dep-disabled", "1.0.0")
		fixture.write(t, "dep-old", "0.1.0")
		fixture.write(t, "dep-missing", "0.5.1")
		for _, name := range []string{"dep-met", "dep-disabled", "dep-old"} {
			fixture.install(t, name, contract.InstallExtensionRequest{})
		}
		if err := fixture.registry.Disable("dep-disabled"); err != nil {
			t.Fatal(err)
		}
		rootDir := fixture.write(t, "root", "1.0.0", "dep-met", "dep-disabled", "dep-old>=0.2.0", "dep-missing>=0.4.0")

		preview, err := fixture.service.PreviewInstall(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath, Ref: rootDir, AllowUnverified: true,
		}, dependencyOperator(t))
		if err != nil {
			t.Fatalf("PreviewInstall() error = %v", err)
		}
		want := map[string]string{
			"dep-met": dependencyStateMet, "dep-disabled": dependencyStateDisabled,
			"dep-old": dependencyStateVersionUnmet, "dep-missing": dependencyStateMissing,
		}
		if got := dependencyStates(preview.Dependencies); !reflect.DeepEqual(got, want) {
			t.Fatalf("states = %#v, want %#v", got, want)
		}
		for _, item := range preview.Dependencies {
			switch item.Name {
			case "dep-old":
				// A local_path package is not marketplace-installed: `extension update` cannot reach 0.2.0 (G-7).
				if item.InstalledVersion != "0.1.0" || item.RetryCommand != "" {
					t.Fatalf("version_unmet entry = %+v", item)
				}
			case "dep-disabled":
				// Consent granted at install is not verification: the recorded tier stays unverified (B-7).
				if item.RegistryTier != extensionpkg.ExtensionRegistryTierUnverified ||
					item.InstalledVersion != "1.0.0" {
					t.Fatalf("disabled entry = %+v, want its unverified tier and installed version", item)
				}
			case "dep-missing":
				if item.Version != "0.5.1" || item.DigestSHA256 == "" || !item.ConsentRequired ||
					item.RegistryTier != extensionpkg.ExtensionRegistryTierUnverified ||
					!slices.Equal(item.RequiredBy, []string{"root"}) || item.Depth != 1 {
					t.Fatalf("missing entry = %+v", item)
				}
			}
		}
		if fixture.installed(t, "dep-missing") || fixture.installed(t, "root") {
			t.Fatal("preview installed a package")
		}
	})

	t.Run("Should see global installations from workspaces of the same profile only", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-global", "1.0.0")
		fixture.write(t, "dep-other-profile", "1.0.0")
		fixture.install(t, "dep-global", contract.InstallExtensionRequest{})
		other, err := fixture.deps.ProfileManager.Create(t.Context(), profilepkg.CreateInput{Name: "other"})
		if err != nil {
			t.Fatal(err)
		}
		fixture.install(t, "dep-other-profile", contract.InstallExtensionRequest{Profile: other.Name})
		workspaceTarget := extensionMutationTarget{
			scope:   extensionpkg.InstallationScope{WorkspaceID: "ws-dependent"},
			profile: extensionDefaultProfileLens(),
		}

		items, err := fixture.resolve(t, dependencyRoot(fixture.localDependency(t, "dep-global", "")), workspaceTarget)
		if err != nil || dependencyStates(items)["dep-global"] != dependencyStateMet {
			t.Fatalf("global dependency from a workspace = %+v, %v; want met", items, err)
		}
		items, err = fixture.resolve(t, dependencyRoot(fixture.localDependency(t, "dep-other-profile", "")),
			extensionMutationTarget{profile: extensionDefaultProfileLens()})
		if err != nil || dependencyStates(items)["dep-other-profile"] != dependencyStateMissing {
			t.Fatalf("dependency installed for another profile = %+v, %v; want missing", items, err)
		}
	})

	t.Run("Should order transitive dependencies first", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-a", "1.0.0", "dep-b")
		fixture.write(t, "dep-b", "1.0.0")

		items, err := fixture.resolve(t, dependencyRoot(fixture.localDependency(t, "dep-a", "")),
			extensionMutationTarget{profile: extensionDefaultProfileLens()})
		if err != nil {
			t.Fatalf("resolveDependencies() error = %v", err)
		}
		if got := dependencyPayloadNames(items); !slices.Equal(got, []string{"dep-b", "dep-a"}) {
			t.Fatalf("order = %v, want [dep-b dep-a]", got)
		}
		if items[0].Depth != 2 || !slices.Equal(items[0].RequiredBy, []string{"dep-a"}) {
			t.Fatalf("transitive node = %+v", items[0])
		}
	})

	t.Run("Should collapse a diamond onto one node with the highest min_version", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-d", "0.2.0")
		fixture.install(t, "dep-d", contract.InstallExtensionRequest{})
		fixture.write(t, "dep-b", "1.0.0", "dep-d>=0.1.0")
		fixture.write(t, "dep-c", "1.0.0", "dep-d>=0.3.0")

		items, err := fixture.resolve(t, dependencyRoot(
			fixture.localDependency(t, "dep-b", ""), fixture.localDependency(t, "dep-c", ""),
		), extensionMutationTarget{profile: extensionDefaultProfileLens()})
		if err != nil {
			t.Fatalf("resolveDependencies() error = %v", err)
		}
		var nodes []contract.ExtensionDependencyPayload
		for _, item := range items {
			if item.Name == "dep-d" {
				nodes = append(nodes, item)
			}
		}
		if len(nodes) != 1 || nodes[0].MinVersion != "0.3.0" || nodes[0].State != dependencyStateVersionUnmet ||
			!slices.Equal(nodes[0].RequiredBy, []string{"dep-b", "dep-c"}) {
			t.Fatalf("diamond nodes = %+v", nodes)
		}
	})

	invalid := []struct {
		name   string
		build  func(t *testing.T, fixture *dependencyFixture) *extensionpkg.Manifest
		reason string
		path   []string
	}{
		{
			name: "a cycle",
			build: func(t *testing.T, fixture *dependencyFixture) *extensionpkg.Manifest {
				t.Helper()
				fixture.write(t, "dep-a", "1.0.0", "dep-b")
				fixture.write(t, "dep-b", "1.0.0", "dep-a")
				return dependencyRoot(fixture.localDependency(t, "dep-a", ""))
			},
			reason: extensionpkg.DependencyGraphReasonCycle,
			path:   []string{"root", "dep-a", "dep-b", "dep-a"},
		},
		{
			name: "a chain deeper than five",
			build: func(t *testing.T, fixture *dependencyFixture) *extensionpkg.Manifest {
				t.Helper()
				for level := 1; level <= 6; level++ {
					var next []string
					if level < 6 {
						next = []string{fmt.Sprintf("dep-%d", level+1)}
					}
					fixture.write(t, fmt.Sprintf("dep-%d", level), "1.0.0", next...)
				}
				return dependencyRoot(fixture.localDependency(t, "dep-1", ""))
			},
			reason: extensionpkg.DependencyGraphReasonDepth,
			path:   []string{"root", "dep-1", "dep-2", "dep-3", "dep-4", "dep-5", "dep-6"},
		},
	}
	for _, test := range invalid {
		t.Run("Should refuse "+test.name, func(t *testing.T) {
			t.Parallel()
			fixture := newDependencyFixture(t)
			_, err := fixture.resolve(t, test.build(t, fixture), extensionMutationTarget{
				profile: extensionDefaultProfileLens(),
			})
			graphErr, ok := errors.AsType[*extensionpkg.DependencyGraphInvalidError](err)
			if !ok || graphErr.Reason != test.reason || !slices.Equal(graphErr.Path, test.path) {
				t.Fatalf("error = %v, want %s with path %v", err, test.reason, test.path)
			}
		})
	}

	t.Run("Should refuse a diamond declared with conflicting sources and install nothing", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-d", "1.0.0")
		fixture.write(t, "dep-b", "1.0.0", "dep-d")
		cDir := fixture.write(t, "dep-c", "1.0.0")
		otherD := t.TempDir()
		conflicting := fmt.Sprintf(
			"\n[[requires.extensions]]\nname = \"dep-d\"\nsource = \"local_path\"\nref = %q\n",
			otherD,
		)
		appendDependencyManifest(t, cDir, conflicting)
		rootDir := fixture.write(t, "root", "1.0.0", "dep-b", "dep-c")

		_, err := fixture.service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath, Ref: rootDir, AllowUnverified: true,
			WithDependencies: true, Dependencies: map[string]contract.ExtensionDependencyConsent{
				"dep-b": {AllowUnverified: true}, "dep-c": {AllowUnverified: true}, "dep-d": {AllowUnverified: true},
			},
		}, dependencyOperator(t))
		graphErr, ok := errors.AsType[*extensionpkg.DependencyGraphInvalidError](err)
		want := []contract.ExtensionDependencyParentPayload{
			{Name: "dep-b", Source: "local_path", Ref: fixture.dir(t, "dep-d")},
			{Name: "dep-c", Source: "local_path", Ref: otherD},
		}
		if !ok || graphErr.Reason != extensionpkg.DependencyGraphReasonConflictingSource ||
			!reflect.DeepEqual(graphErr.Parents, want) {
			t.Fatalf("error = %v, want conflicting_source naming dep-b and dep-c", err)
		}
		for _, name := range []string{"root", "dep-b", "dep-c", "dep-d"} {
			if fixture.installed(t, name) {
				t.Fatalf("%s was installed", name)
			}
		}
	})
}

func appendDependencyManifest(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "extension.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, content...), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Invariant: unmet dependencies refuse install without consent; consented installs run dependencies first with
// per-dependency trust consent, never update a version_unmet dependency, and stop at the first failure keeping
// and reporting what they completed (rules 12 and 13).
// Owner: daemon dependency install gate. Canonical suite: extension_dependencies_test.go.
func TestExtensionDependencyInstall(t *testing.T) {
	t.Parallel()

	t.Run("Should refuse unmet dependencies without consent and install nothing", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "1.0.0")
		fixture.write(t, "root", "1.0.0", "dep-core")

		_, err := fixture.service.Install(t.Context(), fixture.rootRequest(t, nil), dependencyOperator(t))
		required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
		if !ok || len(required.Dependencies) != 1 || required.Dependencies[0].Name != "dep-core" ||
			required.Dependencies[0].State != dependencyStateMissing {
			t.Fatalf("error = %v, want extension_dependencies_required listing dep-core", err)
		}
		if fixture.installed(t, "root") || fixture.installed(t, "dep-core") {
			t.Fatal("refused install left a package installed")
		}
	})

	t.Run("Should install missing and enable disabled dependencies before the dependent", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "1.0.0")
		fixture.write(t, "dep-off", "1.0.0")
		fixture.install(t, "dep-off", contract.InstallExtensionRequest{})
		if err := fixture.registry.Disable("dep-off"); err != nil {
			t.Fatal(err)
		}
		fixture.write(t, "root", "1.0.0", "dep-core", "dep-off")

		item, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{"dep-core": {AllowUnverified: true}},
		), dependencyOperator(t))
		if err != nil {
			t.Fatalf("Install() error = %v", err)
		}
		if item.Name != "root" || !slices.Equal(item.InstallDependenciesInstalled, []string{"dep-core", "dep-off"}) {
			t.Fatalf("install = %s, dependencies_installed = %v", item.Name, item.InstallDependenciesInstalled)
		}
		for _, name := range []string{"dep-core", "dep-off"} {
			enabled, err := fixture.registry.IsEnabledForProfile(name, extensionDefaultProfileLens().ID)
			if err != nil || !enabled {
				t.Fatalf("%s enabled = %t, %v", name, enabled, err)
			}
		}
	})

	t.Run("Should install and enable with automation disabled", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		// [automation] enabled = false wires no automation previewer into the extension service.
		fixture.service.automation = nil
		fixture.write(t, "dep-core", "1.0.0")
		fixture.write(t, "root", "1.0.0", "dep-core")
		fixture.write(t, "solo", "1.0.0")

		item, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{"dep-core": {AllowUnverified: true}},
		), dependencyOperator(t))
		if err != nil || !slices.Equal(item.InstallDependenciesInstalled, []string{"dep-core"}) {
			t.Fatalf("Install(root) = %v, %v; want dep-core installed and enabled first",
				item.InstallDependenciesInstalled, err)
		}
		solo, err := fixture.service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath, Ref: fixture.dir(t, "solo"), AllowUnverified: true,
			Enable: true,
		}, dependencyOperator(t))
		if err != nil || solo.InstallEnableError != nil || !solo.Enabled {
			t.Fatalf("Install(solo --enable) = %+v, %v; want enabled without an enable error", solo, err)
		}
		for _, name := range []string{"dep-core", "solo"} {
			enabled, err := fixture.registry.IsEnabledForProfile(name, extensionDefaultProfileLens().ID)
			if err != nil || !enabled {
				t.Fatalf("%s enabled = %t, %v", name, enabled, err)
			}
		}
	})

	t.Run("Should refuse a dependency without its own unverified consent", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "1.0.0")
		fixture.write(t, "root", "1.0.0", "dep-core")

		_, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{}), dependencyOperator(t))
		gate, ok := errors.AsType[*extensionpkg.DependencyGateError](err)
		if !ok || gate.Dependency != "dep-core" || !errors.Is(err, extensionpkg.ErrExtensionChecksumUnverified) {
			t.Fatalf("error = %v, want checksum_unverified naming dep-core", err)
		}
		if fixture.installed(t, "root") || fixture.installed(t, "dep-core") {
			t.Fatal("refused install left a package installed")
		}
	})

	t.Run("Should refuse a missing root gateway confirmation before installing any dependency", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "1.0.0")
		appendDependencyManifest(t, fixture.write(t, "root", "1.0.0", "dep-core"),
			"\n[gateway]\npermissions = [\"gateway.private\"]\n")

		_, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{"dep-core": {AllowUnverified: true}},
		), dependencyOperator(t))
		if !errors.Is(err, extensionpkg.ErrExtensionGatewayConfirmationRequired) {
			t.Fatalf("error = %v, want the root gateway confirmation refusal", err)
		}
		if _, partial := errors.AsType[*extensionpkg.DependenciesInstalledError](err); partial {
			t.Fatalf("error = %v reports installed dependencies", err)
		}
		if fixture.installed(t, "dep-core") || fixture.installed(t, "root") {
			t.Fatal("root gateway refusal left a package installed")
		}
	})

	t.Run("Should refuse a version_unmet dependency even with consent", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "0.1.0")
		fixture.install(t, "dep-core", contract.InstallExtensionRequest{})
		fixture.write(t, "root", "1.0.0", "dep-core>=0.4.0")

		_, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{"dep-core": {AllowUnverified: true}},
		), dependencyOperator(t))
		required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
		if !ok || required.Dependencies[0].State != dependencyStateVersionUnmet ||
			required.Dependencies[0].RetryCommand != "" {
			t.Fatalf("error = %v, want version_unmet without an update command for a local package", err)
		}
		info, err := fixture.registry.Get("dep-core")
		if err != nil || info.Version != "0.1.0" || fixture.installed(t, "root") {
			t.Fatalf("dep-core = %+v, %v; root installed = %t", info, err, fixture.installed(t, "root"))
		}
	})

	t.Run("Should stop at the first failure and report completed dependencies", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-first", "1.0.0")
		fixture.write(t, "dep-second", "1.0.0")
		fixture.write(t, "root", "1.0.0", "dep-first", "dep-second")

		_, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{"dep-first": {AllowUnverified: true}},
		), dependencyOperator(t))
		partial, ok := errors.AsType[*extensionpkg.DependenciesInstalledError](err)
		gate, gated := errors.AsType[*extensionpkg.DependencyGateError](err)
		if !ok || !slices.Equal(partial.Installed, []string{"dep-first"}) || !gated || gate.Dependency != "dep-second" {
			t.Fatalf("error = %v, want dep-second refused after installing dep-first", err)
		}
		if !fixture.installed(t, "dep-first") || fixture.installed(t, "dep-second") || fixture.installed(t, "root") {
			t.Fatal("partial install state differs from stop-at-first-failure")
		}
	})

	t.Run("Should resolve a relative local_path ref against the dependent's directory", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		// The daemon's working directory is this package; ../dep-core must not resolve against it.
		parent := t.TempDir()
		fixture.dirs["dep-core"], fixture.dirs["root"] = filepath.Join(
			parent,
			"dep-core",
		), filepath.Join(
			parent,
			"root",
		)
		for _, dir := range []string{fixture.dirs["dep-core"], fixture.dirs["root"]} {
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
		}
		fixture.write(t, "dep-core", "1.0.0")
		writeDaemonFile(t, filepath.Join(fixture.dirs["root"], "extension.toml"), fmt.Sprintf(
			"name = \"root\"\nversion = \"1.0.0\"\nmin_compozy_version = %q\n\n[[requires.extensions]]\n"+
				"name = \"dep-core\"\nsource = \"local_path\"\nref = \"../dep-core\"\n", dependencyFixtureMinVersion))

		item, err := fixture.service.Install(t.Context(), fixture.rootRequest(t,
			map[string]contract.ExtensionDependencyConsent{"dep-core": {AllowUnverified: true}},
		), dependencyOperator(t))
		if err != nil || !slices.Equal(item.InstallDependenciesInstalled, []string{"dep-core"}) {
			t.Fatalf("Install() = %v, %v; want dep-core installed from the sibling directory",
				item.InstallDependenciesInstalled, err)
		}
		_, err = fixture.resolve(t, dependencyRoot(extensionpkg.ExtensionDependency{
			Name: "dep-other", Source: string(contract.InstallExtensionSourceLocalPath), Ref: "../dep-other",
		}), extensionMutationTarget{profile: extensionDefaultProfileLens()})
		gate, ok := errors.AsType[*extensionpkg.DependencyGateError](err)
		if !ok || gate.Dependency != "dep-other" || !errors.Is(err, extensionpkg.ErrManifestInvalid) {
			t.Fatalf("error = %v, want a relative ref without a local dependent refused naming dep-other", err)
		}
	})

	t.Run("Should report a dependency kept installed after its enable failed", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		appendDependencyManifest(t, fixture.write(t, "dep-env", "1.0.0"),
			"requires_env = [\"COMPOZY_DEPENDENCY_TEST_NEVER_SET\"]\n")
		fixture.write(t, "root", "1.0.0", "dep-env")
		other, err := fixture.deps.ProfileManager.Create(t.Context(), profilepkg.CreateInput{Name: "other"})
		if err != nil {
			t.Fatal(err)
		}
		req := fixture.rootRequest(
			t,
			map[string]contract.ExtensionDependencyConsent{"dep-env": {AllowUnverified: true}},
		)
		req.Profile = other.Name

		_, err = fixture.service.Install(t.Context(), req, dependencyOperator(t))
		partial, ok := errors.AsType[*extensionpkg.DependenciesInstalledError](err)
		gate, gated := errors.AsType[*extensionpkg.DependencyGateError](err)
		if !ok || !slices.Equal(partial.Installed, []string{"dep-env"}) || !gated || gate.Dependency != "dep-env" {
			t.Fatalf("error = %v, want dep-env failed to enable and reported installed", err)
		}
		if !fixture.installed(t, "dep-env") || fixture.installed(t, "root") {
			t.Fatal("enable failure state differs: dep-env must stay installed and root must not install")
		}
	})

	t.Run("Should refuse committing a dependent whose dependency was removed after resolution", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "1.0.0")
		fixture.install(t, "dep-core", contract.InstallExtensionRequest{})
		fixture.write(t, "root", "1.0.0", "dep-core")
		req := fixture.rootRequest(t, nil)
		target, err := fixture.service.resolveExtensionScope(t.Context(), "", "", "", dependencyOperator(t))
		if err != nil {
			t.Fatal(err)
		}
		prepared, err := fixture.service.prepareExtensionInstall(t.Context(), req, dependencyOperator(t), "operator",
			target)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.service.installDependencies(
			t.Context(),
			prepared,
			req,
			dependencyOperator(t),
		); err != nil {
			t.Fatalf("installDependencies() error = %v", err)
		}
		// The interleaving: the guard sees no installed dependent yet, so the remove proceeds unforced.
		if _, err := fixture.service.Remove(t.Context(), "dep-core", dependencyOperator(t)); err != nil {
			t.Fatalf("Remove(dep-core) error = %v", err)
		}

		_, err = fixture.service.installPrepared(t.Context(), prepared, req, dependencyOperator(t),
			extensionpkg.LifecycleEvent{Type: eventspkg.ExtensionInstallFailed, ExtensionName: "root"})
		required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
		if !ok || len(required.Dependencies) != 1 || required.Dependencies[0].Name != "dep-core" {
			t.Fatalf("error = %v, want the commit refused with dep-core unmet", err)
		}
		if fixture.installed(t, "root") {
			t.Fatal("root committed with an unmet dependency")
		}
	})

	t.Run("Should preview and install a marketplace dependency from its listed digest", func(t *testing.T) {
		t.Parallel()
		market := newMarketplaceDependencyService(t)
		req := market.rootRequest(t)

		preview, err := market.service.PreviewInstall(t.Context(), req, dependencyOperator(t))
		if err != nil || len(preview.Dependencies) != 1 || preview.Dependencies[0].DigestSHA256 != market.digest ||
			preview.Dependencies[0].State != dependencyStateMissing {
			t.Fatalf("preview = %+v, %v; want open-design missing with the listed digest", preview.Dependencies, err)
		}
		consent := contract.ExtensionDependencyConsent{AllowUnverified: true}
		req.WithDependencies = true
		req.Dependencies = map[string]contract.ExtensionDependencyConsent{"open-design": consent}
		_, err = market.service.Install(t.Context(), req, dependencyOperator(t))
		gate, ok := errors.AsType[*extensionpkg.DependencyGateError](err)
		if !ok || gate.Dependency != "open-design" || !strings.Contains(err.Error(), "expected_digest") {
			t.Fatalf("error = %v, want the reviewed digest required for open-design", err)
		}
		consent.ExpectedDigest = market.digest
		req.Dependencies["open-design"] = consent
		item, err := market.service.Install(t.Context(), req, dependencyOperator(t))
		if err != nil || !slices.Equal(item.InstallDependenciesInstalled, []string{"open-design"}) {
			t.Fatalf("Install() = %v, %v; want open-design installed first", item.InstallDependenciesInstalled, err)
		}
	})

	t.Run("Should report a package another profile uses and refuse before installing any dependency",
		func(t *testing.T) {
			t.Parallel()
			market := newMarketplaceDependencyService(t)
			other, err := market.service.profiles.Create(t.Context(), profilepkg.CreateInput{Name: "other"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := market.service.Install(t.Context(), contract.InstallExtensionRequest{
				Source: contract.InstallExtensionSourceMarketplace, Ref: "team/design", Profile: other.Name,
				ExpectedDigest: market.digest, AllowUnverified: true,
			}, dependencyOperator(t)); err != nil {
				t.Fatalf("install open-design for other error = %v", err)
			}
			newer := market.publish(t, "1.1.0")
			// dep-first is declared before open-design, so stop-at-first-failure would have installed it first.
			firstDir, rootDir := t.TempDir(), t.TempDir()
			writeDaemonFile(t, filepath.Join(firstDir, "extension.toml"), fmt.Sprintf(
				"name = \"dep-first\"\nversion = \"1.0.0\"\nmin_compozy_version = %q\n", dependencyFixtureMinVersion))
			writeDaemonFile(t, filepath.Join(rootDir, "extension.toml"), fmt.Sprintf(
				"name = \"root\"\nversion = \"1.0.0\"\nmin_compozy_version = %q\n\n"+
					"[[requires.extensions]]\nname = \"dep-first\"\nsource = \"local_path\"\nref = %q\n\n"+
					"[[requires.extensions]]\nname = \"open-design\"\nsource = \"marketplace\"\nref = \"team/design\"\n",
				dependencyFixtureMinVersion, firstDir))
			req := contract.InstallExtensionRequest{
				Source: contract.InstallExtensionSourceLocalPath, Ref: rootDir, AllowUnverified: true,
			}
			command := "compozy extension update open-design --scope global --profile other"

			preview, err := market.service.PreviewInstall(t.Context(), req, dependencyOperator(t))
			states := map[string]contract.ExtensionDependencyPayload{}
			for _, item := range preview.Dependencies {
				states[item.Name] = item
			}
			design := states["open-design"]
			if err != nil || design.State != dependencyStateInstalledElsewhere || design.InstalledVersion != "1.0.0" ||
				design.RetryCommand != command {
				t.Fatalf("preview = %+v, %v; want open-design installed_elsewhere naming %q", preview.Dependencies,
					err, command)
			}
			req.WithDependencies = true
			req.Dependencies = map[string]contract.ExtensionDependencyConsent{
				"open-design": {AllowUnverified: true, ExpectedDigest: newer},
				"dep-first":   {AllowUnverified: true},
			}

			_, err = market.service.Install(t.Context(), req, dependencyOperator(t))
			required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
			if !ok || !slices.ContainsFunc(required.Dependencies, func(item contract.ExtensionDependencyPayload) bool {
				return item.Name == "open-design" && item.RetryCommand == command
			}) {
				t.Fatalf("error = %v, want extension_dependencies_required naming %q", err, command)
			}
			info, err := market.service.registry.Get("open-design")
			if err != nil || info.Version != "1.0.0" {
				t.Fatalf("open-design = %+v, %v; want 1.0.0 kept for the other profile", info, err)
			}
			for _, name := range []string{"dep-first", "root"} {
				if _, err := market.service.registry.Get(name); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
					t.Fatalf("%s lookup error = %v, want not installed", name, err)
				}
			}
		})
}

// Invariant: a retry command must work: a package `extension update` cannot reach is reinstalled from the
// dependency's own source in the blocking cell, an update selects the installation the dependent resolves (G-9), and
// a failed profile or stalled update lookup only drops the command, never the refusal (G-7). Owner: daemon dependency resolver; canonical suite: this file.
func TestExtensionDependencyRetryCommand(t *testing.T) {
	t.Parallel()

	t.Run("Should name a local reinstall that unblocks the root by attaching the same bytes",
		func(t *testing.T) {
			t.Parallel()
			fixture := newDependencyFixture(t)
			other, err := fixture.deps.ProfileManager.Create(t.Context(), profilepkg.CreateInput{Name: "other"})
			if err != nil {
				t.Fatal(err)
			}
			fixture.write(t, "dep-core", "1.0.0")
			fixture.install(t, "dep-core", contract.InstallExtensionRequest{Profile: other.Name})
			fixture.write(t, "dep-core", "1.1.0")
			fixture.write(t, "root", "1.0.0", "dep-core>=1.1.0")
			req := fixture.rootRequest(t,
				map[string]contract.ExtensionDependencyConsent{"dep-core": {AllowUnverified: true}})

			_, err = fixture.service.Install(t.Context(), req, dependencyOperator(t))
			required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
			want := "compozy extension install " + fixture.dir(t, "dep-core") +
				" --replace --allow-unverified --scope global --profile other"
			if !ok || required.Dependencies[0].State != dependencyStateInstalledElsewhere ||
				required.Dependencies[0].RetryCommand != want {
				t.Fatalf("error = %v, want installed_elsewhere naming %q", err, want)
			}

			// The offered command: replace the other profile's copy from the dependency's source.
			fixture.install(t, "dep-core", contract.InstallExtensionRequest{
				Scope: "global", Profile: other.Name, Replace: true,
			})
			if _, err := fixture.service.Install(t.Context(), req, dependencyOperator(t)); err != nil {
				t.Fatalf("Install() after the retry command error = %v, want the root installed", err)
			}
			installations, err := fixture.registry.Installations(t.Context(), "dep-core")
			if err != nil || len(installations) != 2 {
				t.Fatalf("dep-core installations = %+v, %v; want other and the root's profile", installations, err)
			}
			if !fixture.installed(t, "root") {
				t.Fatal("root is not installed after the retry command")
			}
		})

	t.Run("Should drop the command when the blocking cell profile cannot be read", func(t *testing.T) {
		t.Parallel()
		fixture := newDependencyFixture(t)
		command := fixture.service.dependencyRetryCommand(t.Context(), contract.ExtensionDependencyPayload{
			Name: "dep-core", Source: string(contract.InstallExtensionSourceLocalPath), Ref: fixture.dir(t, "dep-core"),
			Version: "1.1.0", State: dependencyStateInstalledElsewhere,
		}, &extensionpkg.InstallationScope{ProfileID: "profile-missing"})
		if command != "" {
			t.Fatalf("dependencyRetryCommand() = %q, want no command without the profile name", command)
		}
	})

	t.Run("Should scope the update command to the global installation a workspace dependent resolves",
		func(t *testing.T) {
			t.Parallel()
			market := newMarketplaceDependencyService(t)
			market.installGlobalDesign(t)
			cell := extensionpkg.InstallationScope{
				ProfileID: extensionDefaultProfileLens().ID, WorkspaceID: "ws-dependent",
			}
			want := "compozy extension update open-design --scope global"

			design := market.workspaceDesignDependency(t, cell)
			if design.State != dependencyStateVersionUnmet || design.RetryCommand != want {
				t.Fatalf("install hint = %+v, want version_unmet naming %q", design, want)
			}
			unmet, err := market.service.cellUnmetDependencies(t.Context(), "root",
				[]extensionpkg.ExtensionDependency{designDependencyAtLeast("1.1.0")}, cell)
			if err != nil || len(unmet) != 1 {
				t.Fatalf("cellUnmetDependencies() = %+v, %v; want open-design unmet", unmet, err)
			}
			market.service.addDependencyRetryCommands(t.Context(), unmet, cell)
			if unmet[0].RetryCommand != want {
				t.Fatalf("enable guidance = %+v, want %q", unmet[0], want)
			}
		})

	t.Run("Should drop the command and keep the refusal when the update lookup stalls", func(t *testing.T) {
		t.Parallel()
		market := newMarketplaceDependencyService(t)
		market.installGlobalDesign(t)
		market.service.marketplaceCatalog = stalledEntryCatalog{Service: market.service.marketplaceCatalog}
		market.service.dependencyUpdateHintTimeout = 50 * time.Millisecond
		cell := extensionpkg.InstallationScope{
			ProfileID: extensionDefaultProfileLens().ID, WorkspaceID: "ws-dependent",
		}

		design := market.workspaceDesignDependency(t, cell)
		if design.State != dependencyStateVersionUnmet || design.RetryCommand != "" {
			t.Fatalf("install hint = %+v, want version_unmet without a command", design)
		}
		rootDir := t.TempDir()
		writeDaemonFile(t, filepath.Join(rootDir, "extension.toml"), fmt.Sprintf(
			"name = \"root\"\nversion = \"1.0.0\"\nmin_compozy_version = %q\n\n[[requires.extensions]]\n"+
				"name = \"open-design\"\nsource = \"marketplace\"\nref = \"team/design\"\nmin_version = \"1.1.0\"\n",
			dependencyFixtureMinVersion))
		_, err := market.service.Install(t.Context(), contract.InstallExtensionRequest{
			Source: contract.InstallExtensionSourceLocalPath, Ref: rootDir, AllowUnverified: true,
			WithDependencies: true,
			Dependencies:     map[string]contract.ExtensionDependencyConsent{"open-design": {AllowUnverified: true}},
		}, dependencyOperator(t))
		required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
		if !ok || len(required.Dependencies) != 1 || required.Dependencies[0].State != dependencyStateVersionUnmet ||
			required.Dependencies[0].RetryCommand != "" {
			t.Fatalf("error = %v, want extension_dependencies_required for open-design without a command", err)
		}
	})
}

// stalledEntryCatalog blocks every catalog entry lookup until its context ends, like an unresponsive source.
type stalledEntryCatalog struct{ marketplace.Service }

func (stalledEntryCatalog) Entry(ctx context.Context, _ marketplace.Origin) (*marketplace.Entry, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func designDependencyAtLeast(minVersion string) extensionpkg.ExtensionDependency {
	return extensionpkg.ExtensionDependency{
		Name: "open-design", Source: string(contract.InstallExtensionSourceMarketplace), Ref: "team/design",
		MinVersion: minVersion,
	}
}

// installGlobalDesign installs open-design 1.0.0 globally for every profile, then lists 1.1.0.
func (m *marketplaceDependencyFixture) installGlobalDesign(t *testing.T) {
	t.Helper()
	if _, err := m.service.Install(t.Context(), contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceMarketplace, Ref: "team/design", ExpectedDigest: m.digest,
		AllowUnverified: true,
	}, dependencyOperator(t)); err != nil {
		t.Fatalf("install open-design globally error = %v", err)
	}
	m.publish(t, "1.1.0")
}

// workspaceDesignDependency resolves a root requiring open-design>=1.1.0 for the dependent's cell.
func (m *marketplaceDependencyFixture) workspaceDesignDependency(
	t *testing.T, cell extensionpkg.InstallationScope,
) contract.ExtensionDependencyPayload {
	t.Helper()
	target := extensionMutationTarget{scope: cell, profile: extensionDefaultProfileLens()}
	graph, err := m.service.resolveDependencies(t.Context(), dependencyRoot(designDependencyAtLeast("1.1.0")), "",
		target, nil, dependencyOperator(t))
	if err != nil {
		t.Fatalf("resolveDependencies() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := graph.Close(); closeErr != nil {
			t.Errorf("graph.Close() error = %v", closeErr)
		}
	})
	items := graph.payloads()
	if len(items) != 1 {
		t.Fatalf("dependencies = %+v, want open-design only", items)
	}
	return items[0]
}

// marketplaceDependencyFixture serves the open-design client plugin from a file marketplace source "team".
type marketplaceDependencyFixture struct {
	service   *daemonExtensionService
	runtime   *marketplaceRuntime
	pluginDir string
	digest    string
}

// rootRequest installs a local root requiring team/design as open-design.
func (m *marketplaceDependencyFixture) rootRequest(t *testing.T) contract.InstallExtensionRequest {
	t.Helper()
	rootDir := t.TempDir()
	writeDaemonFile(t, filepath.Join(rootDir, "extension.toml"), fmt.Sprintf(
		"name = \"root\"\nversion = \"1.0.0\"\nmin_compozy_version = %q\n\n[[requires.extensions]]\n"+
			"name = \"open-design\"\nsource = \"marketplace\"\nref = \"team/design\"\n",
		dependencyFixtureMinVersion))
	return contract.InstallExtensionRequest{
		Source: contract.InstallExtensionSourceLocalPath, Ref: rootDir, AllowUnverified: true,
	}
}

// publish changes the plugin version, refreshes the source and returns the newly listed digest.
func (m *marketplaceDependencyFixture) publish(t *testing.T, version string) string {
	t.Helper()
	path := filepath.Join(m.pluginDir, ".claude-plugin", "plugin.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeDaemonFile(t, path, strings.Replace(string(data), `"version": "1.0.0"`, `"version": "`+version+`"`, 1))
	if _, err := m.runtime.Refresh(t.Context(), "team"); err != nil {
		t.Fatal(err)
	}
	entry, err := m.runtime.Detail(t.Context(), "team", "design")
	if err != nil || entry == nil || entry.DigestSHA256 == m.digest {
		t.Fatalf("republished entry = %+v, %v; want a new digest", entry, err)
	}
	return entry.DigestSHA256
}

func newMarketplaceDependencyService(t *testing.T) *marketplaceDependencyFixture {
	t.Helper()
	ctx := t.Context()
	root := t.TempDir()
	if err := os.CopyFS(filepath.Join(root, "plugin"),
		os.DirFS(filepath.Join("..", "extension", "testdata", "client-plugins", "open-design"))); err != nil {
		t.Fatal(err)
	}
	writeDaemonFile(t, filepath.Join(root, "marketplace.json"), `{"plugins":[{"name":"design","source":"./plugin"}]}`)
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := fmt.Fprint(
			w,
			`{"manifest_version":3,"generated_at":"2026-07-13T12:00:00Z","entries":[]}`,
		); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(feed.Close)
	home := testHomePaths(t)
	cfg := testConfig(t, home)
	cfg.Extensions.Trust.AllowUnverified = true
	cfg.Marketplace.Catalog.BaseURL = feed.URL
	cfg.Marketplace.PluginSources = []compozyconfig.MarketplacePluginSourceConfig{
		{Name: "team", Source: (&url.URL{Scheme: "file", Path: root}).String()},
	}
	db := openDaemonTestGlobalDB(t)
	catalog, err := marketplace.NewSQLiteStore(db)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := newMarketplaceRuntime(ctx, catalog, nil, cfg.Marketplace, home, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := runtime.Shutdown(context.WithoutCancel(ctx)); err != nil {
			t.Error(err)
		}
	})
	if _, err := runtime.Refresh(ctx, "team"); err != nil {
		t.Fatal(err)
	}
	entry, err := runtime.Detail(ctx, "team", "design")
	if err != nil || entry == nil || !entry.Installable {
		t.Fatalf("catalog entry = %+v, %v", entry, err)
	}
	profiles, err := profilepkg.NewManager(profilepkg.WithStore(db), profilepkg.WithHomePaths(home))
	if err != nil {
		t.Fatal(err)
	}
	registry := extensionpkg.NewRegistry(db.DB())
	service := newDaemonExtensionService(&daemonExtensionServiceDeps{
		Registry: registry, Runtime: newLifecycleStateRuntime(registry), Profiles: profiles, HomePaths: home,
		Logger: discardLogger(),
	}, withDaemonExtensionMarketplace(cfg.Extensions, nil), withDaemonExtensionCatalog(runtime),
		withDaemonMarketplacePackageAcquirer(runtime.resolver),
		withDaemonMarketplacePackageCache(runtime.resolver.Cache),
		withDaemonExtensionAutomation(&fakeAutomationManager{})).(*daemonExtensionService)
	return &marketplaceDependencyFixture{
		service: service, runtime: runtime, pluginDir: filepath.Join(root, "plugin"), digest: entry.DigestSHA256,
	}
}

// Invariant: removing or disabling a dependency an installed dependent still resolves refuses with the dependents
// unless forced; force never cascades, the dependent keeps its runtime state and reports unmet_dependencies, and
// enabling a dependent with unmet dependencies refuses with the install payload (rules 14 and 15).
// Owner: daemon dependency guard. Canonical suite: extension_dependencies_test.go.
func TestExtensionDependencyGuard(t *testing.T) {
	t.Parallel()

	installPair := func(t *testing.T) *dependencyFixture {
		t.Helper()
		fixture := newDependencyFixture(t)
		fixture.write(t, "dep-core", "1.0.0")
		fixture.install(t, "dep-core", contract.InstallExtensionRequest{})
		fixture.write(t, "root", "1.0.0", "dep-core")
		fixture.install(t, "root", contract.InstallExtensionRequest{})
		return fixture
	}
	defaultProfile := extensionDefaultProfileLens()
	disable := func(force bool) contract.SetExtensionEnablementRequest {
		return contract.SetExtensionEnablementRequest{Profile: defaultProfile.Name, Enabled: false, Force: force}
	}
	assertUnmet := func(t *testing.T, fixture *dependencyFixture, state string) {
		t.Helper()
		status, err := fixture.service.Status(t.Context(), "root")
		if err != nil {
			t.Fatalf("Status(root) error = %v", err)
		}
		if len(status.UnmetDependencies) != 1 || status.UnmetDependencies[0].Name != "dep-core" ||
			status.UnmetDependencies[0].State != state {
			t.Fatalf("root unmet_dependencies = %+v, want dep-core %s", status.UnmetDependencies, state)
		}
		enabled, err := fixture.registry.IsEnabledForProfile("root", defaultProfile.ID)
		if err != nil || !enabled {
			t.Fatalf("root enabled = %t, %v; the dependent's runtime state must not change", enabled, err)
		}
	}

	t.Run("Should refuse removing a needed dependency and list its dependents", func(t *testing.T) {
		t.Parallel()
		fixture := installPair(t)

		_, err := fixture.service.Remove(t.Context(), "dep-core", dependencyOperator(t))
		inUse, ok := errors.AsType[*extensionpkg.DependencyInUseError](err)
		want := []contract.ExtensionDependentPayload{{Name: "root"}}
		if !ok || !errors.Is(err, extensionpkg.ErrExtensionDependencyInUse) ||
			!reflect.DeepEqual(inUse.Dependents, want) {
			t.Fatalf("error = %v, want extension_dependency_in_use listing root", err)
		}
		if !fixture.installed(t, "dep-core") {
			t.Fatal("refused removal uninstalled dep-core")
		}
		status, err := fixture.service.Status(t.Context(), "root")
		if err != nil || len(status.UnmetDependencies) != 0 {
			t.Fatalf("met dependent unmet_dependencies = %+v, %v; want none", status.UnmetDependencies, err)
		}
	})

	t.Run("Should remove with force without cascading and report the dependent unmet", func(t *testing.T) {
		t.Parallel()
		fixture := installPair(t)

		ctx := extensionpkg.WithDependencyForce(t.Context(), true)
		if _, err := fixture.service.Remove(ctx, "dep-core", dependencyOperator(t)); err != nil {
			t.Fatalf("forced Remove(dep-core) error = %v", err)
		}
		if fixture.installed(t, "dep-core") || !fixture.installed(t, "root") {
			t.Fatal("forced removal did not remove only dep-core")
		}
		assertUnmet(t, fixture, dependencyStateMissing)
	})

	t.Run("Should refuse disabling a needed dependency for the profile unless forced", func(t *testing.T) {
		t.Parallel()
		fixture := installPair(t)

		_, err := fixture.service.SetEnablement(t.Context(), "dep-core", disable(false), dependencyOperator(t))
		inUse, ok := errors.AsType[*extensionpkg.DependencyInUseError](err)
		want := []contract.ExtensionDependentPayload{{Name: "root", Profile: defaultProfile.Name}}
		if !ok || !reflect.DeepEqual(inUse.Dependents, want) {
			t.Fatalf("error = %v, want extension_dependency_in_use listing root in %s", err, defaultProfile.Name)
		}
		if enabled, err := fixture.registry.IsEnabledForProfile("dep-core", defaultProfile.ID); err != nil || !enabled {
			t.Fatalf("refused disable changed dep-core: enabled = %t, %v", enabled, err)
		}
		_, err = fixture.service.SetEnablement(t.Context(), "dep-core", disable(true), dependencyOperator(t))
		if err != nil {
			t.Fatalf("forced disable error = %v", err)
		}
		assertUnmet(t, fixture, dependencyStateDisabled)
	})

	t.Run("Should refuse enabling a dependent whose dependency is unmet", func(t *testing.T) {
		t.Parallel()
		fixture := installPair(t)
		// The dependent goes first; an installed dependent still guards its dependency, so that needs force.
		for _, name := range []string{"root", "dep-core"} {
			req := disable(name == "dep-core")
			if _, err := fixture.service.SetEnablement(t.Context(), name, req, dependencyOperator(t)); err != nil {
				t.Fatalf("disable %s error = %v", name, err)
			}
		}

		_, err := fixture.service.SetEnablement(t.Context(), "root", contract.SetExtensionEnablementRequest{
			Profile: defaultProfile.Name, Enabled: true,
		}, dependencyOperator(t))
		required, ok := errors.AsType[*extensionpkg.DependenciesRequiredError](err)
		if !ok || len(required.Dependencies) != 1 || required.Dependencies[0].Name != "dep-core" ||
			required.Dependencies[0].State != dependencyStateDisabled ||
			!slices.Equal(required.Dependencies[0].RequiredBy, []string{"root"}) {
			t.Fatalf("error = %v, want extension_dependencies_required listing disabled dep-core", err)
		}
		if enabled, err := fixture.registry.IsEnabledForProfile("root", defaultProfile.ID); err != nil || enabled {
			t.Fatalf("refused enable changed root: enabled = %t, %v", enabled, err)
		}
	})
}
