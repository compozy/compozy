package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
)

type extensionFixtureOptions struct {
	capabilities []string
	permissions  []string
	requiresEnv  []string
}

type extensionSecretTestWriter struct {
	writes    int
	failAfter int
	err       error
}

func TestParseExtensionRemoteHeader(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		input      string
		wantServer string
		wantHeader string
		wantError  string
	}{
		{name: "Should leave an omitted mapping empty", input: ""},
		{
			name: "Should parse and trim one mapping", input: " deployment-api : X-Deployment-Key ",
			wantServer: "deployment-api", wantHeader: "X-Deployment-Key",
		},
		{
			name: "Should allow operator-owned authorization without OAuth", input: "deployment-api:Authorization",
			wantServer: "deployment-api", wantHeader: "Authorization",
		},
		{name: "Should reject a missing separator", input: "deployment-api", wantError: "<server>:<header>"},
		{name: "Should reject an empty server", input: ":X-Key", wantError: "server is required"},
		{name: "Should reject an empty header", input: "deployment-api:", wantError: "header is required"},
		{name: "Should reject an invalid server name", input: "bad/server:X-Key", wantError: "--remote-header server"},
		{
			name: "Should reject an invalid header name", input: "deployment-api:Bad Header",
			wantError: "--remote-header header",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			server, header, err := parseExtensionRemoteHeader(testCase.input)
			if testCase.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
					t.Fatalf(
						"parseExtensionRemoteHeader(%q) error = %v, want %q",
						testCase.input,
						err,
						testCase.wantError,
					)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseExtensionRemoteHeader(%q) error = %v", testCase.input, err)
			}
			if server != testCase.wantServer || header != testCase.wantHeader {
				t.Fatalf(
					"parseExtensionRemoteHeader(%q) = %q/%q, want %q/%q",
					testCase.input,
					server,
					header,
					testCase.wantServer,
					testCase.wantHeader,
				)
			}
		})
	}
}

// Invariant: every embedded extension scaffold is discoverable from `extension init --help`.
// Owning layer: CLI extension authoring command.
// Canonical suite: no existing extension-init help suite owns this product contract.
func TestExtensionInitHelpListsEveryScaffoldTemplate(t *testing.T) {
	t.Parallel()

	stdout, _, err := executeRootCommand(t, commandDeps{}, "extension", "init", "--help")
	if err != nil {
		t.Fatalf("executeRootCommand(extension init --help) error = %v", err)
	}
	for _, template := range extensionpkg.ScaffoldTemplates() {
		if !strings.Contains(stdout, string(template)) {
			t.Fatalf("extension init --help omitted template %q", template)
		}
	}
}

