//go:build integration && !windows

package daemon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// E2E-002 and E2E-003 exercise the frozen human CLI transcripts against a real daemon.
func TestDaemonE2EAgentPluginCLIJourneys(t *testing.T) {
	t.Run("Should match the portable golden-path transcript", testDaemonE2EAgentPluginCLIGoldenPath)
	t.Run("Should match the portable failure-path transcripts", testDaemonE2EAgentPluginCLIFailurePaths)
}

func testDaemonE2EAgentPluginCLIGoldenPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	const credential = "agent-plugin-cli-e2e-token"
	githubServer := newDistributionGitHubServer(t, credential)
	t.Cleanup(githubServer.Close)
	fixtureDir := filepath.Join("..", "extension", "testdata", "agent-plugin-conformant")
	seedPortableDistributionRelease(
		t,
		githubServer,
		"v1.2.0",
		"acme-tools.tar.gz",
		archivePortableFixture(t, fixtureDir),
	)

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Extensions.Trust.AllowUnverified = true
			cfg.Extensions.Sources.GitHub.Enabled = true
			cfg.Extensions.Sources.GitHub.BaseURL = githubServer.URL
		}},
		Env: map[string]string{"GITHUB_TOKEN": credential},
	})

	installOut := runAgentPluginHumanCLI(
		t,
		ctx,
		harness,
		true,
		"extension", "install", "github:acme/hello", "--allow-unverified", "--yes",
	)
	assertAgentPluginTranscriptOrder(t, installOut,
		"✓ install acme.tools",
		"Format: agent plugin",
		"Ingested",
		"skill",
		"Skipped",
		"legacy-events",
		"next: compozy extension enable acme.tools",
		"Extension",
		"Name:",
		"acme.tools",
		"Summary:",
		"disabled",
	)

	enableOut := runAgentPluginHumanCLI(
		t, ctx, harness, true, "extension", "enable", "acme.tools",
	)
	assertAgentPluginTranscriptOrder(t, enableOut,
		"✓ enable acme.tools",
		"next: compozy extension status acme.tools",
	)

	statusOut := runAgentPluginHumanCLI(
		t, ctx, harness, true, "extension", "status", "acme.tools",
	)
	assertAgentPluginTranscriptOrder(t, statusOut,
		"Extension",
		"Name:",
		"acme.tools",
		"Format:",
		"agent plugin",
		"Summary:",
		"components skipped)",
		"Diagnostics",
		"WARN",
		"legacy-events",
	)

	validateOut := runAgentPluginHumanCLI(
		t, ctx, harness, true, "extension", "validate", fixtureDir,
	)
	assertAgentPluginTranscriptOrder(t, validateOut,
		"Extension bundle validation",
		"Status:",
		"valid",
		"Format:",
		"agent plugin",
		"Package:",
		"acme.tools 1.2.0",
		"Would ingest:",
		"WARN",
	)

	dataPath, err := harness.HomePaths.ExtensionDataPath("acme.tools", "")
	if err != nil {
		t.Fatalf("ExtensionDataPath(acme.tools) error = %v", err)
	}
	if err := os.MkdirAll(dataPath, 0o700); err != nil {
		t.Fatalf("MkdirAll(portable data path) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataPath, "state.json"), []byte(`{"ready":true}`), 0o600); err != nil {
		t.Fatalf("WriteFile(portable data state) error = %v", err)
	}

	updateOut := runAgentPluginHumanCLI(
		t, ctx, harness, true, "extension", "update", "acme.tools",
	)
	assertAgentPluginTranscriptOrder(t, updateOut,
		"✓ update acme.tools",
		"Format: agent plugin",
		"Ingested",
		"note: data directory preserved ("+dataPath+")",
	)
	if _, err := os.Stat(filepath.Join(dataPath, "state.json")); err != nil {
		t.Fatalf("portable update did not preserve data: %v", err)
	}

	removeOut := runAgentPluginHumanCLI(
		t, ctx, harness, true, "extension", "remove", "acme.tools", "--global",
	)
	assertAgentPluginTranscriptOrder(t, removeOut,
		"✓ remove acme.tools",
		"removed: "+extensionpkg.ManagedInstallPath(harness.HomePaths, "acme.tools"),
		"removed: "+dataPath,
	)
}

