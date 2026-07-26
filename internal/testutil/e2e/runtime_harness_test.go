package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestPrepareRuntimeLayoutCreatesIsolatedPaths(t *testing.T) {
	t.Parallel()

	first := prepareRuntimeLayout(t, &RuntimeHarnessOptions{})
	second := prepareRuntimeLayout(t, &RuntimeHarnessOptions{})

	if first.HomePaths.HomeDir == second.HomePaths.HomeDir {
		t.Fatalf("first.HomePaths.HomeDir = %q, want different isolated home", first.HomePaths.HomeDir)
	}
	if first.HomePaths.DatabaseFile == second.HomePaths.DatabaseFile {
		t.Fatalf(
			"first.HomePaths.DatabaseFile = %q, want different isolated database path",
			first.HomePaths.DatabaseFile,
		)
	}
	if first.WorkspaceRoot == second.WorkspaceRoot {
		t.Fatalf("first.WorkspaceRoot = %q, want different isolated workspace path", first.WorkspaceRoot)
	}
	if first.Artifacts.RootDir() == second.Artifacts.RootDir() {
		t.Fatalf("first.Artifacts.RootDir() = %q, want different isolated artifact path", first.Artifacts.RootDir())
	}
}

func TestPrepareRuntimeLayoutUsesEnabledNetworkByDefaultAndAllowsExplicitDisable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts RuntimeHarnessOptions
		want bool
	}{
		{
			name: "ShouldEnableNetworkByDefault",
			opts: RuntimeHarnessOptions{},
			want: true,
		},
		{
			name: "ShouldAllowExplicitDisableFromConfigSeed",
			opts: RuntimeHarnessOptions{
				ConfigSeed: ConfigSeedOptions{
					Mutate: func(cfg *aghconfig.Config) {
						cfg.Network.Enabled = false
					},
				},
			},
			want: false,
		},
		{
			name: "ShouldOverrideDisabledSeedWhenEnableNetworkIsRequested",
			opts: RuntimeHarnessOptions{
				EnableNetwork: true,
				ConfigSeed: ConfigSeedOptions{
					Mutate: func(cfg *aghconfig.Config) {
						cfg.Network.Enabled = false
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			layout := prepareRuntimeLayout(t, &tt.opts)
			if got := layout.Config.Network.Enabled; got != tt.want {
				t.Fatalf("layout.Config.Network.Enabled = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestPrepareRuntimeLayoutOverridesCallerHomeState(t *testing.T) {
	t.Setenv("HOME", "/tmp/caller-home")
	t.Setenv("AGH_HOME", "/tmp/caller-agh-home")

	layout := prepareRuntimeLayout(t, &RuntimeHarnessOptions{})

	if got, want := lookupEnvValue(layout.Env, "HOME"), layout.HomePaths.HomeDir; got != want {
		t.Fatalf("lookupEnvValue(HOME) = %q, want %q", got, want)
	}
	if got, want := lookupEnvValue(layout.Env, "AGH_HOME"), layout.HomePaths.HomeDir; got != want {
		t.Fatalf("lookupEnvValue(AGH_HOME) = %q, want %q", got, want)
	}
	if got, want := countEnvEntries(layout.Env, "HOME"), 1; got != want {
		t.Fatalf("countEnvEntries(HOME) = %d, want %d", got, want)
	}
	if got, want := countEnvEntries(layout.Env, "AGH_HOME"), 1; got != want {
		t.Fatalf("countEnvEntries(AGH_HOME) = %d, want %d", got, want)
	}
}

func TestRuntimeHarnessHTTPReadinessProbeRequirement(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		host string
		want bool
	}{
		{name: "ShouldProbeLoopbackIPv4HTTP", host: "127.0.0.1", want: true},
		{name: "ShouldProbeLoopbackIPv6HTTP", host: "::1", want: true},
		{name: "ShouldProbeLocalhostHTTP", host: "localhost", want: true},
		{name: "ShouldProbeDefaultEmptyHostHTTP", host: "", want: true},
		{name: "ShouldSkipWildcardIPv4HTTP", host: "0.0.0.0", want: false},
		{name: "ShouldSkipWildcardIPv6HTTP", host: "::", want: false},
		{name: "ShouldSkipNonLoopbackHTTP", host: "192.0.2.10", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := runtimeHarnessRequiresHTTPReadinessProbe(tt.host); got != tt.want {
				t.Fatalf("runtimeHarnessRequiresHTTPReadinessProbe(%q) = %t, want %t", tt.host, got, tt.want)
			}
		})
	}
}

func TestRuntimeHarnessWriteRuntimeManifestIncludesPathsAndTransportMetadata(t *testing.T) {
	t.Parallel()

	homePaths := NewHomePaths(t)
	config := SeedConfig(t, homePaths, ConfigSeedOptions{})
	workspaceRoot := SeedWorkspace(t, WorkspaceSeedOptions{
		Files: map[string]string{"README.md": "runtime manifest"},
	})
	collector := NewArtifactCollector(t)
	processLogPath := filepath.Join(collector.RootDir(), "daemon-process.log")
	if err := os.WriteFile(processLogPath, []byte("daemon booted\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", processLogPath, err)
	}
	sessionDir := filepath.Join(homePaths.SessionsDir, "sess-1")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", sessionDir, err)
	}

	harness := &RuntimeHarness{
		HomePaths:     homePaths,
		Config:        config,
		Artifacts:     collector,
		WorkspaceRoot: workspaceRoot,
		HTTPBaseURL:   "http://127.0.0.1:4317",
		UDSBaseURL:    "http://unix",
		CLI: &CLIClient{
			binaryPath: "/tmp/agh-test-bin",
			workdir:    "/repo",
		},
		processLogPath: processLogPath,
	}

	manifest, err := harness.WriteRuntimeManifest()
	if err != nil {
		t.Fatalf("WriteRuntimeManifest() error = %v", err)
	}

	if got, want := manifest.Home.DatabaseFile, homePaths.DatabaseFile; got != want {
		t.Fatalf("manifest.Home.DatabaseFile = %q, want %q", got, want)
	}
	if got, want := manifest.Logs.ProcessLogFile, processLogPath; got != want {
		t.Fatalf("manifest.Logs.ProcessLogFile = %q, want %q", got, want)
	}
	if got, want := manifest.Runs.RootDir, homePaths.SessionsDir; got != want {
		t.Fatalf("manifest.Runs.RootDir = %q, want %q", got, want)
	}
	if got, want := len(manifest.Runs.Directories), 1; got != want {
		t.Fatalf("len(manifest.Runs.Directories) = %d, want %d", got, want)
	}
	if got, want := manifest.Transport.HTTPBaseURL, "http://127.0.0.1:4317"; got != want {
		t.Fatalf("manifest.Transport.HTTPBaseURL = %q, want %q", got, want)
	}
	if got, want := manifest.Transport.HTTPPort, config.HTTP.Port; got != want {
		t.Fatalf("manifest.Transport.HTTPPort = %d, want %d", got, want)
	}
	if got, want := manifest.Transport.SocketPath, config.Daemon.Socket; got != want {
		t.Fatalf("manifest.Transport.SocketPath = %q, want %q", got, want)
	}
	if got, want := manifest.Transport.CLIBinary, "/tmp/agh-test-bin"; got != want {
		t.Fatalf("manifest.Transport.CLIBinary = %q, want %q", got, want)
	}
	if got, want := manifest.ArtifactManifestPath, collector.ManifestPath(); got != want {
		t.Fatalf("manifest.ArtifactManifestPath = %q, want %q", got, want)
	}
	if _, err := os.Stat(harness.RuntimeManifestPath()); err != nil {
		t.Fatalf("os.Stat(%q) error = %v", harness.RuntimeManifestPath(), err)
	}
}

func TestInstallRuntimeCLIInstallsShimIntoRuntimeHome(t *testing.T) {
	t.Parallel()

	homePaths := NewHomePaths(t)
	sourceBinary := filepath.Join(t.TempDir(), "agh-source")
	payload := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(sourceBinary, payload, 0o755); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", sourceBinary, err)
	}

	shimPath, err := installRuntimeCLI(homePaths, sourceBinary)
	if err != nil {
		t.Fatalf("installRuntimeCLI() error = %v", err)
	}

	expectedName := "agh"
	if windowsGOOS == "windows" && filepath.Ext(shimPath) == ".exe" {
		expectedName = "agh.exe"
	}
	if got, want := filepath.Base(shimPath), expectedName; got != want {
		t.Fatalf("filepath.Base(shimPath) = %q, want %q", got, want)
	}
	if got, want := filepath.Dir(shimPath), filepath.Join(homePaths.HomeDir, "bin"); got != want {
		t.Fatalf("filepath.Dir(shimPath) = %q, want %q", got, want)
	}

	gotPayload, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", shimPath, err)
	}
	if !bytes.Equal(gotPayload, payload) {
		t.Fatalf("shim payload = %q, want %q", gotPayload, payload)
	}
}

func TestRuntimeRunDirectoriesReturnsSortedDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, dir := range []string{"run-b", "run-a"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("skip"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(README.txt) error = %v", err)
	}

	directories, err := runtimeRunDirectories("  " + root + "  ")
	if err != nil {
		t.Fatalf("runtimeRunDirectories(root) error = %v", err)
	}
	want := []string{
		filepath.Join(root, "run-a"),
		filepath.Join(root, "run-b"),
	}
	if len(directories) != len(want) {
		t.Fatalf("len(directories) = %d, want %d", len(directories), len(want))
	}
	for idx := range want {
		if got := directories[idx]; got != want[idx] {
			t.Fatalf("directories[%d] = %q, want %q", idx, got, want[idx])
		}
	}

	missing, err := runtimeRunDirectories(filepath.Join(root, "missing"))
	if err != nil {
		t.Fatalf("runtimeRunDirectories(missing) error = %v", err)
	}
	if missing != nil {
		t.Fatalf("runtimeRunDirectories(missing) = %v, want nil", missing)
	}

	blank, err := runtimeRunDirectories("   ")
	if err != nil {
		t.Fatalf("runtimeRunDirectories(blank) error = %v", err)
	}
	if blank != nil {
		t.Fatalf("runtimeRunDirectories(blank) = %v, want nil", blank)
	}
}

func countEnvEntries(env []string, key string) int {
	count := 0
	for _, entry := range env {
		if len(entry) > len(key) && entry[:len(key)+1] == key+"=" {
			count++
		}
	}
	return count
}
