//go:build integration && !windows

package daemon

// Suite: Extension authoring lifecycle E2E
// Invariant: Code-backed and passive resource-only sources complete the workspace development loop without install trust.
// Boundary IN: CLI build/dev/reload, immutable generations, daemon dev links, and workspace resource projection.
// Boundary OUT: Distribution, marketplace installation, and client-side watch polling.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"golang.org/x/sys/execabs"
)

const (
	extensionAuthoringE2EName             = "authoring-dev-e2e"
	resourceOnlyExtensionAuthoringE2EName = "resource-only-authoring-e2e"
	resourceOnlyAuthoringAgentName        = "resource-only-agent"
	resourceOnlyAuthoringSkillName        = "resource-only-skill"
)

func TestDaemonE2EExtensionAuthoringShouldCompleteTheDevelopmentLoopWithoutTrustPrompts(t *testing.T) {
	t.Run(
		"Should complete the development loop without trust prompts",
		testDaemonE2EExtensionAuthoringShouldCompleteTheDevelopmentLoopWithoutTrustPrompts,
	)
	t.Run(
		"Should complete the resource-only development loop",
		testDaemonE2EResourceOnlyExtensionAuthoring,
	)
}

func testDaemonE2EExtensionAuthoringShouldCompleteTheDevelopmentLoopWithoutTrustPrompts(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath:   binaryPath,
		StartTimeout: 30 * time.Second,
	})
	sourceDir := filepath.Join(harness.WorkspaceRoot, extensionAuthoringE2EName)

	var scaffold extensionpkg.ScaffoldResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&scaffold,
		"extension", "init", extensionAuthoringE2EName,
		"--template", "tool-provider-go",
		"--dir", sourceDir,
		"-o", "json",
	)
	if scaffold.Directory != sourceDir || scaffold.Name != extensionAuthoringE2EName {
		t.Fatalf("extension init result = %#v, want source %q", scaffold, sourceDir)
	}
	configureExtensionAuthoringSDKReplace(t, ctx, sourceDir, repoRoot)
	rewriteExtensionAuthoringGeneration(t, sourceDir, "No results for ", "generation-one:", "generation-one-observed")

	var firstBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&firstBuild,
		"extension", "build", sourceDir, "-o", "json",
	)
	assertExtensionAuthoringValidation(t, ctx, harness, firstBuild.GenerationDir)

	var linked compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&linked,
		"extension", "dev", sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if !linked.Dev || linked.WorkspaceID != harness.WorkspaceID || linked.GenerationHash != firstBuild.GenerationHash {
		t.Fatalf("extension dev result = %#v, want workspace dev generation %q", linked, firstBuild.GenerationHash)
	}
	assertExtensionAuthoringInvocation(t, ctx, harness, "generation-one:alpha")
	assertExtensionAuthoringLogs(t, ctx, harness, "generation-one-observed")

	rewriteExtensionAuthoringGeneration(
		t,
		sourceDir,
		"generation-one:",
		"generation-two:",
		"generation-two-observed",
	)
	var reloaded compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&reloaded,
		"extension", "reload", extensionAuthoringE2EName, sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if reloaded.GenerationHash == firstBuild.GenerationHash || !reloaded.Dev {
		t.Fatalf("extension reload result = %#v, want a new dev generation", reloaded)
	}
	assertExtensionAuthoringInvocation(t, ctx, harness, "generation-two:alpha")
	assertExtensionAuthoringLogs(t, ctx, harness, "generation-two-observed")

	var removed compozycontract.ManagedExtensionRemovePayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&removed,
		"extension", "remove", extensionAuthoringE2EName,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if removed.Name != extensionAuthoringE2EName || removed.Status != "removed" || removed.Path != sourceDir {
		t.Fatalf("extension remove result = %#v, want removed dev link", removed)
	}
}

