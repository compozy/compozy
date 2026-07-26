package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	aghdaemon "github.com/compozy/agh/internal/daemon"
	extensionpkg "github.com/compozy/agh/internal/extension"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
)

type extensionFixtureOptions struct {
	capabilities []string
	actions      []string
	requiresEnv  []string
}

func TestExtensionInstallOfflinePersistsExtension(t *testing.T) {
	t.Parallel()

	deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "alpha-ext", extensionFixtureOptions{})

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		dir,
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension install error = %v", err)
	}

	var item ExtensionRecord
	if err := json.Unmarshal([]byte(stdout), &item); err != nil {
		t.Fatalf("json.Unmarshal(install) error = %v", err)
	}
	if item.Name != "alpha-ext" || item.DaemonRunning {
		t.Fatalf("install payload = %#v, want local installed extension", item)
	}

	info := getInstalledExtension(t, homePaths, "alpha-ext")
	if !info.Enabled {
		t.Fatalf("installed extension enabled = false, want true")
	}
	if !info.Provenance.AllowUnverified {
		t.Fatalf("installed provenance allow_unverified = false, want true")
	}
}

func TestExtensionInstallOfflineRequiresSideLoadPolicy(t *testing.T) {
	t.Parallel()
	t.Run("Should reject an offline side-load when live policy is disabled", func(t *testing.T) {
		t.Parallel()

		deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
		deps.loadConfig = func() (aghconfig.Config, error) {
			return aghconfig.DefaultWithHome(homePaths), nil
		}
		dir := writeExtensionFixture(t, "blocked-offline-ext", extensionFixtureOptions{})

		_, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"install",
			dir,
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		)
		if !errors.Is(err, extensionpkg.ErrExtensionUnverifiedPolicyBlocked) {
			t.Fatalf("extension install policy error = %v, want ErrExtensionUnverifiedPolicyBlocked", err)
		}
	})
}

func TestPrepareExtensionInstallMissingDirectory(t *testing.T) {
	t.Parallel()

	_, err := prepareExtensionInstall(filepath.Join(t.TempDir(), "missing"))
	if err == nil || !strings.Contains(err.Error(), "stat install path") {
		t.Fatalf("prepareExtensionInstall(missing) error = %v, want stat install path", err)
	}
}

func TestExtensionInstallOfflineInvalidManifest(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	dir := t.TempDir()
	writeExtensionManifest(t, filepath.Join(dir, "extension.toml"), `[extension]
version = "0.1.0"
description = "broken"
min_agh_version = "0.5.0"

[resources]
`)

	_, _, err := executeRootCommand(t, deps, "extension", "install", dir, "-o", "json")
	if err == nil || !errors.Is(err, extensionpkg.ErrManifestInvalid) {
		t.Fatalf("extension install invalid manifest error = %v, want ErrManifestInvalid", err)
	}
}

func TestInstallPreparedExtensionDetectsChecksumMismatch(t *testing.T) {
	t.Parallel()

	_, homePaths := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "checksum-ext", extensionFixtureOptions{})
	prepared, err := prepareExtensionInstall(dir)
	if err != nil {
		t.Fatalf("prepareExtensionInstall() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(README.md) error = %v", err)
	}

	registry, cleanup := openExtensionRegistry(t, homePaths)
	defer cleanup()

	if err := installPreparedExtension(
		homePaths,
		registry,
		prepared,
		fixedTestNow,
		true,
	); err == nil ||
		!errors.Is(err, extensionpkg.ErrExtensionChecksumMismatch) {
		t.Fatalf("installPreparedExtension(checksum mismatch) error = %v, want ErrExtensionChecksumMismatch", err)
	}
}

func TestExtensionInstallAndRemoveOfflinePreservesSourceDirectory(t *testing.T) {
	t.Parallel()

	deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
	sourceDir := writeExtensionFixture(t, "local-remove-ext", extensionFixtureOptions{})

	if _, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		sourceDir,
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("extension install error = %v", err)
	}

	info := getInstalledExtension(t, homePaths, "local-remove-ext")
	wantManifestPath := filepath.Join(extensionpkg.ManagedInstallPath(homePaths, "local-remove-ext"), "extension.toml")
	if info.ManifestPath != wantManifestPath {
		t.Fatalf("installed manifest path = %q, want %q", info.ManifestPath, wantManifestPath)
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "extension.toml")); err != nil {
		t.Fatalf("source manifest stat after install error = %v", err)
	}

	if _, _, err := executeRootCommand(t, deps, "extension", "remove", "local-remove-ext", "-o", "json"); err == nil ||
		!strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension remove offline error = %v, want running daemon requirement", err)
	}

	if _, err := os.Stat(filepath.Join(sourceDir, "extension.toml")); err != nil {
		t.Fatalf("source manifest stat after offline remove rejection error = %v", err)
	}
	if _, err := os.Stat(
		extensionpkg.ManagedInstallPath(homePaths, "local-remove-ext"),
	); err != nil {
		t.Fatalf("managed install dir stat after offline remove rejection error = %v", err)
	}
}