func TestExtensionValidatePortableExitContract(t *testing.T) {
	t.Parallel()

	t.Run("Should keep warn-only portable validation successful and daemonless", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join("..", "extension", "testdata", "agent-plugin-conformant")
		stdout, _, err := executeRootCommand(t, commandDeps{}, "extension", "validate", path)
		if err != nil {
			t.Fatalf("executeRootCommand(extension validate portable) error = %v", err)
		}
		for _, want := range []string{"Status:", "valid", "Format:", "agent plugin", "WARN"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("portable validate output = %q, want %q", stdout, want)
			}
		}
	})

	t.Run("Should print a fatal portable report before returning exit failure", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		manifest := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"Bad--Name"}`
		if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(manifest), 0o600); err != nil {
			t.Fatalf("os.WriteFile(plugin.json) error = %v", err)
		}
		stdout, _, err := executeRootCommand(t, commandDeps{}, "extension", "validate", root)
		if err == nil || !strings.Contains(err.Error(), "extension bundle validation failed") {
			t.Fatalf("executeRootCommand(extension validate fatal) error = %v, want validation failure", err)
		}
		for _, want := range []string{"Status:", "invalid", "Format:", "agent plugin", "ERROR plugin.json"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("fatal portable validate output = %q, want %q", stdout, want)
			}
		}
	})

	t.Run("Should preserve native human validation output", func(t *testing.T) {
		t.Parallel()

		path := writeExtensionFixture(t, "native-validation", extensionFixtureOptions{})
		stdout, _, err := executeRootCommand(t, commandDeps{}, "extension", "validate", path)
		if err != nil {
			t.Fatalf("executeRootCommand(extension validate native) error = %v", err)
		}
		if !strings.Contains(stdout, "Extension bundle validation") || !strings.Contains(stdout, "Consent:") ||
			strings.Contains(stdout, "Format:") || strings.Contains(stdout, "Would ingest") {
			t.Fatalf("native validate output changed shape: %q", stdout)
		}
	})
}

func TestExtensionLogsCursorRequiresItsStreamEpoch(t *testing.T) {
	t.Parallel()
	t.Run("Should reject a sequence cursor without its ring identity", func(t *testing.T) {
		t.Parallel()

		_, _, err := executeRootCommand(
			t,
			commandDeps{},
			"extension",
			"logs",
			"alpha",
			"--global",
			"--after",
			"12",
		)
		if err == nil || !strings.Contains(err.Error(), "--stream-epoch is required with --after") {
			t.Fatalf("extension logs cursor error = %v, want paired stream epoch refusal", err)
		}
	})
}

func TestExtensionLogsForwardsThePairedCursor(t *testing.T) {
	t.Parallel()
	t.Run("Should forward the paired cursor to the daemon client", func(t *testing.T) {
		t.Parallel()

		called := false
		client := &stubClient{
			extensionLogsFn: func(
				_ context.Context,
				workspaceRef string,
				name string,
				after int64,
				streamEpoch string,
			) (ExtensionLogsRecord, error) {
				called = true
				if workspaceRef != "" || name != "alpha" || after != 12 || streamEpoch != "epoch-alpha" {
					t.Fatalf(
						"ExtensionLogs() = workspace %q, name %q, after %d, epoch %q",
						workspaceRef,
						name,
						after,
						streamEpoch,
					)
				}
				return ExtensionLogsRecord{Logs: []ExtensionLogRecord{}, StreamEpoch: "epoch-alpha"}, nil
			},
		}
		deps, _ := newExtensionLocalDeps(t, client)
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(int) bool { return true }
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"logs",
			"alpha",
			"--global",
			"--after",
			"12",
			"--stream-epoch",
			"epoch-alpha",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension logs paired cursor error = %v", err)
		}
		if !called {
			t.Fatal("ExtensionLogs() was not called")
		}
		var output ExtensionLogsRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(extension logs output) error = %v", err)
		}
		if output.StreamEpoch != "epoch-alpha" {
			t.Fatalf("extension logs output epoch = %q, want epoch-alpha", output.StreamEpoch)
		}
	})
}

func TestExtensionLogsFollowEmitsAnEmptyResetRecord(t *testing.T) {
	t.Parallel()
	t.Run("Should emit an identified empty reset record while following", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			streamExtensionLogsFn: func(
				_ context.Context,
				workspaceRef string,
				name string,
				after int64,
				streamEpoch string,
				handler SSEHandler,
			) error {
				if workspaceRef != "" || name != "alpha" || after != 4 || streamEpoch != "epoch-old" {
					t.Fatalf(
						"StreamExtensionLogs() = workspace %q, name %q, after %d, epoch %q",
						workspaceRef,
						name,
						after,
						streamEpoch,
					)
				}
				return handler(SSEEvent{
					Event: "extension_log_reset",
					Data: mustJSON(t, ExtensionLogsRecord{
						Logs: []ExtensionLogRecord{}, StreamEpoch: "epoch-new",
					}),
				})
			},
		}
		deps, _ := newExtensionLocalDeps(t, client)
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(int) bool { return true }
		stdout, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"logs",
			"alpha",
			"--global",
			"--follow",
			"--after",
			"4",
			"--stream-epoch",
			"epoch-old",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("extension logs --follow error = %v", err)
		}
		var reset extensionLogResetRecord
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &reset); err != nil {
			t.Fatalf("json.Unmarshal(extension log reset) error = %v", err)
		}
		if reset.Event != "extension_log_reset" ||
			reset.StreamEpoch != "epoch-new" ||
			len(reset.Logs) != 0 {
			t.Fatalf("extension log reset = %#v, want identified empty reset", reset)
		}
	})
}

func (w *extensionSecretTestWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes > w.failAfter {
		return 0, w.err
	}
	return len(data), nil
}

func TestExtensionSecretReaderReadsHiddenInput(t *testing.T) {
	t.Parallel()

	t.Run("Should return a trimmed hidden value", func(t *testing.T) {
		t.Parallel()

		input := openExtensionSecretTestInput(t)
		var output bytes.Buffer
		reader := newExtensionSecretReader(input, &output, func(io.Reader) bool { return true })
		reader.readPassword = func(int) ([]byte, error) { return []byte(" secret-value\r\n"), nil }

		value, err := reader.Read("API_KEY")
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if value != " secret-value" || output.String() != "API_KEY: \n" {
			t.Fatalf("Read() value=%q output=%q, want trimmed hidden value and prompt", value, output.String())
		}
	})

	t.Run("Should return a password read error", func(t *testing.T) {
		t.Parallel()

		input := openExtensionSecretTestInput(t)
		readErr := errors.New("password read failed")
		reader := newExtensionSecretReader(input, io.Discard, func(io.Reader) bool { return true })
		reader.readPassword = func(int) ([]byte, error) { return nil, readErr }

		_, err := reader.Read("API_KEY")
		if !errors.Is(err, readErr) {
			t.Fatalf("Read() error = %v, want password read failure", err)
		}
	})

	t.Run("Should join password and newline write errors", func(t *testing.T) {
		t.Parallel()

		input := openExtensionSecretTestInput(t)
		readErr := errors.New("password read failed")
		newlineErr := errors.New("newline write failed")
		output := &extensionSecretTestWriter{failAfter: 1, err: newlineErr}
		reader := newExtensionSecretReader(input, output, func(io.Reader) bool { return true })
		reader.readPassword = func(int) ([]byte, error) { return nil, readErr }

		_, err := reader.Read("API_KEY")
		if !errors.Is(err, readErr) || !errors.Is(err, newlineErr) {
			t.Fatalf("Read() error = %v, want joined read and newline failures", err)
		}
	})
}

func openExtensionSecretTestInput(t *testing.T) *os.File {
	t.Helper()

	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("os.Open(os.DevNull) error = %v", err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close extension secret test input: %v", err)
		}
	})
	return file
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
		t.Fatal("installed extension enabled = false, want default-on install")
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
		deps.loadConfig = func() (compozyconfig.Config, error) {
			cfg := compozyconfig.DefaultWithHome(homePaths)
			cfg.Extensions.Trust.AllowUnverified = false
			return cfg, nil
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
min_compozy_version = "0.5.0"

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
			"Update",
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
		if !strings.Contains(
			stdout,
			"extensions[1]{name,version,update,type,state,source,missing_env,capabilities}:",
		) {
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

func TestExtensionScopedReadsResolveStableWorkspaceID(t *testing.T) {
	t.Parallel()

	t.Run("Should list the resolved workspace extension overlay", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if got, want := ref, "workspace-alias"; got != want {
					t.Fatalf("GetWorkspace() ref = %q, want %q", got, want)
				}
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
					ID: "workspace-stable", Name: "workspace-alias", RootDir: t.TempDir(),
				}}, nil
			},
			listExtensionsScopedFn: func(_ context.Context, workspaceRef string) ([]ExtensionRecord, error) {
				if got, want := workspaceRef, "workspace-stable"; got != want {
					t.Fatalf("ListExtensionsScoped() workspace = %q, want %q", got, want)
				}
				return []ExtensionRecord{{
					Name: "dev-extension", WorkspaceID: workspaceRef, Source: "workspace", Dev: true,
				}}, nil
			},
		}
		deps, _ := newExtensionLocalDeps(t, client)
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(int) bool { return true }

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"list",
			"--workspace",
			"workspace-alias",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension list --workspace error = %v", err)
		}
		var items []ExtensionRecord
		if err := json.Unmarshal([]byte(stdout), &items); err != nil {
			t.Fatalf("json.Unmarshal(scoped list) error = %v", err)
		}
		if len(items) != 1 || items[0].Name != "dev-extension" || !items[0].Dev {
			t.Fatalf("scoped list = %#v, want dev extension overlay", items)
		}
	})

	t.Run("Should read status from the resolved workspace extension overlay", func(t *testing.T) {
		t.Parallel()

		client := &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if got, want := ref, "workspace-alias"; got != want {
					t.Fatalf("GetWorkspace() ref = %q, want %q", got, want)
				}
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
					ID: "workspace-stable", Name: "workspace-alias", RootDir: t.TempDir(),
				}}, nil
			},
			extensionStatusScopedFn: func(
				_ context.Context,
				workspaceRef string,
				name string,
			) (ExtensionRecord, error) {
				if got, want := workspaceRef, "workspace-stable"; got != want {
					t.Fatalf("ExtensionStatusScoped() workspace = %q, want %q", got, want)
				}
				if got, want := name, "dev-extension"; got != want {
					t.Fatalf("ExtensionStatusScoped() name = %q, want %q", got, want)
				}
				return ExtensionRecord{
					Name: name, WorkspaceID: workspaceRef, Source: "workspace", Dev: true, State: "active",
				}, nil
			},
		}
		deps, _ := newExtensionLocalDeps(t, client)
		deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
		}
		deps.processAlive = func(int) bool { return true }

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"extension",
			"status",
			"dev-extension",
			"--workspace",
			"workspace-alias",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension status --workspace error = %v", err)
		}
		var item ExtensionRecord
		if err := json.Unmarshal([]byte(stdout), &item); err != nil {
			t.Fatalf("json.Unmarshal(scoped status) error = %v", err)
		}
		if item.Name != "dev-extension" || !item.Dev || item.State != "active" {
			t.Fatalf("scoped status = %#v, want active dev extension overlay", item)
		}
	})
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
		Permissions:   []string{"memory/store"},
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
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
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

	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
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
	deps.newClient = func(ClientTarget) (DaemonClient, error) {
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
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
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
	if captured.Source != contract.InstallExtensionSourceLocalPath ||
		captured.Ref != filepath.Clean(dir) ||
		!captured.AllowUnverified {
		t.Fatalf("captured install request = %#v, want local_path ref and allow_unverified", captured)
	}

	t.Run("Should print the declared-profile preview before installing [UT-058][E2E-008]", func(t *testing.T) {
		var previewed bool
		client := &stubClient{
			previewExtensionInstallFn: func(
				_ context.Context,
				request InstallExtensionRequest,
			) (ExtensionInstallPreviewRecord, error) {
				previewed = true
				return ExtensionInstallPreviewRecord{
					Name: "growth-kit",
					DeclaredProfiles: []contract.ExtensionInstallDeclaredProfilePayload{{
						Name: "growth", Create: true,
						Credentials: []contract.ProfileCredentialRequirement{{Provider: "openai", Slot: "api_key"}},
					}},
					Placements: []contract.ExtensionPlacementPayload{{
						Kind: "skill", Resource: "tweet-writer", Profile: "growth",
					}},
				}, nil
			},
			installExtensionFn: func(
				_ context.Context,
				_ InstallExtensionRequest,
			) (ExtensionRecord, error) {
				if !previewed {
					t.Fatal("InstallExtension called before PreviewExtensionInstall")
				}
				return ExtensionRecord{Name: "growth-kit", Enabled: true}, nil
			},
		}
		previewDeps, _ := newExtensionLocalDeps(t, client)
		previewDeps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
			return compozydaemon.Info{PID: 102, StartedAt: fixedTestNow}, nil
		}
		previewDeps.processAlive = func(int) bool { return true }

		_, stderr, err := executeRootCommand(
			t,
			previewDeps,
			"extension",
			"install",
			dir,
			"--allow-unverified",
			"--yes",
		)
		if err != nil {
			t.Fatalf("extension install human preview error = %v", err)
		}
		for _, want := range []string{
			"growth-kit will:",
			"create profile growth (needs openai api_key)",
			"add skill tweet-writer to profile growth",
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("extension install preview = %q, want %q", stderr, want)
			}
		}
	})
}

func TestExtensionDevBindsResolvedCurrentWorkspace(t *testing.T) {
	t.Parallel()

	sourceDir := filepath.Join(t.TempDir(), "cli-dev-extension")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(sourceDir) error = %v", err)
	}
	writeExtensionManifest(t, filepath.Join(sourceDir, "go.mod"), "module example.com/cli-dev-extension\n\ngo 1.26.4\n")
	writeExtensionManifest(t, filepath.Join(sourceDir, "main.go"), `package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "__describe" {
		fmt.Print(`+"`"+`{"name":"cli-dev","version":"0.1.0","description":"CLI dev fixture","provides":[],"permissions":[],"subprocess":{"command":"./bin"},"sdk":{"name":"go","version":"0.1.0","protocol_version":"1","min_compozy_version":"0.3.0-beta.1"}}`+"`"+`)
	}
}
`)

	var capturedWorkspace string
	var capturedRequest DevLinkExtensionRequest
	client := &stubClient{
		getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			if ref != "workspace-alias" {
				t.Fatalf("GetWorkspace() ref = %q, want workspace-alias", ref)
			}
			return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
				ID:      "workspace-stable",
				Name:    "workspace-alias",
				RootDir: filepath.Dir(sourceDir),
			}}, nil
		},
		devExtensionFn: func(
			_ context.Context,
			workspaceRef string,
			request DevLinkExtensionRequest,
		) (ExtensionRecord, error) {
			capturedWorkspace = workspaceRef
			capturedRequest = request
			return ExtensionRecord{
				Name:          "cli-dev",
				WorkspaceID:   workspaceRef,
				Version:       "0.1.0",
				Dev:           true,
				DaemonRunning: true,
			}, nil
		},
	}
	deps, _ := newExtensionLocalDeps(t, client)
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"dev",
		sourceDir,
		"--workspace",
		"workspace-alias",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension dev error = %v", err)
	}
	if capturedWorkspace != "workspace-stable" {
		t.Fatalf("DevExtension() workspace = %q, want stable resolved ID", capturedWorkspace)
	}
	wantOrigin, err := filepath.Abs(sourceDir)
	if err != nil {
		t.Fatalf("filepath.Abs(sourceDir) error = %v", err)
	}
	if capturedRequest.OriginPath != wantOrigin || len(capturedRequest.GenerationHash) != 64 {
		t.Fatalf("DevExtension() request = %#v, want canonical origin and content hash", capturedRequest)
	}
	var output ExtensionRecord
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("json.Unmarshal(extension dev) error = %v", err)
	}
	if output.WorkspaceID != "workspace-stable" || !output.Dev {
		t.Fatalf("extension dev output = %#v", output)
	}
}

func TestExtensionDevWatchReloadsResourceOnlyChanges(t *testing.T) {
	t.Parallel()

	t.Run("Should rebuild and reload a changed resource-only source", func(t *testing.T) {
		t.Parallel()

		synctest.Test(t, func(t *testing.T) {
			sourceDir := filepath.Join(t.TempDir(), "resource-watch")
			skillDir := filepath.Join(sourceDir, "skills", "writer")
			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				t.Fatalf("os.MkdirAll(skillDir) error = %v", err)
			}
			writeExtensionManifest(t, filepath.Join(sourceDir, "extension.toml"), `[extension]
