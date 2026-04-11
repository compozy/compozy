package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/spf13/cobra"
)

type listRow struct {
	Name         string
	Version      string
	Source       string
	Enabled      string
	Active       string
	Capabilities string
}

func TestExtHelpShowsAllManagementSubcommands(t *testing.T) {
	deps := newTestDeps(t)
	output, err := executeExtCommand(t, deps, "--help")
	if err != nil {
		t.Fatalf("execute ext help: %v\noutput:\n%s", err, output)
	}

	for _, snippet := range []string{
		"list",
		"inspect",
		"install",
		"uninstall",
		"enable",
		"disable",
		"doctor",
	} {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected ext help to include %q\noutput:\n%s", snippet, output)
		}
	}
}

func TestNewExtCommandUsesHelpByDefault(t *testing.T) {
	cmd := NewExtCommand(nil)

	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute ext root command: %v\noutput:\n%s", err, output.String())
	}
	if !strings.Contains(output.String(), "Inspect and manage bundled, user, and workspace Compozy extensions.") {
		t.Fatalf("expected ext help output\noutput:\n%s", output.String())
	}
}

func TestListWithNoExtensionsPrintsHeaderOnly(t *testing.T) {
	deps := newTestDeps(t)
	output, err := executeExtCommand(t, deps, "list")
	if err != nil {
		t.Fatalf("execute ext list: %v\noutput:\n%s", err, output)
	}

	rows := parseListRows(t, output)
	if len(rows) != 0 {
		t.Fatalf("expected no extension rows, got %d\noutput:\n%s", len(rows), output)
	}
	if !strings.Contains(output, "NAME") || !strings.Contains(output, "CAPABILITIES") {
		t.Fatalf("expected list header in output:\n%s", output)
	}
}

func TestListWithThreeSourcesPrintsRowsAndEnabledState(t *testing.T) {
	deps := newTestDeps(t)
	writeManifestJSON(t, bundledExtensionDir(deps.bundledRoot, "bundled-ext"), manifestFixture("bundled-ext"))
	writeManifestJSON(t, userExtensionDir(deps.homeDir, "user-ext"), manifestFixture("user-ext"))
	writeManifestJSON(t, workspaceExtensionDir(deps.workspaceRoot, "workspace-ext"), manifestFixture("workspace-ext"))
	enableUserExtension(t, deps.homeDir, "user-ext")

	output, err := executeExtCommand(t, deps, "list")
	if err != nil {
		t.Fatalf("execute ext list: %v\noutput:\n%s", err, output)
	}

	rows := parseListRows(t, output)
	if len(rows) != 3 {
		t.Fatalf("expected 3 extension rows, got %d\noutput:\n%s", len(rows), output)
	}

	bundled := findListRow(t, rows, "bundled-ext", "bundled")
	if bundled.Enabled != "true" || bundled.Active != "true" {
		t.Fatalf("unexpected bundled row: %#v", bundled)
	}

	user := findListRow(t, rows, "user-ext", "user")
	if user.Enabled != "true" || user.Active != "true" {
		t.Fatalf("unexpected user row: %#v", user)
	}

	workspace := findListRow(t, rows, "workspace-ext", "workspace")
	if workspace.Enabled != "false" || workspace.Active != "false" {
		t.Fatalf("unexpected workspace row: %#v", workspace)
	}
}