func testDaemonE2EAgentPluginCLIFailurePaths(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Extensions.Trust.AllowUnverified = true
		}},
	})

	clientLayout := t.TempDir()
	if err := os.MkdirAll(filepath.Join(clientLayout, ".claude-plugin"), 0o700); err != nil {
		t.Fatalf("MkdirAll(.claude-plugin) error = %v", err)
	}
	writeAgentPluginE2EManifest(t, filepath.Join(clientLayout, ".claude-plugin", "plugin.json"), "portable-client")
	clientLayoutOut := runAgentPluginHumanCLI(
		t,
		ctx,
		harness,
		false,
		"extension", "install", clientLayout, "--allow-unverified", "--yes",
	)
	for _, want := range []string{
		"error: extension: " + clientLayout + " is a Claude Code plugin (.claude-plugin/plugin.json)",
		"not an Agent Plugins package",
		"plugin.json at the package root",
	} {
		if !strings.Contains(clientLayoutOut, want) {
			t.Fatalf("client-layout output = %q, want %q", clientLayoutOut, want)
		}
	}

	badName := t.TempDir()
	writeAgentPluginE2EManifest(t, filepath.Join(badName, "plugin.json"), "Bad--Name")
	badNameOut := runAgentPluginHumanCLI(
		t,
		ctx,
		harness,
		false,
		"extension", "install", badName, "--allow-unverified", "--yes",
	)
	wantBadName := `error: extension: agent plugin manifest invalid: name "Bad--Name" must be 1-64 lowercase letters, digits, dots, or hyphens, start and end alphanumeric, without "--" or ".."`
	if !strings.Contains(badNameOut, wantBadName) {
		t.Fatalf("bad-name output = %q, want %q", badNameOut, wantBadName)
	}

	dualTarget := t.TempDir()
	writeNativeAgentPluginE2EManifest(t, dualTarget, "dual-target")
	writeAgentPluginE2EManifest(t, filepath.Join(dualTarget, "plugin.json"), "dual-portable")
	dualOut := runAgentPluginHumanCLI(
		t,
		ctx,
		harness,
		true,
		"extension", "install", dualTarget, "--allow-unverified", "--yes",
	)
	note := "note: directory carries both extension.toml and plugin.json; installed as a Compozy extension (native manifest wins)"
	if strings.Count(dualOut, note) != 1 {
		t.Fatalf("dual-manifest output = %q, want note exactly once", dualOut)
	}

	validateLayoutOut := runAgentPluginHumanCLI(
		t, ctx, harness, false, "extension", "validate", clientLayout,
	)
	if !strings.Contains(validateLayoutOut, "error: extension: "+clientLayout+" is a Claude Code plugin") {
		t.Fatalf("client-layout validation output = %q, want the install detection error", validateLayoutOut)
	}
}

// E2E-001 walks the minimum portable runtime distribution contract through a
// real daemon, CLI, ACP session, hosted MCP proxy, and stdio subprocess.
func TestDaemonE2EAgentPluginRuntimeDistribution(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Extensions.Trust.AllowUnverified = true
		}},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "portable_agent_plugin_fixture.json"),
			FixtureAgent: "portable-agent-plugin",
			AgentName:    "portable-e2e",
		}},
	})

	sourceDir := preparePortableAgentPluginGitSource(t, ctx)
	var installed compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&installed,
		"extension", "install", sourceDir, "--allow-unverified", "--yes", "-o", "json",
	)
	if installed.Name != "acme.tools" || installed.Version != "1.2.0" {
		t.Fatalf("installed portable package = %#v, want acme.tools@1.2.0", installed)
	}
	if installed.Enabled {
		t.Fatalf("installed portable package = %#v, want inert before enable", installed)
	}
	dataPath, err := harness.HomePaths.ExtensionDataPath(installed.Name, "")
	if err != nil {
		t.Fatalf("ExtensionDataPath(%q) error = %v", installed.Name, err)
	}
	if _, err := os.Stat(dataPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("portable data path before first MCP launch stat error = %v, want not-exist", err)
	}

	enabled, err := harness.EnableExtension(ctx, installed.Name)
	if err != nil {
		t.Fatalf("EnableExtension(%q) error = %v", installed.Name, err)
	}
	if !enabled.Enabled {
		t.Fatalf("enabled portable package = %#v, want enabled", enabled)
	}

	active := createFixtureBackedSession(t, ctx, harness, "portable-e2e", "portable-distribution")
	command := requirePortableSkillCommand(t, ctx, harness, active.ID, installed.Name, "review")
	stream, err := harness.PromptSessionHTTP(
		ctx,
		active.ID,
		command.CanonicalToken+" activate portable skill",
	)
	if err != nil {
		t.Fatalf("PromptSessionHTTP(portable skill) error = %v", err)
	}
	assertSuccessfulExtensionPromptStream(t, stream, "portable skill")
	assertPersistedExtensionSkillInvocation(t, ctx, harness, active.ID, command)

	registration, ok := harness.MockAgentRegistration("portable-e2e")
	if !ok {
		t.Fatal("MockAgentRegistration(portable-e2e) = missing")
	}
	records, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(portable-e2e) error = %v", err)
	}
	sessionRecords := acpmock.DiagnosticsForCompozySession(records, active.ID)
	client := startHostedMCPClient(
		t,
		ctx,
		requireHostedMCPStdioServer(t, sessionRecords, hostedMCPServerEarliest),
	)
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("Close(portable hosted MCP client) error = %v", closeErr)
		}
	}()

	const toolID = "mcp__local__echo_environment"
	listed, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools(portable hosted MCP) error = %v", err)
	}
	if !sdkToolListContains(listed.Tools, toolID) {
		t.Fatalf("portable hosted MCP tools = %#v, want %q", sdkToolNames(listed.Tools), toolID)
	}
	result, err := client.CallTool(ctx, &sdkmcp.CallToolParams{Name: toolID, Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolID, err)
	}
	assertPortableRuntimeEnvironment(t, result, harness, installed.Name, dataPath)
}