func testDaemonE2EResourceOnlyExtensionAuthoring(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath:   binaryPath,
		StartTimeout: 30 * time.Second,
	})
	sourceDir := filepath.Join(harness.WorkspaceRoot, resourceOnlyExtensionAuthoringE2EName)
	writeResourceOnlyAuthoringFixture(t, sourceDir, "generation one")

	var firstBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&firstBuild,
		"extension", "build", sourceDir, "-o", "json",
	)
	assertExtensionAuthoringValidation(t, ctx, harness, firstBuild.GenerationDir)
	if _, err := os.Stat(filepath.Join(
		firstBuild.GenerationDir,
		"agents",
		resourceOnlyAuthoringAgentName,
		"AGENT.md",
	)); err != nil {
		t.Fatalf("os.Stat(generated agent) error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(
		firstBuild.GenerationDir,
		"skills",
		resourceOnlyAuthoringSkillName,
		"SKILL.md",
	)); err != nil {
		t.Fatalf("os.Stat(generated skill) error = %v", err)
	}

	var linked compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&linked,
		"extension", "dev", sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if !linked.Dev || linked.WorkspaceID != harness.WorkspaceID || linked.GenerationHash != firstBuild.GenerationHash {
		t.Fatalf("resource-only extension dev result = %#v, want generation %q", linked, firstBuild.GenerationHash)
	}
	assertResourceOnlyAuthoringAgent(t, ctx, harness, "generation one")
	assertResourceOnlyAuthoringSkill(t, ctx, harness)
	assertResourceOnlyAuthoringAgentIsWorkspaceScoped(t, ctx, harness)
	assertResourceOnlyAuthoringSkillIsWorkspaceScoped(t, ctx, harness)

	writeResourceOnlyAuthoringAgent(t, sourceDir, "generation two")
	var reloaded compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&reloaded,
		"extension", "reload", resourceOnlyExtensionAuthoringE2EName, sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if !reloaded.Dev || reloaded.GenerationHash == firstBuild.GenerationHash {
		t.Fatalf("resource-only extension reload result = %#v, want a new dev generation", reloaded)
	}
	assertResourceOnlyAuthoringAgent(t, ctx, harness, "generation two")

	writeInvalidResourceOnlyAuthoringAgent(t, sourceDir)
	stdout, stderr, err := harness.CLI.RunInDir(
		ctx,
		harness.WorkspaceRoot,
		"extension", "reload", resourceOnlyExtensionAuthoringE2EName, sourceDir,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if err == nil {
		t.Fatalf("invalid resource-only reload error = nil; stdout=%s stderr=%s", stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "" || !strings.Contains(stderr, "AGENT.md") ||
		!strings.Contains(stderr, "decode agent frontmatter") {
		t.Fatalf("invalid resource-only reload stdout=%q stderr=%q, want invalid AGENT.md diagnostic", stdout, stderr)
	}
	active := findWorkspaceExtension(t, ctx, harness, resourceOnlyExtensionAuthoringE2EName)
	if active.GenerationHash != reloaded.GenerationHash || !active.Dev {
		t.Fatalf("active extension after invalid edit = %#v, want generation %q", active, reloaded.GenerationHash)
	}
	assertResourceOnlyAuthoringAgent(t, ctx, harness, "generation two")

	var removed compozycontract.ManagedExtensionRemovePayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&removed,
		"extension", "remove", resourceOnlyExtensionAuthoringE2EName,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if removed.Status != "removed" || removed.Path != sourceDir {
		t.Fatalf("resource-only extension remove result = %#v, want removed source %q", removed, sourceDir)
	}
}

func writeResourceOnlyAuthoringFixture(t *testing.T, sourceDir, prompt string) {
	t.Helper()
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(resource-only source) error = %v", err)
	}
	manifest := fmt.Sprintf(`[extension]
name = %q
version = "0.1.0"
min_compozy_version = "0.3.0-beta.1"

[[resources.agents]]
path = "agents"

[[resources.skills]]
path = "skills"
`, resourceOnlyExtensionAuthoringE2EName)
	if err := os.WriteFile(filepath.Join(sourceDir, "extension.toml"), []byte(manifest), 0o600); err != nil {
		t.Fatalf("os.WriteFile(resource-only manifest) error = %v", err)
	}
	writeResourceOnlyAuthoringAgent(t, sourceDir, prompt)
	writeResourceOnlyAuthoringSkill(t, sourceDir)
}

func writeResourceOnlyAuthoringAgent(t *testing.T, sourceDir, prompt string) {
	t.Helper()
	agentDir := filepath.Join(sourceDir, "agents", resourceOnlyAuthoringAgentName)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(resource-only agent) error = %v", err)
	}
	content := fmt.Sprintf("---\nname: %s\n---\n%s\n", resourceOnlyAuthoringAgentName, prompt)
	if err := os.WriteFile(filepath.Join(agentDir, "AGENT.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(resource-only agent) error = %v", err)
	}
}

func writeInvalidResourceOnlyAuthoringAgent(t *testing.T, sourceDir string) {
	t.Helper()
	agentPath := filepath.Join(sourceDir, "agents", resourceOnlyAuthoringAgentName, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte("---\nname: [\n---\ninvalid\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(invalid resource-only agent) error = %v", err)
	}
}