name = "resource-watch"
version = "0.1.0"
description = "Resource-only watch fixture"
min_compozy_version = "0.5.0"

[[resources.skills]]
path = "skills"
`)
			skillPath := filepath.Join(skillDir, "SKILL.md")
			writeExtensionManifest(t, skillPath, `---
name: writer
description: Write clear release notes.
---

# Writer
`)

			initialHash := make(chan string, 1)
			reloadHash := make(chan string, 1)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			client := &stubClient{
				getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
					if ref != "workspace-alias" {
						return WorkspaceDetailRecord{}, fmt.Errorf(
							"GetWorkspace() ref = %q, want workspace-alias",
							ref,
						)
					}
					return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
						ID: "workspace-stable", Name: ref, RootDir: filepath.Dir(sourceDir),
					}}, nil
				},
				devExtensionFn: func(
					_ context.Context,
					workspaceRef string,
					request DevLinkExtensionRequest,
				) (ExtensionRecord, error) {
					initialHash <- request.GenerationHash
					return ExtensionRecord{
						Name: "resource-watch", WorkspaceID: workspaceRef, Dev: true, DaemonRunning: true,
					}, nil
				},
				reloadDevExtensionFn: func(
					_ context.Context,
					workspaceRef string,
					name string,
					request ReloadExtensionRequest,
				) (ExtensionRecord, error) {
					if workspaceRef != "workspace-stable" || name != "resource-watch" {
						return ExtensionRecord{}, fmt.Errorf(
							"ReloadDevExtension() target = %q/%q, want workspace-stable/resource-watch",
							workspaceRef,
							name,
						)
					}
					reloadHash <- request.GenerationHash
					cancel()
					return ExtensionRecord{
						Name: name, WorkspaceID: workspaceRef, Dev: true, DaemonRunning: true,
					}, nil
				},
			}
			deps, homePaths := newExtensionLocalDeps(t, client)
			deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
				return compozydaemon.Info{PID: 101, StartedAt: fixedTestNow}, nil
			}
			deps.processAlive = func(int) bool { return true }
			deps.loadConfig = func() (compozyconfig.Config, error) {
				cfg := compozyconfig.DefaultWithHome(homePaths)
				cfg.Extensions.Dev.WatchInterval = time.Second
				return cfg, nil
			}

			result := make(chan error, 1)
			go func() {
				command := newRootCommand(deps)
				command.SetOut(io.Discard)
				command.SetErr(io.Discard)
				command.SetArgs([]string{
					"extension", "dev", sourceDir, "--watch", "--workspace", "workspace-alias", "-o", "json",
				})
				result <- command.ExecuteContext(ctx)
			}()

			firstGeneration := <-initialHash
			synctest.Wait()
			writeExtensionManifest(t, skillPath, `---