func preparePortableAgentPluginGitSource(t testing.TB, ctx context.Context) string {
	t.Helper()

	sourceDir := filepath.Join(t.TempDir(), "acme-tools")
	fixtureDir := filepath.Join("..", "extension", "testdata", "agent-plugin-conformant")
	if err := os.CopyFS(sourceDir, os.DirFS(fixtureDir)); err != nil {
		t.Fatalf("CopyFS(agent-plugin-conformant) error = %v", err)
	}
	for _, args := range [][]string{
		{"init"},
		{"add", "."},
		{"-c", "user.name=Compozy E2E", "-c", "user.email=e2e@compozy.test", "commit", "-m", "fixture"},
	} {
		command := exec.CommandContext(ctx, "git", args...)
		command.Dir = sourceDir
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s error = %v; output=%s", strings.Join(args, " "), err, output)
		}
	}
	return sourceDir
}

func runAgentPluginHumanCLI(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	wantSuccess bool,
	args ...string,
) string {
	t.Helper()
	stdout, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, args...)
	if wantSuccess {
		if err != nil {
			t.Fatalf("CLI %q error = %v; stdout=%s stderr=%s", strings.Join(args, " "), err, stdout, stderr)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Fatalf("CLI %q stderr = %q, want empty", strings.Join(args, " "), stderr)
		}
		return stdout
	}
	if err == nil {
		t.Fatalf("CLI %q error = nil, want failure; stdout=%s", strings.Join(args, " "), stdout)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Fatalf("CLI %q stdout = %q, want failure output on stderr", strings.Join(args, " "), stdout)
	}
	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok || exitErr.ExitCode() != 1 {
		t.Fatalf("CLI %q exit error = %v, want exit code 1", strings.Join(args, " "), err)
	}
	return stderr
}

func assertAgentPluginTranscriptOrder(t *testing.T, output string, tokens ...string) {
	t.Helper()
	offset := 0
	for _, token := range tokens {
		index := strings.Index(output[offset:], token)
		if index < 0 {
			t.Fatalf("CLI transcript = %q, want %q after byte %d", output, token, offset)
		}
		offset += index + len(token)
	}
}