func writeResourceOnlyAuthoringSkill(t *testing.T, sourceDir string) {
	t.Helper()
	skillDir := filepath.Join(sourceDir, "skills", resourceOnlyAuthoringSkillName)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(resource-only skill) error = %v", err)
	}
	content := fmt.Sprintf(`---
name: %s
description: Exercise resource-only extension authoring.
---

# Resource-only authoring
`, resourceOnlyAuthoringSkillName)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(resource-only skill) error = %v", err)
	}
}

func assertResourceOnlyAuthoringAgent(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	wantPrompt string,
) {
	t.Helper()
	response := listAuthoringAgents(t, ctx, harness, harness.WorkspaceRoot)
	for _, agent := range response.Agents {
		if agent.Name != resourceOnlyAuthoringAgentName {
			continue
		}
		if strings.TrimSpace(agent.Prompt) != wantPrompt || agent.WorkspaceID != harness.WorkspaceID {
			t.Fatalf(
				"resource-only agent = %#v, want prompt %q in workspace %q",
				agent,
				wantPrompt,
				harness.WorkspaceID,
			)
		}
		return
	}
	t.Fatalf("workspace agents = %#v, want %q", response.Agents, resourceOnlyAuthoringAgentName)
}

func assertResourceOnlyAuthoringAgentIsWorkspaceScoped(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	response := listAuthoringAgents(t, ctx, harness, "")
	for _, agent := range response.Agents {
		if agent.Name == resourceOnlyAuthoringAgentName {
			t.Fatalf("global agents contain workspace resource-only agent: %#v", agent)
		}
	}
}

func assertResourceOnlyAuthoringSkill(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	response := listAuthoringSkills(t, ctx, harness, harness.WorkspaceRoot)
	for _, skill := range response.Skills {
		if skill.Name == resourceOnlyAuthoringSkillName {
			return
		}
	}
	t.Fatalf("workspace skills = %#v, want %q", response.Skills, resourceOnlyAuthoringSkillName)
}

func assertResourceOnlyAuthoringSkillIsWorkspaceScoped(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	response := listAuthoringSkills(t, ctx, harness, "")
	for _, skill := range response.Skills {
		if skill.Name == resourceOnlyAuthoringSkillName {
			t.Fatalf("global skills contain workspace resource-only skill: %#v", skill)
		}
	}
}

func listAuthoringAgents(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspace string,
) compozycontract.AgentsResponse {
	t.Helper()
	path := "/api/agents"
	if workspace != "" {
		path += "?workspace=" + url.QueryEscape(workspace)
	}
	var response compozycontract.AgentsResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("list agents for workspace %q error = %v", workspace, err)
	}
	return response
}

func listAuthoringSkills(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	workspace string,
) compozycontract.SkillsResponse {
	t.Helper()
	path := "/api/skills"
	if workspace != "" {
		path += "?workspace=" + url.QueryEscape(workspace)
	}
	var response compozycontract.SkillsResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("list skills for workspace %q error = %v", workspace, err)
	}
	return response
}

func findWorkspaceExtension(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	name string,
) compozycontract.ExtensionPayload {
	t.Helper()
	var response compozycontract.ExtensionsResponse
	path := "/api/extensions?workspace=" + url.QueryEscape(harness.WorkspaceID)
	if err := harness.HTTPJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		t.Fatalf("list workspace extensions error = %v", err)
	}
	for _, extension := range response.Extensions {
		if extension.Name == name {
			return extension
		}
	}
	t.Fatalf("workspace extensions = %#v, want %q", response.Extensions, name)
	return compozycontract.ExtensionPayload{}
}

// quickstartGuidePath is the published page that owns the replayed command list. The fenced block
// between the markers below is the single source: this test never keeps a second copy of it.
const (
	quickstartGuidePath  = "packages/site/content/docs/guides/build-your-first-extension.mdx"
	quickstartBeginMark  = "{/* extension-quickstart:begin */}"
	quickstartEndMark    = "{/* extension-quickstart:end */}"
	quickstartExtension  = "hello"
	quickstartToolResult = "No results for compozy"
)

func TestDaemonE2EExtensionQuickstartReplayEndsWithAnInvocableExtension(t *testing.T) {
	t.Run(
		"Should replay the quickstart and invoke the installed extension",
		testDaemonE2EExtensionQuickstartReplayEndsWithAnInvocableExtension,
	)
}