func TestInspectPrintsManifestPathOverridesAndHooks(t *testing.T) {
	deps := newTestDeps(t)

	writeManifestJSON(t, bundledExtensionDir(deps.bundledRoot, "shared"), manifestWithPromptHook("shared", "1.0.0"))
	writeManifestJSON(t, userExtensionDir(deps.homeDir, "shared"), manifestWithPromptHook("shared", "1.1.0"))
	writeManifestJSON(t, workspaceExtensionDir(deps.workspaceRoot, "shared"), manifestWithPromptHook("shared", "1.2.0"))
	enableUserExtension(t, deps.homeDir, "shared")
	enableWorkspaceExtension(t, deps.homeDir, deps.workspaceRoot, "shared")

	output, err := executeExtCommand(t, deps, "inspect", "shared")
	if err != nil {
		t.Fatalf("execute ext inspect: %v\noutput:\n%s", err, output)
	}

	workspaceManifestPath := filepath.Join(
		deps.workspaceRoot,
		".compozy",
		"extensions",
		"shared",
		extensions.ManifestFileNameJSON,
	)
	if !strings.Contains(output, workspaceManifestPath) {
		t.Fatalf("expected inspect output to include manifest path %q\noutput:\n%s", workspaceManifestPath, output)
	}
	if !strings.Contains(output, `"extension": {`) ||
		!strings.Contains(output, `"security": {`) ||
		!strings.Contains(output, `"hooks": [`) {
		t.Fatalf("expected inspect output to include manifest sections\noutput:\n%s", output)
	}
	if !strings.Contains(output, "winner=workspace@") || !strings.Contains(output, "loser=user@") {
		t.Fatalf("expected inspect output to describe overrides\noutput:\n%s", output)
	}
	if !strings.Contains(output, "prompt.post_build priority=500 required=false timeout=0s") {
		t.Fatalf("expected inspect output to include active hook declarations\noutput:\n%s", output)
	}
}