func writeAgentPluginE2EManifest(t *testing.T, path string, name string) {
	t.Helper()
	payload := fmt.Sprintf(
		`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":%q,"version":"1.2.0"}`,
		name,
	)
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func writeNativeAgentPluginE2EManifest(t *testing.T, root string, name string) {
	t.Helper()
	payload := fmt.Sprintf(`[extension]
name = %q
version = "1.0.0"
min_compozy_version = "0.3.0-beta.1"

[capabilities]
provides = []

[permissions]
requires = []
`, name)
	if err := os.WriteFile(filepath.Join(root, "extension.toml"), []byte(payload), 0o600); err != nil {
		t.Fatalf("WriteFile(extension.toml) error = %v", err)
	}
}

func archivePortableFixture(t *testing.T, root string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, pathErr error) error {
		if pathErr != nil {
			return pathErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect archive entry %q: %w", path, err)
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("resolve archive entry %q: %w", path, err)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read archive entry %q: %w", path, err)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("create archive header %q: %w", path, err)
		}
		header.Name = filepath.ToSlash(filepath.Join("acme.tools", relative))
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write archive header %q: %w", path, err)
		}
		if _, err := tarWriter.Write(content); err != nil {
			return fmt.Errorf("write archive entry %q: %w", path, err)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("WalkDir(portable fixture) error = %v", walkErr)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("Close(portable tar writer) error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("Close(portable gzip writer) error = %v", err)
	}
	return buffer.Bytes()
}

func seedPortableDistributionRelease(
	t *testing.T,
	server *distributionGitHubServer,
	tag string,
	assetName string,
	payload []byte,
) {
	t.Helper()
	server.mu.Lock()
	defer server.mu.Unlock()
	server.nextRelease++
	server.nextAsset++
	asset := distributionGitHubAsset{
		ID:                 server.nextAsset,
		Name:               assetName,
		URL:                server.URL + "/assets/" + strconv.FormatInt(server.nextAsset, 10),
		BrowserDownloadURL: server.URL + "/assets/" + strconv.FormatInt(server.nextAsset, 10),
		ContentType:        "application/gzip",
		Size:               int64(len(payload)),
		payload:            append([]byte(nil), payload...),
	}
	release := &distributionGitHubRelease{
		ID:        server.nextRelease,
		Name:      tag,
		TagName:   tag,
		HTMLURL:   server.URL + "/releases/" + url.PathEscape(tag),
		UploadURL: fmt.Sprintf("%s/uploads/%d/assets{?name,label}", server.URL, server.nextRelease),
		Author:    map[string]string{"login": "acme"},
		Assets:    []distributionGitHubAsset{asset},
	}
	server.assets[asset.ID] = asset
	server.releases = append([]*distributionGitHubRelease{release}, server.releases...)
}

func requirePortableSkillCommand(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	extensionName string,
	skillName string,
) compozycontract.SessionCommandPayload {
	t.Helper()

	path := "/api/workspaces/" + url.PathEscape(harness.WorkspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + "/commands"
	var catalog compozycontract.SessionCommandsResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &catalog); err != nil {
		t.Fatalf("HTTP portable session commands error = %v", err)
	}
	for _, command := range catalog.Commands {
		if command.Lane == "skill" && command.DisplayName == "/"+skillName &&
			command.Source.Kind == "extension" && command.Source.ID == extensionName {
			if strings.TrimSpace(command.CanonicalToken) == "" {
				t.Fatalf("portable skill command = %#v, want canonical token", command)
			}
			return command
		}
	}
	t.Fatalf(
		"portable session commands = %#v, want extension %q skill %q",
		catalog.Commands,
		extensionName,
		skillName,
	)
	return compozycontract.SessionCommandPayload{}
}

type portableRuntimeEnvironment struct {
	PluginRoot string `json:"pluginRoot"`
	PluginData string `json:"pluginData"`
	Mode       string `json:"mode"`
	StatePath  string `json:"statePath"`
	UnknownArg string `json:"unknownArg"`
	CWD        string `json:"cwd"`
	Writable   bool   `json:"writable"`
}

func assertPortableRuntimeEnvironment(
	t testing.TB,
	result *sdkmcp.CallToolResult,
	harness *e2etest.RuntimeHarness,
	extensionName string,
	dataPath string,
) {
	t.Helper()

	if result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("portable CallTool result = %#v, want one successful text result", result)
	}
	content, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("portable CallTool content = %#v, want text", result.Content)
	}
	var environment portableRuntimeEnvironment
	if err := json.Unmarshal([]byte(content.Text), &environment); err != nil {
		t.Fatalf("decode portable runtime environment error = %v; payload=%s", err, content.Text)
	}
	pluginRoot := filepath.Join(harness.HomePaths.HomeDir, "extensions", extensionName)
	canonicalRoot, err := filepath.EvalSymlinks(pluginRoot)
	if err != nil {
		t.Fatalf("EvalSymlinks(portable plugin root) error = %v", err)
	}
	canonicalData, err := filepath.EvalSymlinks(dataPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(portable plugin data) error = %v", err)
	}
	want := portableRuntimeEnvironment{
		PluginRoot: canonicalRoot,
		PluginData: canonicalData,
		Mode:       "review",
		StatePath:  filepath.Join(canonicalData, "state.json"),
		UnknownArg: "${UNKNOWN}",
		CWD:        canonicalRoot,
		Writable:   true,
	}
	if environment != want {
		t.Fatalf("portable runtime environment = %#v, want %#v", environment, want)
	}
	state, err := os.ReadFile(environment.StatePath)
	if err != nil {
		t.Fatalf("ReadFile(portable state) error = %v", err)
	}
	if !strings.Contains(string(state), "launched") {
		t.Fatalf("portable state = %q, want launched marker", state)
	}
	if !filepath.IsAbs(environment.PluginRoot) || !filepath.IsAbs(environment.PluginData) {
		t.Fatalf("portable runtime roots = %#v, want absolute paths", environment)
	}
	if got, want := filepath.Dir(environment.StatePath), environment.PluginData; got != want {
		t.Fatalf("portable state parent = %q, want %q", got, want)
	}
	if got, want := environment.CWD, environment.PluginRoot; got != want {
		t.Fatalf("portable cwd = %q, want %q", got, want)
	}
	if environment.Mode != "review" || environment.UnknownArg != "${UNKNOWN}" || !environment.Writable {
		t.Fatalf("portable runtime contract = %#v, want mode/literal/writable checklist", environment)
	}
}