name: writer
description: Write concise release notes.
---

# Writer
`)
			time.Sleep(time.Second)
			synctest.Wait()

			secondGeneration := <-reloadHash
			if secondGeneration == firstGeneration {
				t.Fatalf("reload generation = %q, want a new hash after the skill changed", secondGeneration)
			}
			if err := <-result; !errors.Is(err, context.Canceled) {
				t.Fatalf("extension dev --watch error = %v, want context cancellation", err)
			}
		})
	})
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
		Permissions:   []string{"observe/health"},
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
		"extension{name,version,type,format,source,enabled,state,daemon_running,"+
			"pid,uptime_seconds,health,last_error,capabilities,permissions,requires_env,missing_env,"+
			"consecutive_failures,restart_backoff_ms,summary,diagnostics}:",
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

func newExtensionLocalDeps(t *testing.T, client DaemonClient) (commandDeps, compozyconfig.HomePaths) {
	t.Helper()

	deps := newTestDeps(t, client)
	homePaths, err := deps.resolveHome()
	if err != nil {
		t.Fatalf("resolveHome() error = %v", err)
	}
	if err := cliTestStoreSeed.Clone(homePaths.DatabaseFile); err != nil {
		t.Fatalf("cli store seed Clone() error = %v", err)
	}
	deps.ensureHome = compozyconfig.EnsureHomeLayout
	deps.loadConfig = func() (compozyconfig.Config, error) {
		cfg := compozyconfig.DefaultWithHome(homePaths)
		cfg.Extensions.Trust.AllowUnverified = true
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
min_compozy_version = "0.5.0"
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
	if len(opts.permissions) > 0 {
		fmt.Fprintf(&builder, `
[permissions]
requires = [%s]
`, quotedTOMLValues(opts.permissions))
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

func installExtensionFixture(t *testing.T, homePaths compozyconfig.HomePaths, dir string) {
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

func openExtensionRegistry(t *testing.T, homePaths compozyconfig.HomePaths) (*extensionpkg.Registry, func()) {
	t.Helper()

	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
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

func getInstalledExtension(t *testing.T, homePaths compozyconfig.HomePaths, name string) *extensionpkg.ExtensionInfo {
	t.Helper()

	registry, cleanup := openExtensionRegistry(t, homePaths)
	defer cleanup()

	info, err := registry.Get(name)
	if err != nil {
		t.Fatalf("registry.Get(%q) error = %v", name, err)
	}
	return info
}