func TestExtensionListFormatsOffline(t *testing.T) {
	t.Parallel()

	deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "list-ext", extensionFixtureOptions{
		capabilities: []string{"memory.backend"},
	})
	installExtensionFixture(t, homePaths, dir)

	t.Run("Should human", func(t *testing.T) {
		stdout, _, err := executeRootCommand(t, deps, "extension", "list", "-o", "human")
		if err != nil {
			t.Fatalf("extension list human error = %v", err)
		}
		for _, token := range []string{
			"Extensions",
			"Name",
			"Version",
			"Type",
			"State",
			"Capabilities",
			"list-ext",
			"memory.backend",
		} {
			if !strings.Contains(stdout, token) {
				t.Fatalf("human output missing %q: %s", token, stdout)
			}
		}
	})

	t.Run("Should json", func(t *testing.T) {
		stdout, _, err := executeRootCommand(t, deps, "extension", "list", "-o", "json")
		if err != nil {
			t.Fatalf("extension list json error = %v", err)
		}
		var items []ExtensionRecord
		if err := json.Unmarshal([]byte(stdout), &items); err != nil {
			t.Fatalf("json.Unmarshal(list) error = %v", err)
		}
		if len(items) != 1 || items[0].Name != "list-ext" || items[0].Type != "subprocess" {
			t.Fatalf("list json = %#v, want one subprocess extension", items)
		}
	})

	t.Run("Should toon", func(t *testing.T) {
		stdout, _, err := executeRootCommand(t, deps, "extension", "list", "-o", "toon")
		if err != nil {
			t.Fatalf("extension list toon error = %v", err)
		}
		if !strings.Contains(stdout, "extensions[1]{name,version,type,state,source,missing_env,capabilities}:") {
			t.Fatalf("toon output = %q, want extensions TOON table", stdout)
		}
	})
}

func TestExtensionEnableDisableOffline(t *testing.T) {
	t.Parallel()

	deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "toggle-ext", extensionFixtureOptions{})
	installExtensionFixture(t, homePaths, dir)

	registry, cleanup := openExtensionRegistry(t, homePaths)
	if err := registry.Disable("toggle-ext"); err != nil {
		t.Fatalf("registry.Disable() error = %v", err)
	}
	cleanup()

	if _, _, err := executeRootCommand(t, deps, "extension", "enable", "toggle-ext", "-o", "json"); err == nil ||
		!strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension enable offline error = %v, want running daemon requirement", err)
	}

	if _, _, err := executeRootCommand(t, deps, "extension", "disable", "toggle-ext", "-o", "json"); err == nil ||
		!strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension disable offline error = %v, want running daemon requirement", err)
	}
}

func TestExtensionEnableUnknownReturnsNotFound(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})

	_, _, err := executeRootCommand(t, deps, "extension", "enable", "missing-ext", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension enable unknown offline error = %v, want running daemon requirement", err)
	}
}

func TestExtensionStatusOnlineUsesDaemonClient(t *testing.T) {
	t.Parallel()

	expected := ExtensionRecord{
		Name:          "runtime-ext",
		Version:       "1.2.3",
		Type:          "subprocess",
		Source:        "user",
		Enabled:       true,
		State:         "active",
		Capabilities:  []string{"memory.backend"},
		Actions:       []string{"memory/store"},
		PID:           4242,
		UptimeSeconds: 120,
		Health:        "healthy",
		DaemonRunning: true,
	}
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		extensionStatusFn: func(_ context.Context, name string) (ExtensionRecord, error) {
			if name != "runtime-ext" {
				t.Fatalf("ExtensionStatus() name = %q, want %q", name, "runtime-ext")
			}
			return expected, nil
		},
	})
	deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
		return aghdaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }

	stdout, _, err := executeRootCommand(t, deps, "extension", "status", "runtime-ext", "-o", "json")
	if err != nil {
		t.Fatalf("extension status error = %v", err)
	}

	var item ExtensionRecord
	if err := json.Unmarshal([]byte(stdout), &item); err != nil {
		t.Fatalf("json.Unmarshal(status) error = %v", err)
	}
	if !reflect.DeepEqual(item, expected) {
		t.Fatalf("status payload = %#v, want %#v", item, expected)
	}
}

