//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

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