func TestInspectUnknownReturnsHumanReadableError(t *testing.T) {
	deps := newTestDeps(t)
	output, err := executeExtCommand(t, deps, "inspect", "missing-ext")
	if err == nil {
		t.Fatalf("expected inspect unknown extension to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `extension "missing-ext" not found`) {
		t.Fatalf("unexpected inspect error: %v", err)
	}
}

func TestInstallRequiresYesWhenNonInteractive(t *testing.T) {
	deps := newTestDeps(t)
	sourceDir := filepath.Join(t.TempDir(), "needs-confirmation")
	writeManifestJSON(t, sourceDir, manifestFixture("needs-confirmation"))

	output, err := executeExtCommand(t, deps, "install", sourceDir)
	if err == nil {
		t.Fatalf("expected install without --yes to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), "requires --yes in non-interactive mode") {
		t.Fatalf("unexpected install error: %v", err)
	}
	if !strings.Contains(output, "Capabilities: (none)") {
		t.Fatalf("expected install plan to print capabilities\noutput:\n%s", output)
	}
}

func TestInstallRejectsAlreadyInstalledTarget(t *testing.T) {
	deps := newTestDeps(t)
	firstSource := filepath.Join(t.TempDir(), "first")
	secondSource := filepath.Join(t.TempDir(), "second")
	writeManifestJSON(t, firstSource, manifestFixture("duplicate-name"))
	writeManifestJSON(t, secondSource, manifestFixture("duplicate-name"))

	if _, err := executeExtCommand(t, deps, "install", "--yes", firstSource); err != nil {
		t.Fatalf("install first extension: %v", err)
	}

	output, err := executeExtCommand(t, deps, "install", "--yes", secondSource)
	if err == nil {
		t.Fatalf("expected duplicate install to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `user extension "duplicate-name" already exists`) {
		t.Fatalf("unexpected duplicate install error: %v", err)
	}
}

func TestInstallRejectsAlreadyInstalledPath(t *testing.T) {
	deps := newTestDeps(t)
	installedPath := userExtensionDir(deps.homeDir, "same-path")
	writeManifestJSON(t, installedPath, manifestFixture("same-path"))

	output, err := executeExtCommand(t, deps, "install", "--yes", installedPath)
	if err == nil {
		t.Fatalf("expected same-path install to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `extension "same-path" is already installed`) {
		t.Fatalf("unexpected same-path error: %v", err)
	}
}

func TestInstallWithYesCopiesDirectoryAndRecordsDisabledState(t *testing.T) {
	deps := newTestDeps(t)
	sourceDir := filepath.Join(t.TempDir(), "sample-ext")
	writeManifestJSON(t, sourceDir, manifestFixture("sample-ext"))
	writeTestFile(t, filepath.Join(sourceDir, "resources", "notes.txt"), "hello\n")

	output, err := executeExtCommand(t, deps, "install", "--yes", sourceDir)
	if err != nil {
		t.Fatalf("execute ext install --yes: %v\noutput:\n%s", err, output)
	}

	installPath := filepath.Join(deps.homeDir, ".compozy", "extensions", "sample-ext")
	if _, statErr := os.Stat(filepath.Join(installPath, extensions.ManifestFileNameJSON)); statErr != nil {
		t.Fatalf("expected installed manifest: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(installPath, "resources", "notes.txt")); statErr != nil {
		t.Fatalf("expected installed resource file: %v", statErr)
	}

	store := mustEnablementStore(t, deps.homeDir)
	enabled, err := store.Enabled(
		context.Background(),
		extensions.Ref{Name: "sample-ext", Source: extensions.SourceUser},
	)
	if err != nil {
		t.Fatalf("load installed enablement state: %v", err)
	}
	if enabled {
		t.Fatal("installed user extension should default to disabled")
	}

	statePath := filepath.Join(installPath, ".compozy-state.json")
	if _, statErr := os.Stat(statePath); statErr != nil {
		t.Fatalf("expected recorded user state file: %v", statErr)
	}
	if !strings.Contains(output, `Installed extension "sample-ext"`) || !strings.Contains(output, "disabled") {
		t.Fatalf("unexpected install output:\n%s", output)
	}
}

func TestUninstallBundledRefuses(t *testing.T) {
	deps := newTestDeps(t)
	writeManifestJSON(t, bundledExtensionDir(deps.bundledRoot, "bundled-ext"), manifestFixture("bundled-ext"))

	output, err := executeExtCommand(t, deps, "uninstall", "bundled-ext")
	if err == nil {
		t.Fatalf("expected bundled uninstall to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `refuse to uninstall bundled extension "bundled-ext"`) {
		t.Fatalf("unexpected bundled uninstall error: %v", err)
	}
}

func TestUninstallWorkspaceRefuses(t *testing.T) {
	deps := newTestDeps(t)
	writeManifestJSON(t, workspaceExtensionDir(deps.workspaceRoot, "workspace-ext"), manifestFixture("workspace-ext"))

	output, err := executeExtCommand(t, deps, "uninstall", "workspace-ext")
	if err == nil {
		t.Fatalf("expected workspace uninstall to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `refuse to uninstall workspace extension "workspace-ext"`) {
		t.Fatalf("unexpected workspace uninstall error: %v", err)
	}
}

func TestUninstallUserRemovesDirectoryAndState(t *testing.T) {
	deps := newTestDeps(t)
	sourceDir := filepath.Join(t.TempDir(), "remove-me")
	writeManifestJSON(t, sourceDir, manifestFixture("remove-me"))

	if _, err := executeExtCommand(t, deps, "install", "--yes", sourceDir); err != nil {
		t.Fatalf("install remove-me: %v", err)
	}
	enableUserExtension(t, deps.homeDir, "remove-me")

	output, err := executeExtCommand(t, deps, "uninstall", "remove-me")
	if err != nil {
		t.Fatalf("execute ext uninstall: %v\noutput:\n%s", err, output)
	}

	installPath := filepath.Join(deps.homeDir, ".compozy", "extensions", "remove-me")
	if _, statErr := os.Stat(installPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected installed directory to be removed, stat err=%v", statErr)
	}

	store := mustEnablementStore(t, deps.homeDir)
	enabled, err := store.Enabled(
		context.Background(),
		extensions.Ref{Name: "remove-me", Source: extensions.SourceUser},
	)
	if err != nil {
		t.Fatalf("load user enablement after uninstall: %v", err)
	}
	if enabled {
		t.Fatal("uninstalled user extension should not remain enabled")
	}
}

func TestEnableMarksExtensionEnabled(t *testing.T) {
	deps := newTestDeps(t)
	sourceDir := filepath.Join(t.TempDir(), "toggle-me")
	writeManifestJSON(t, sourceDir, manifestFixture("toggle-me"))
	if _, err := executeExtCommand(t, deps, "install", "--yes", sourceDir); err != nil {
		t.Fatalf("install toggle-me: %v", err)
	}

	output, err := executeExtCommand(t, deps, "enable", "toggle-me")
	if err != nil {
		t.Fatalf("execute ext enable: %v\noutput:\n%s", err, output)
	}

	store := mustEnablementStore(t, deps.homeDir)
	enabled, err := store.Enabled(
		context.Background(),
		extensions.Ref{Name: "toggle-me", Source: extensions.SourceUser},
	)
	if err != nil {
		t.Fatalf("load user enablement: %v", err)
	}
	if !enabled {
		t.Fatal("expected user extension to be enabled")
	}
}

func TestDisableBundledExtensionFails(t *testing.T) {
	deps := newTestDeps(t)
	writeManifestJSON(t, bundledExtensionDir(deps.bundledRoot, "bundled-ext"), manifestFixture("bundled-ext"))

	output, err := executeExtCommand(t, deps, "disable", "bundled-ext")
	if err == nil {
		t.Fatalf("expected bundled disable to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `bundled extension "bundled-ext" cannot be disabled`) {
		t.Fatalf("unexpected bundled disable error: %v", err)
	}
}

func TestDisableMarksExtensionDisabled(t *testing.T) {
	deps := newTestDeps(t)
	sourceDir := filepath.Join(t.TempDir(), "disable-me")
	writeManifestJSON(t, sourceDir, manifestFixture("disable-me"))
	if _, err := executeExtCommand(t, deps, "install", "--yes", sourceDir); err != nil {
		t.Fatalf("install disable-me: %v", err)
	}
	enableUserExtension(t, deps.homeDir, "disable-me")

	output, err := executeExtCommand(t, deps, "disable", "disable-me")
	if err != nil {
		t.Fatalf("execute ext disable: %v\noutput:\n%s", err, output)
	}

	store := mustEnablementStore(t, deps.homeDir)
	enabled, err := store.Enabled(
		context.Background(),
		extensions.Ref{Name: "disable-me", Source: extensions.SourceUser},
	)
	if err != nil {
		t.Fatalf("load user enablement: %v", err)
	}
	if enabled {
		t.Fatal("expected user extension to be disabled")
	}
}

func TestUninstallMissingUserExtensionReturnsError(t *testing.T) {
	deps := newTestDeps(t)

	output, err := executeExtCommand(t, deps, "uninstall", "missing-user-ext")
	if err == nil {
		t.Fatalf("expected missing uninstall to fail\noutput:\n%s", output)
	}
	if !strings.Contains(err.Error(), `user extension "missing-user-ext" is not installed`) {
		t.Fatalf("unexpected missing uninstall error: %v", err)
	}
}

func TestInstallEnableDisableUninstallRoundTrip(t *testing.T) {
	deps := newTestDeps(t)
	sourceDir := filepath.Join(t.TempDir(), "round-trip")
	writeManifestJSON(t, sourceDir, manifestFixture("round-trip"))

	if _, err := executeExtCommand(t, deps, "install", "--yes", sourceDir); err != nil {
		t.Fatalf("install round-trip: %v", err)
	}
	if _, err := executeExtCommand(t, deps, "enable", "round-trip"); err != nil {
		t.Fatalf("enable round-trip: %v", err)
	}

	listOutput, err := executeExtCommand(t, deps, "list")
	if err != nil {
		t.Fatalf("list after enable: %v\noutput:\n%s", err, listOutput)
	}
	enabledRow := findListRow(t, parseListRows(t, listOutput), "round-trip", "user")
	if enabledRow.Enabled != "true" || enabledRow.Active != "true" {
		t.Fatalf("expected enabled row after enable, got %#v", enabledRow)
	}

	if _, err := executeExtCommand(t, deps, "disable", "round-trip"); err != nil {
		t.Fatalf("disable round-trip: %v", err)
	}
	listOutput, err = executeExtCommand(t, deps, "list")
	if err != nil {
		t.Fatalf("list after disable: %v\noutput:\n%s", err, listOutput)
	}
	disabledRow := findListRow(t, parseListRows(t, listOutput), "round-trip", "user")
	if disabledRow.Enabled != "false" || disabledRow.Active != "false" {
		t.Fatalf("expected disabled row after disable, got %#v", disabledRow)
	}

	if _, err := executeExtCommand(t, deps, "uninstall", "round-trip"); err != nil {
		t.Fatalf("uninstall round-trip: %v", err)
	}
	listOutput, err = executeExtCommand(t, deps, "list")
	if err != nil {
		t.Fatalf("list after uninstall: %v\noutput:\n%s", err, listOutput)
	}
	for _, row := range parseListRows(t, listOutput) {
		if row.Name == "round-trip" {
			t.Fatalf("expected round-trip extension to be absent after uninstall\noutput:\n%s", listOutput)
		}
	}
}

func TestResolveEnvUsesInjectedHomeAndWorkspace(t *testing.T) {
	deps := newTestDeps(t)

	env, err := deps.resolveEnv(context.Background())
	if err != nil {
		t.Fatalf("resolveEnv: %v", err)
	}
	if env.homeDir != deps.homeDir {
		t.Fatalf("unexpected home dir: %q", env.homeDir)
	}
	if env.workspaceRoot != deps.workspaceRoot {
		t.Fatalf("unexpected workspace root: %q", env.workspaceRoot)
	}
	if env.store == nil {
		t.Fatal("expected enablement store")
	}
}

func TestPathExistsAndRenderHooksHelpers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists.txt")
	writeTestFile(t, path, "data")

	exists, err := pathExists(path)
	if err != nil {
		t.Fatalf("pathExists existing path: %v", err)
	}
	if !exists {
		t.Fatal("expected existing path to be reported")
	}

	exists, err = pathExists(filepath.Join(t.TempDir(), "missing.txt"))
	if err != nil {
		t.Fatalf("pathExists missing path: %v", err)
	}
	if exists {
		t.Fatal("expected missing path to be reported as absent")
	}

	if hooks := renderHooks(nil); len(hooks) != 1 || hooks[0] != "(none)" {
		t.Fatalf("unexpected renderHooks(nil) result: %v", hooks)
	}
}

func TestDefaultCommandDepsAndHelperFunctions(t *testing.T) {
	deps := defaultCommandDeps()
	if deps.resolveHomeDir == nil ||
		deps.resolveWorkspaceRoot == nil ||
		deps.isInteractive == nil ||
		deps.confirmInstall == nil ||
		deps.loadManifest == nil ||
		deps.newEnablementStore == nil ||
		deps.discover == nil ||
		deps.copyDir == nil ||
		deps.removeAll == nil ||
		deps.pathExists == nil {
		t.Fatalf("expected default command deps to populate every callback: %#v", deps)
	}

	root, err := deps.resolveWorkspaceRoot(context.Background())
	if err != nil {
		t.Fatalf("resolveWorkspaceRoot: %v", err)
	}
	if strings.TrimSpace(root) == "" {
		t.Fatal("expected non-empty workspace root")
	}

	exists, err := deps.pathExists(".")
	if err != nil {
		t.Fatalf("pathExists on cwd: %v", err)
	}
	if !exists {
		t.Fatal("expected cwd to exist")
	}

	_ = isInteractiveTerminal()
}

func TestWriteInstallPlanAndSortedCapabilities(t *testing.T) {
	cmd := &cobra.Command{Use: "install"}
	var output bytes.Buffer
	cmd.SetOut(&output)

	capabilities := sortedCapabilities([]extensions.Capability{
		extensions.CapabilityTasksCreate,
		extensions.CapabilityArtifactsRead,
	})
	if got := []extensions.Capability{capabilities[0], capabilities[1]}; got[0] != extensions.CapabilityArtifactsRead ||
		got[1] != extensions.CapabilityTasksCreate {
		t.Fatalf("unexpected sorted capabilities: %v", got)
	}

	if err := writeInstallPlan(cmd, installPrompt{
		Name:         "planner",
		SourcePath:   "/tmp/src",
		InstallPath:  "/tmp/dst",
		Capabilities: capabilities,
	}); err != nil {
		t.Fatalf("writeInstallPlan: %v", err)
	}
	if !strings.Contains(output.String(), "Extension: planner") ||
		!strings.Contains(output.String(), "artifacts.read") {
		t.Fatalf("unexpected install plan output:\n%s", output.String())
	}
}

func TestDiscoverExtensionsUsesRequest(t *testing.T) {
	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	store := mustEnablementStore(t, homeDir)

	result, err := discoverExtensions(context.Background(), discoverRequest{
		HomeDir:         homeDir,
		WorkspaceRoot:   workspaceRoot,
		IncludeDisabled: true,
		Store:           store,
	})
	if err != nil {
		t.Fatalf("discoverExtensions: %v", err)
	}
	if len(result.Discovered) != 0 || len(result.Extensions) != 0 {
		t.Fatalf("expected no discovered extensions, got %#v", result)
	}
}

func TestToggleEntryRejectsBundledMutations(t *testing.T) {
	store := mustEnablementStore(t, t.TempDir())
	entry := extensions.DiscoveredExtension{
		Ref: extensions.Ref{
			Name:   "bundled-ext",
			Source: extensions.SourceBundled,
		},
	}

	if err := toggleEntry(context.Background(), store, entry, true); err == nil {
		t.Fatal("expected enabling bundled extension to fail")
	}
	if err := toggleEntry(context.Background(), store, entry, false); err == nil {
		t.Fatal("expected disabling bundled extension to fail")
	}
}

func TestConfirmInstallBehaviors(t *testing.T) {
	cmd := &cobra.Command{Use: "install"}

	deps := defaultCommandDeps()
	deps.isInteractive = func() bool { return false }
	if err := confirmInstall(cmd, deps, installPrompt{Name: "sample"}, true); err != nil {
		t.Fatalf("confirmInstall yes path: %v", err)
	}

	if err := confirmInstall(cmd, deps, installPrompt{Name: "sample"}, false); err == nil {
		t.Fatal("expected non-interactive install confirmation to fail")
	}

	deps.isInteractive = func() bool { return true }
	deps.confirmInstall = func(*cobra.Command, installPrompt) (bool, error) { return false, nil }
	if err := confirmInstall(cmd, deps, installPrompt{Name: "sample"}, false); err == nil {
		t.Fatal("expected canceled install confirmation to fail")
	}

	deps.confirmInstall = func(*cobra.Command, installPrompt) (bool, error) { return true, nil }
	if err := confirmInstall(cmd, deps, installPrompt{Name: "sample"}, false); err != nil {
		t.Fatalf("confirmInstall confirmed path: %v", err)
	}
}

func TestResolveSourcePathAndInstallTargetValidation(t *testing.T) {
	if _, err := resolveSourcePath(""); err == nil {
		t.Fatal("expected empty source path to fail")
	}

	filePath := filepath.Join(t.TempDir(), "not-a-dir.txt")
	writeTestFile(t, filePath, "content")
	if _, err := resolveSourcePath(filePath); err == nil {
		t.Fatal("expected file source path to fail")
	}

	sourceDir := filepath.Join(t.TempDir(), "ext-dir")
	writeManifestJSON(t, sourceDir, manifestFixture("ext-dir"))
	if _, err := resolveSourcePath(sourceDir); err != nil {
		t.Fatalf("resolveSourcePath directory: %v", err)
	}

	deps := defaultCommandDeps()
	deps.pathExists = func(string) (bool, error) { return true, nil }
	if err := ensureInstallTargetAvailable(deps, filepath.Join(t.TempDir(), "ext-dir"), "ext-dir"); err == nil {
		t.Fatal("expected existing install target to fail")
	}
}

func TestValidateCopyTargetRejectsNestedDestination(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "source")
	nestedDest := filepath.Join(sourceDir, "nested")
	if err := validateCopyTarget(sourceDir, nestedDest); err == nil {
		t.Fatal("expected nested destination to fail")
	}

	siblingDest := filepath.Join(t.TempDir(), "dest")
	if err := validateCopyTarget(sourceDir, siblingDest); err != nil {
		t.Fatalf("validateCopyTarget sibling destination: %v", err)
	}
}

func TestCopyDirectoryTreeCopiesFilesAndSymlinks(t *testing.T) {
	sourceDir := filepath.Join(t.TempDir(), "copy-source")
	destDir := filepath.Join(t.TempDir(), "copy-dest")

	writeTestFile(t, filepath.Join(sourceDir, "nested", "data.txt"), "hello\n")
	linkSource := filepath.Join(sourceDir, "nested", "data.txt")
	linkPath := filepath.Join(sourceDir, "link.txt")
	if err := os.Symlink(linkSource, linkPath); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	if err := copyDirectoryTree(sourceDir, destDir); err != nil {
		t.Fatalf("copyDirectoryTree: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(destDir, "nested", "data.txt"))
	if err != nil {
		t.Fatalf("read copied file: %v", err)
	}
	if string(content) != "hello\n" {
		t.Fatalf("unexpected copied content: %q", string(content))
	}

	linkTarget, err := os.Readlink(filepath.Join(destDir, "link.txt"))
	if err != nil {
		t.Fatalf("read copied symlink: %v", err)
	}
	if linkTarget != linkSource {
		t.Fatalf("unexpected copied symlink target: %q", linkTarget)
	}
}

func TestAppendDiscoveryFailureNotesMatchesName(t *testing.T) {
	var buf strings.Builder
	result := extensions.DiscoveryResult{
		Failures: []extensions.DiscoveryFailure{
			{
				Source:       extensions.SourceWorkspace,
				ExtensionDir: "/tmp/workspace/shared",
				ManifestPath: "/tmp/workspace/shared/extension.json",
				Err:          os.ErrNotExist,
			},
		},
	}

	appendDiscoveryFailureNotes(context.Background(), &buf, result, "shared")
	if !strings.Contains(buf.String(), "Discovery failures:") || !strings.Contains(buf.String(), "extension.json") {
		t.Fatalf("expected discovery failure notes, got:\n%s", buf.String())
	}
}

func executeExtCommand(t *testing.T, deps testDeps, args ...string) (string, error) {
	t.Helper()

	cmd := newExtCommandWithDeps(nil, deps.commandDeps)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return output.String(), err
}

func newTestDeps(t *testing.T) testDeps {
	t.Helper()

	homeDir := t.TempDir()
	workspaceRoot := t.TempDir()
	bundledRoot := t.TempDir()

	deps := defaultCommandDeps()
	deps.resolveHomeDir = func() (string, error) { return homeDir, nil }
	deps.resolveWorkspaceRoot = func(context.Context) (string, error) { return workspaceRoot, nil }
	deps.isInteractive = func() bool { return false }
	deps.confirmInstall = func(*cobra.Command, installPrompt) (bool, error) { return true, nil }
	deps.discover = func(ctx context.Context, req discoverRequest) (extensions.DiscoveryResult, error) {
		return extensions.Discovery{
			WorkspaceRoot:   req.WorkspaceRoot,
			HomeDir:         req.HomeDir,
			IncludeDisabled: req.IncludeDisabled,
			Enablement:      req.Store,
			BundledFS:       os.DirFS(bundledRoot),
		}.Discover(ctx)
	}

	return testDeps{
		commandDeps:   deps,
		homeDir:       homeDir,
		workspaceRoot: workspaceRoot,
		bundledRoot:   bundledRoot,
	}
}

type testDeps struct {
	commandDeps
	homeDir       string
	workspaceRoot string
	bundledRoot   string
}

func parseListRows(t *testing.T, output string) []listRow {
	t.Helper()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("expected list output to contain a header line")
	}

	rows := make([]listRow, 0, len(lines)-1)
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			t.Fatalf("unexpected list row %q", line)
		}
		rows = append(rows, listRow{
			Name:         fields[0],
			Version:      fields[1],
			Source:       fields[2],
			Enabled:      fields[3],
			Active:       fields[4],
			Capabilities: fields[5],
		})
	}
	return rows
}

func findListRow(t *testing.T, rows []listRow, name string, source string) listRow {
	t.Helper()

	for _, row := range rows {
		if row.Name == name && row.Source == source {
			return row
		}
	}
	t.Fatalf("list row not found for %s/%s", source, name)
	return listRow{}
}

func manifestFixture(name string) *extensions.Manifest {
	return &extensions.Manifest{
		Extension: extensions.ExtensionInfo{
			Name:              name,
			Version:           "1.0.0",
			Description:       "Fixture " + name,
			MinCompozyVersion: "0.0.1",
		},
		Security: extensions.SecurityConfig{},
	}
}

func manifestWithPromptHook(name string, version string) *extensions.Manifest {
	manifest := manifestFixture(name)
	manifest.Extension.Version = version
	manifest.Subprocess = &extensions.SubprocessConfig{Command: "bin/" + name}
	manifest.Security.Capabilities = []extensions.Capability{extensions.CapabilityPromptMutate}
	manifest.Hooks = []extensions.HookDeclaration{
		{
			Event:    extensions.HookPromptPostBuild,
			Priority: extensions.DefaultHookPriority,
		},
	}
	return manifest
}

func writeManifestJSON(t *testing.T, dir string, manifest *extensions.Manifest) {
	t.Helper()

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	writeTestFile(t, filepath.Join(dir, extensions.ManifestFileNameJSON), string(data))
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}

func mustEnablementStore(t *testing.T, homeDir string) *extensions.EnablementStore {
	t.Helper()

	store, err := extensions.NewEnablementStore(context.Background(), homeDir)
	if err != nil {
		t.Fatalf("create enablement store: %v", err)
	}
	return store
}

func enableUserExtension(t *testing.T, homeDir string, name string) {
	t.Helper()

	if err := mustEnablementStore(t, homeDir).Enable(
		context.Background(),
		extensions.Ref{Name: name, Source: extensions.SourceUser},
	); err != nil {
		t.Fatalf("enable user extension %q: %v", name, err)
	}
}

func enableWorkspaceExtension(t *testing.T, homeDir string, workspaceRoot string, name string) {
	t.Helper()

	if err := mustEnablementStore(t, homeDir).Enable(
		context.Background(),
		extensions.Ref{
			Name:          name,
			Source:        extensions.SourceWorkspace,
			WorkspaceRoot: workspaceRoot,
		},
	); err != nil {
		t.Fatalf("enable workspace extension %q: %v", name, err)
	}
}

func bundledExtensionDir(root string, name string) string {
	return filepath.Join(root, name)
}

func userExtensionDir(homeDir string, name string) string {
	return filepath.Join(homeDir, ".compozy", "extensions", name)
}

func workspaceExtensionDir(workspaceRoot string, name string) string {
	return filepath.Join(workspaceRoot, ".compozy", "extensions", name)
}