func TestExtensionStatusOfflineUsesRegistryState(t *testing.T) {
	t.Parallel()

	deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "offline-ext", extensionFixtureOptions{
		capabilities: []string{"memory.backend"},
	})
	installExtensionFixture(t, homePaths, dir)

	_, _, err := executeRootCommand(t, deps, "extension", "status", "offline-ext", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension status offline error = %v, want running daemon requirement", err)
	}
}

func TestExtensionStatusOfflineReportsMissingEnvWithoutLeakingValues(t *testing.T) {
	t.Parallel()

	deps, homePaths := newExtensionLocalDeps(t, &stubClient{})
	deps.getenv = func(key string) string {
		if key == "PRESENT_TOKEN" {
			return "super-secret-present-value"
		}
		return ""
	}
	dir := writeExtensionFixture(t, "env-ext", extensionFixtureOptions{
		requiresEnv: []string{"PRESENT_TOKEN", "MISSING_TOKEN"},
	})
	installExtensionFixture(t, homePaths, dir)

	deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
		return aghdaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }
	statusClient := &stubClient{
		extensionStatusFn: func(_ context.Context, name string) (ExtensionRecord, error) {
			if name != "env-ext" {
				t.Fatalf("ExtensionStatus() name = %q, want env-ext", name)
			}
			return localExtensionRecord(*getInstalledExtension(t, homePaths, "env-ext"), deps.now, deps.getenv), nil
		},
	}
	deps.newClient = func(string) (DaemonClient, error) {
		return statusClient, nil
	}

	stdout, _, err := executeRootCommand(t, deps, "extension", "status", "env-ext", "-o", "json")
	if err != nil {
		t.Fatalf("extension status env-ext error = %v", err)
	}
	if strings.Contains(stdout, "super-secret-present-value") {
		t.Fatalf("extension status leaked env value:\n%s", stdout)
	}

	var item ExtensionRecord
	if err := json.Unmarshal([]byte(stdout), &item); err != nil {
		t.Fatalf("json.Unmarshal(status env-ext) error = %v", err)
	}
	if !reflect.DeepEqual(item.RequiresEnv, []string{"PRESENT_TOKEN", "MISSING_TOKEN"}) {
		t.Fatalf("RequiresEnv = %#v, want present+missing", item.RequiresEnv)
	}
	if !reflect.DeepEqual(item.MissingEnv, []string{"MISSING_TOKEN"}) {
		t.Fatalf("MissingEnv = %#v, want MISSING_TOKEN", item.MissingEnv)
	}

	human, _, err := executeRootCommand(t, deps, "extension", "status", "env-ext")
	if err != nil {
		t.Fatalf("extension status human env-ext error = %v", err)
	}
	if !strings.Contains(human, "Missing Env") || !strings.Contains(human, "MISSING_TOKEN") {
		t.Fatalf("extension status human = %q, want missing env diagnostic", human)
	}
	if strings.Contains(human, "super-secret-present-value") {
		t.Fatalf("extension status human leaked env value:\n%s", human)
	}
}

func TestExtensionInstallUsesDaemonClientWhenRunning(t *testing.T) {
	t.Parallel()

	dir := writeExtensionFixture(t, "online-install-ext", extensionFixtureOptions{})
	var captured InstallExtensionRequest
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		installExtensionFn: func(_ context.Context, request InstallExtensionRequest) (ExtensionRecord, error) {
			captured = request
			return ExtensionRecord{
				Name:          "online-install-ext",
				Version:       "0.1.0",
				Type:          "resource",
				Source:        "user",
				Enabled:       true,
				State:         "active",
				DaemonRunning: true,
			}, nil
		},
	})
	deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
		return aghdaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }

	if _, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		dir,
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	); err != nil {
		t.Fatalf("extension install online error = %v", err)
	}
	if captured.Path == "" || captured.Checksum == "" || !captured.AllowUnverified {
		t.Fatalf("captured install request = %#v, want path, checksum, and allow_unverified", captured)
	}
}