func testDaemonE2EExtensionQuickstartReplayEndsWithAnInvocableExtension(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	repoRoot := extensionAuthoringE2ERepoRoot(t)
	commands := readQuickstartCommands(t, repoRoot)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath:   binaryPath,
		StartTimeout: 30 * time.Second,
	})
	lastStdout := ""
	for _, command := range commands {
		stdout, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, command...)
		if err != nil {
			t.Fatalf(
				"quickstart command %q error = %v; stdout=%s stderr=%s",
				strings.Join(command, " "),
				err,
				strings.TrimSpace(stdout),
				strings.TrimSpace(stderr),
			)
		}
		lastStdout = stdout
		// The published page assumes the SDK resolves from its registry. The test runs offline, so
		// the scaffolded module is pointed at this checkout right after the documented scaffold step.
		if len(command) >= 2 && command[0] == "extension" && command[1] == "init" {
			configureExtensionAuthoringSDKReplace(
				t,
				ctx,
				filepath.Join(harness.WorkspaceRoot, quickstartExtension),
				repoRoot,
			)
		}
	}
	if !strings.Contains(lastStdout, quickstartToolResult) {
		t.Fatalf("quickstart tool invocation output = %q, want %q", lastStdout, quickstartToolResult)
	}
	assertQuickstartExtensionInstalled(t, ctx, harness)
}

func assertQuickstartExtensionInstalled(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) {
	t.Helper()
	var response compozycontract.ExtensionsResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodGet,
		"/api/extensions?workspace="+url.QueryEscape(harness.WorkspaceID),
		nil,
		&response,
	); err != nil {
		t.Fatalf("list workspace-scoped extensions error = %v", err)
	}
	for _, item := range response.Extensions {
		if item.Name != quickstartExtension {
			continue
		}
		if !item.Enabled || !item.Dev || strings.TrimSpace(item.GenerationHash) == "" {
			t.Fatalf("quickstart extension record = %#v, want an enabled dev generation", item)
		}
		return
	}
	t.Fatalf("workspace extensions = %#v, want %q installed", response.Extensions, quickstartExtension)
}

// readQuickstartCommands parses the published quickstart's machine-readable block and returns each
// documented `compozy` invocation as argv. Any drift between the page and this replay is a failure
// here rather than a silently stale second command list.
func readQuickstartCommands(t *testing.T, repoRoot string) [][]string {
	t.Helper()
	guidePath := filepath.Join(repoRoot, filepath.FromSlash(quickstartGuidePath))
	data, err := os.ReadFile(guidePath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", guidePath, err)
	}
	body := string(data)
	_, after, found := strings.Cut(body, quickstartBeginMark)
	if !found {
		t.Fatalf("quickstart guide %q is missing marker %q", quickstartGuidePath, quickstartBeginMark)
	}
	block, _, found := strings.Cut(after, quickstartEndMark)
	if !found {
		t.Fatalf("quickstart guide %q is missing marker %q", quickstartGuidePath, quickstartEndMark)
	}
	fenced := quickstartFencedLines(t, block)
	commands := make([][]string, 0, len(fenced))
	for _, line := range fenced {
		argv, err := splitQuickstartCommand(line)
		if err != nil {
			t.Fatalf("quickstart command %q is not replayable: %v", line, err)
		}
		if len(argv) < 2 || argv[0] != "compozy" {
			t.Fatalf("quickstart command %q must invoke the compozy binary", line)
		}
		commands = append(commands, argv[1:])
	}
	if len(commands) == 0 {
		t.Fatalf("quickstart guide %q declares no replayable commands", quickstartGuidePath)
	}
	return commands
}

func quickstartFencedLines(t *testing.T, block string) []string {
	t.Helper()
	_, after, found := strings.Cut(block, "```bash\n")
	if !found {
		t.Fatal("quickstart block is missing a bash code fence")
	}
	fence, _, found := strings.Cut(after, "```")
	if !found {
		t.Fatal("quickstart bash code fence is unterminated")
	}
	lines := make([]string, 0, 4)
	for _, raw := range strings.Split(fence, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

// splitQuickstartCommand splits one documented shell line into argv, honoring the single and double
// quoting the page uses. Any other shell metacharacter means the line is not verbatim-replayable.
func splitQuickstartCommand(line string) ([]string, error) {
	argv := make([]string, 0, 6)
	var current strings.Builder
	quote := rune(0)
	pending := false
	for _, character := range line {
		switch {
		case quote != 0 && character == quote:
			quote = 0
		case quote != 0:
			current.WriteRune(character)
		case character == '\'' || character == '"':
			quote = character
			pending = true
		case character == ' ' || character == '\t':
			if current.Len() > 0 || pending {
				argv = append(argv, current.String())
				current.Reset()
				pending = false
			}
		case strings.ContainsRune("|&;<>$`\\*?()", character):
			return nil, fmt.Errorf("unsupported shell metacharacter %q", character)
		default:
			current.WriteRune(character)
		}
	}
	if quote != 0 {
		return nil, errors.New("unterminated quote")
	}
	if current.Len() > 0 || pending {
		argv = append(argv, current.String())
	}
	return argv, nil
}

func runExtensionAuthoringCLI(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	destination any,
	args ...string,
) {
	t.Helper()
	if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, destination, args...); err != nil {
		t.Fatalf("CLI %q error = %v", strings.Join(args, " "), err)
	}
}

func assertExtensionAuthoringValidation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	generationDir string,
) {
	t.Helper()
	var report extensionpkg.ValidationReport
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&report,
		"extension", "validate", generationDir, "-o", "json",
	)
	if len(report.Issues) != 0 {
		t.Fatalf("extension validation issues = %#v, want none", report.Issues)
	}
}