func TestExtensionBundleAndHelpers(t *testing.T) {
	t.Parallel()

	item := ExtensionRecord{
		Name:          "bundle-ext",
		Version:       "0.1.0",
		Type:          "resource",
		Source:        "user",
		Enabled:       true,
		State:         "active",
		Capabilities:  []string{"observe.exporter"},
		Actions:       []string{"observe/health"},
		PID:           321,
		UptimeSeconds: 3660,
		Health:        "healthy",
		HealthMessage: "steady",
		LastError:     "",
		DaemonRunning: true,
	}

	bundle := extensionBundle(item)
	human, err := bundle.human()
	if err != nil {
		t.Fatalf("bundle.human() error = %v", err)
	}
	if !strings.Contains(human, "Daemon") || !strings.Contains(human, "running") || !strings.Contains(human, "1h 1m") {
		t.Fatalf("human output = %q, want daemon/uptime content", human)
	}

	toon, err := bundle.toon()
	if err != nil {
		t.Fatalf("bundle.toon() error = %v", err)
	}
	if !strings.Contains(
		toon,
		"extension{name,version,type,source,enabled,state,daemon_running,"+
			"pid,uptime_seconds,health,last_error,capabilities,actions,requires_env,missing_env}:",
	) {
		t.Fatalf("toon output = %q, want extension TOON object", toon)
	}

	if got := formatExtensionUptime(59); got != "59s" {
		t.Fatalf("formatExtensionUptime(59) = %q, want %q", got, "59s")
	}
	if got := formatExtensionUptime(0); got != "" {
		t.Fatalf("formatExtensionUptime(0) = %q, want empty string", got)
	}
	if got := formatExtensionUptime(3600); got != "1h" {
		t.Fatalf("formatExtensionUptime(3600) = %q, want %q", got, "1h")
	}
	if got := joinExtensionHealth("healthy", "steady"); got != "healthy (steady)" {
		t.Fatalf("joinExtensionHealth() = %q, want %q", got, "healthy (steady)")
	}
	if got := joinExtensionHealth("healthy", ""); got != "healthy" {
		t.Fatalf("joinExtensionHealth(no message) = %q, want %q", got, "healthy")
	}
	if got := joinExtensionHealth("", "steady"); got != "" {
		t.Fatalf("joinExtensionHealth(no health) = %q, want empty string", got)
	}
	if got := boolLabel(false, "running", "offline"); got != "offline" {
		t.Fatalf("boolLabel(false) = %q, want %q", got, "offline")
	}
}

func newExtensionLocalDeps(t *testing.T, client DaemonClient) (commandDeps, aghconfig.HomePaths) {
	t.Helper()

	deps := newTestDeps(t, client)
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if err := cliTestStoreSeed.Clone(homePaths.DatabaseFile); err != nil {
		t.Fatalf("cli store seed Clone() error = %v", err)
	}
	deps.ensureHome = aghconfig.EnsureHomeLayout
	deps.loadConfig = func() (aghconfig.Config, error) {
		cfg := aghconfig.DefaultWithHome(homePaths)
		cfg.Extensions.Marketplace.AllowUnverified = true
		return cfg, nil
	}
	return deps, homePaths
}

func writeExtensionFixture(t *testing.T, name string, opts extensionFixtureOptions) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
	}
	writeExtensionManifest(t, filepath.Join(dir, "extension.toml"), extensionFixtureManifest(name, opts))
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("fixture"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(README.md) error = %v", err)
	}
	return dir
}

func extensionFixtureManifest(name string, opts extensionFixtureOptions) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, `[extension]
name = %q
version = "0.1.0"
description = "CLI extension test fixture"
min_agh_version = "0.5.0"
`, name)
	if len(opts.requiresEnv) > 0 {
		fmt.Fprintf(&builder, `requires_env = [%s]
`, quotedTOMLValues(opts.requiresEnv))
	}
	builder.WriteString(`
[resources]
`)

	if len(opts.capabilities) > 0 {
		fmt.Fprintf(&builder, `
[capabilities]
provides = [%s]
`, quotedTOMLValues(opts.capabilities))
	}
	if len(opts.actions) > 0 {
		fmt.Fprintf(&builder, `
[actions]
requires = [%s]
`, quotedTOMLValues(opts.actions))
	}
	return builder.String()
}

func quotedTOMLValues(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, fmt.Sprintf("%q", value))
	}
	return strings.Join(quoted, ", ")
}

func writeExtensionManifest(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func installExtensionFixture(t *testing.T, homePaths aghconfig.HomePaths, dir string) {
	t.Helper()

	registry, cleanup := openExtensionRegistry(t, homePaths)
	defer cleanup()

	manifest, err := extensionpkg.LoadManifest(dir)
	if err != nil {
		t.Fatalf("LoadManifest(%q) error = %v", dir, err)
	}
	checksum, err := extensionpkg.ComputeDirectoryChecksum(dir)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksum(%q) error = %v", dir, err)
	}
	if err := registry.Install(manifest, dir, checksum); err != nil {
		t.Fatalf("registry.Install(%q) error = %v", dir, err)
	}
}

func openExtensionRegistry(t *testing.T, homePaths aghconfig.HomePaths) (*extensionpkg.Registry, func()) {
	t.Helper()

	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	db, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	return extensionpkg.NewRegistry(db.DB()), func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	}
}

func getInstalledExtension(t *testing.T, homePaths aghconfig.HomePaths, name string) *extensionpkg.ExtensionInfo {
	t.Helper()

	registry, cleanup := openExtensionRegistry(t, homePaths)
	defer cleanup()

	info, err := registry.Get(name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", name, err)
	}
	return info
}