func assertExtensionAuthoringInvocation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	toolID, err := toolspkg.CanonicalToolID("ext", extensionAuthoringE2EName, "search")
	if err != nil {
		t.Fatalf("CanonicalToolID() error = %v", err)
	}
	var response compozycontract.ToolInvokeResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		"/api/tools/"+url.PathEscape(string(toolID))+"/invoke",
		compozycontract.ToolInvokeRequest{
			WorkspaceID: harness.WorkspaceID,
			Input:       json.RawMessage(`{"query":"alpha"}`),
		},
		&response,
	); err != nil {
		t.Fatalf("invoke extension tool %q error = %v", toolID, err)
	}
	if len(response.Result.Content) != 1 || response.Result.Content[0].Text != want {
		t.Fatalf("invoke extension result = %#v, want %q", response.Result.Content, want)
	}
}

func assertExtensionAuthoringLogs(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	var snapshot compozycontract.ExtensionLogsResponse
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&snapshot,
		"extension", "logs", extensionAuthoringE2EName,
		"--workspace", harness.WorkspaceRoot,
		"-o", "json",
	)
	if strings.TrimSpace(snapshot.StreamEpoch) == "" {
		t.Fatal("extension logs stream_epoch is empty")
	}
	for _, entry := range snapshot.Logs {
		if entry.StreamEpoch != snapshot.StreamEpoch {
			t.Fatalf(
				"extension log stream_epoch = %q, want snapshot epoch %q",
				entry.StreamEpoch,
				snapshot.StreamEpoch,
			)
		}
		if strings.Contains(entry.Message, want) {
			return
		}
	}
	t.Fatalf("extension logs = %#v, want message containing %q", snapshot.Logs, want)
}

func rewriteExtensionAuthoringGeneration(
	t *testing.T,
	sourceDir string,
	oldResult string,
	newResult string,
	logMarker string,
) {
	t.Helper()
	mainPath := filepath.Join(sourceDir, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", mainPath, err)
	}
	content := string(data)
	content = strings.Replace(content, oldResult, newResult, 1)
	needle := "return compozysdk.TextResult("
	if !strings.Contains(content, needle) {
		t.Fatalf("scaffold source %q is missing tool result handler", mainPath)
	}
	content = strings.Replace(
		content,
		needle,
		fmt.Sprintf("fmt.Fprintln(os.Stderr, %q)\n\t\t\t%s", logMarker, needle),
		1,
	)
	if err := os.WriteFile(mainPath, []byte(content), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", mainPath, err)
	}
}

func configureExtensionAuthoringSDKReplace(
	t *testing.T,
	ctx context.Context,
	sourceDir string,
	repoRoot string,
) {
	t.Helper()
	// #nosec G204 -- the isolated fixture intentionally points its SDK dependency at this checkout.
	command := execabs.CommandContext(
		ctx,
		"go",
		"mod",
		"edit",
		"-replace",
		"github.com/compozy/compozy/sdk/go="+filepath.Join(repoRoot, "sdk", "go"),
	)
	command.Dir = sourceDir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("go mod edit SDK replace error = %v; output=%s", err, strings.TrimSpace(string(output)))
	}
}

func buildStampedExtensionAuthoringBinary(t *testing.T, ctx context.Context, repoRoot string) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "compozy")
	// #nosec G204 -- the test builds the current checkout with an explicit release-compatible version stamp.
	command := execabs.CommandContext(
		ctx,
		"go",
		"build",
		"-ldflags",
		"-X github.com/compozy/compozy/internal/version.Version=v0.3.0-beta.1",
		"-o",
		binaryPath,
		"./cmd/compozy",
	)
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build stamped compozy binary error = %v; output=%s", err, strings.TrimSpace(string(output)))
	}
	return binaryPath
}

func extensionAuthoringE2ERepoRoot(t *testing.T) string {
	t.Helper()
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("filepath.Abs(repo root) error = %v", err)
	}
	return repoRoot
}
