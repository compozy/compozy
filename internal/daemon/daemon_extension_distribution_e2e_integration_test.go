//go:build integration && !windows

package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestDaemonE2EExtensionDistributionAcrossIsolatedHomes(t *testing.T) {
	t.Run(
		"Should publish install update and remove across isolated homes",
		testDaemonE2EExtensionDistributionAcrossIsolatedHomes,
	)
}

func testDaemonE2EExtensionDistributionAcrossIsolatedHomes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	const publishCredential = "distribution-e2e-publish-token"
	githubServer := newDistributionGitHubServer(t, publishCredential)
	t.Cleanup(githubServer.Close)
	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)
	configSeed := e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
		cfg.Extensions.Trust.AllowUnverified = true
		cfg.Extensions.Sources.GitHub.Enabled = true
		cfg.Extensions.Sources.GitHub.BaseURL = githubServer.URL
		cfg.Tools.Policy.TrustedSources = append(cfg.Tools.Policy.TrustedSources, "extension:hello")
	}}
	publisher := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		ConfigSeed: configSeed,
		Env:        map[string]string{"GITHUB_TOKEN": publishCredential},
	})

	sourceDir := filepath.Join(publisher.WorkspaceRoot, "hello")
	var scaffold extensionpkg.ScaffoldResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		publisher,
		&scaffold,
		"extension", "init", "hello", "--template", "tool-provider-go", "--dir", sourceDir, "-o", "json",
	)
	configureExtensionAuthoringSDKReplaceWithoutShell(t, sourceDir, repoRoot)
	rewriteExtensionAuthoringGeneration(t, sourceDir, "No results for ", "published-v1:", "published-v1")
	configureExtensionKitE2ESource(t, sourceDir)
	var firstBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(t, ctx, publisher, &firstBuild, "extension", "build", sourceDir, "-o", "json")
	firstNetworkDigest := addExtensionKitE2ENetworkRequirement(t, firstBuild.GenerationDir, "builders")
	var firstPublish extensionpkg.PublishResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		publisher,
		&firstPublish,
		"extension", "publish", firstBuild.GenerationDir,
		"--repository", "acme/hello", "--tag", "v0.1.0", "-o", "json",
	)
	if firstPublish.DigestSHA256 == "" || firstPublish.ReleaseURL == "" || firstPublish.AssetURL == "" {
		t.Fatalf("first publish result = %#v, want release, asset, and digest", firstPublish)
	}

	consumer := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		ConfigSeed: configSeed,
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "hosted_native_tools_fixture.json"),
			FixtureAgent: "hosted-native",
			AgentName:    "mock-installed-extension",
		}},
	})
	var installed compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		consumer,
		&installed,
		"extension", "install", "github:acme/hello", "--allow-unverified", "--yes", "-o", "json",
	)
	if installed.Name != "hello" || installed.Provenance == nil || !installed.Provenance.DigestMatched ||
		installed.Provenance.ChecksumVerified || installed.Provenance.InstalledFrom != extensionpkg.ExtensionInstalledFromGitHub {
		t.Fatalf("installed extension = %#v, want digest-verified integrity-only GitHub provenance", installed)
	}
	if installed.Enabled {
		t.Fatalf("installed extension = %#v, want inert before explicit enable", installed)
	}
	setExtensionKitE2ESecret(t, ctx, consumer, "hello")
	var enabled compozycontract.ExtensionEnableResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		consumer,
		&enabled,
		"extension", "enable", "hello", "--confirm-network-requirement", firstNetworkDigest, "-o", "json",
	)
	if !enabled.Extension.Enabled || len(enabled.AutomationStarted) != 2 {
		t.Fatalf("extension enable result = %#v, want enabled extension and started job plus trigger", enabled)
	}
	assertDistributionExtensionInventory(t, ctx, consumer, "hello", true, "daily", true)
	assertDistributionExtensionInvocation(t, ctx, consumer, "published-v1:alpha")
	assertDistributionHostedExtensionInvocation(t, ctx, consumer, "published-v1:alpha")

	writeExtensionKitE2EAutomation(t, sourceDir, "hourly")
	rewriteExtensionAuthoringGeneration(t, sourceDir, "published-v1:", "published-v2:", "published-v2")
	var secondBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(t, ctx, publisher, &secondBuild, "extension", "build", sourceDir, "-o", "json")
	secondNetworkDigest := addExtensionKitE2ENetworkRequirement(t, secondBuild.GenerationDir, "reviewers")
	var secondPublish extensionpkg.PublishResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		publisher,
		&secondPublish,
		"extension", "publish", secondBuild.GenerationDir,
		"--repository", "acme/hello", "--tag", "v0.2.0", "-o", "json",
	)
	if secondPublish.DigestSHA256 == firstPublish.DigestSHA256 {
		t.Fatalf("second publish digest = %q, want changed generation digest", secondPublish.DigestSHA256)
	}

	stdout, stderr, updateErr := consumer.CLI.RunInDir(
		ctx,
		consumer.WorkspaceRoot,
		"extension", "update", "hello", "--allow-unverified", "--yes", "-o", "json",
	)
	if updateErr == nil || !strings.Contains(stderr, secondNetworkDigest) {
		t.Fatalf(
			"unconfirmed extension update error = %v; stdout=%s stderr=%s, want refusal with candidate digest %q",
			updateErr,
			stdout,
			stderr,
			secondNetworkDigest,
		)
	}
	unchanged, err := consumer.GetExtension(ctx, "hello")
	if err != nil || unchanged.Version != "0.1.0" {
		t.Fatalf("extension after refused update = %#v, error = %v, want original manifest version", unchanged, err)
	}

	var updates []compozycontract.ManagedExtensionUpdatePayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		consumer,
		&updates,
		"extension", "update", "hello", "--allow-unverified", "--yes",
		"--confirm-network-requirement", secondNetworkDigest, "-o", "json",
	)
	if len(updates) != 1 || updates[0].Status != extensionpkg.MarketplaceUpdateStatusUpdated ||
		updates[0].LatestVersion != "v0.2.0" {
		t.Fatalf("extension update result = %#v, want one update to v0.2.0", updates)
	}
	assertDistributionExtensionInventory(t, ctx, consumer, "hello", true, "hourly", true)
	assertDistributionExtensionInvocation(t, ctx, consumer, "published-v2:alpha")

	var disabled compozycontract.ExtensionPayload
	runExtensionAuthoringCLI(t, ctx, consumer, &disabled, "extension", "disable", "hello", "-o", "json")
	if disabled.Enabled {
		t.Fatalf("extension disable result = %#v, want disabled", disabled)
	}
	assertDistributionExtensionInventory(t, ctx, consumer, "hello", false, "hourly", false)
	assertDistributionAutomationRemoved(t, ctx, consumer, "hello")

	assertDistributionNativeKitJourney(t, ctx, binaryPath, configSeed, secondNetworkDigest)

	var removed compozycontract.ManagedExtensionRemovePayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		consumer,
		&removed,
		"extension", "remove", "hello", "--global", "-o", "json",
	)
	if removed.Name != "hello" || removed.Status != "removed" {
		t.Fatalf("extension remove result = %#v, want removed hello", removed)
	}
	if _, err := consumer.GetExtension(ctx, "hello"); err == nil {
		t.Fatal("GetExtension(after remove) error = nil, want not found")
	}
	githubServer.requireReleaseCount(t, 2)
}

func assertDistributionHostedExtensionInvocation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	registration, ok := harness.MockAgentRegistration("mock-installed-extension")
	if !ok {
		t.Fatal("MockAgentRegistration(mock-installed-extension) = missing")
	}
	active := createBoundFixtureBackedSession(
		t,
		ctx,
		harness,
		"mock-installed-extension",
		"installed-extension-hosted-mcp",
	)
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(mock-installed-extension) error = %v", err)
	}
	sessionDiagnostics := acpmock.DiagnosticsForCompozySession(diagnostics, active.ID)
	client := startHostedMCPClient(
		t,
		ctx,
		requireHostedMCPStdioServer(t, sessionDiagnostics, hostedMCPServerLatest),
	)
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("Close(installed extension hosted MCP client) error = %v", err)
		}
	}()

	toolID, err := toolspkg.CanonicalToolID("ext", "hello", "search")
	if err != nil {
		t.Fatalf("CanonicalToolID(hello search) error = %v", err)
	}
	listed, err := client.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools(installed extension hosted MCP) error = %v", err)
	}
	if !sdkToolListContains(listed.Tools, toolID.String()) {
		t.Fatalf("hosted MCP tools = %#v, want installed extension tool %s", sdkToolNames(listed.Tools), toolID)
	}

	result, err := client.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      toolID.String(),
		Arguments: map[string]any{"query": "alpha"},
	})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", toolID, err)
	}
	if result == nil || result.IsError || len(result.Content) != 1 {
		t.Fatalf("CallTool(%s) result = %#v, want one successful text result", toolID, result)
	}
	textContent, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok || textContent.Text != want {
		t.Fatalf("CallTool(%s) content = %#v, want %q", toolID, result.Content, want)
	}
}

func assertDistributionExtensionInvocation(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want string,
) {
	t.Helper()
	toolID, err := toolspkg.CanonicalToolID("ext", "hello", "search")
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
		t.Fatalf("extension result = %#v, want %q", response.Result.Content, want)
	}
}

func assertDistributionExtensionInventory(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	extensionName string,
	wantEnabled bool,
	wantJobName string,
	wantLive bool,
) {
	t.Helper()
	var inventory compozycontract.ExtensionInventoryPayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		harness,
		&inventory,
		"extension", "inventory", extensionName, "-o", "json",
	)
	if inventory.Extension != extensionName || inventory.Enabled != wantEnabled || len(inventory.Items) < 7 {
		t.Fatalf(
			"extension inventory = %#v, want %q enabled=%t with the full shipped kit",
			inventory,
			extensionName,
			wantEnabled,
		)
	}
	wantKinds := map[string]bool{
		"skill": false, "loop": false, "agent": false, "automation.job": false,
		"automation.trigger": false, "window_layout": false, "tool": false,
	}
	foundJob := false
	for _, item := range inventory.Items {
		if item.Live != wantLive {
			t.Fatalf("inventory item = %#v, want live=%t", item, wantLive)
		}
		if _, tracked := wantKinds[string(item.Kind)]; tracked {
			wantKinds[string(item.Kind)] = true
		}
		if string(item.Kind) == "automation.job" && strings.Contains(item.Name, wantJobName) {
			foundJob = true
		}
	}
	for kind, found := range wantKinds {
		if !found {
			t.Fatalf("extension inventory = %#v, want shipped kind %q", inventory.Items, kind)
		}
	}
	if !foundJob {
		t.Fatalf("extension inventory = %#v, want job containing %q", inventory.Items, wantJobName)
	}
}

func assertDistributionAutomationRemoved(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	extensionName string,
) {
	t.Helper()
	var jobs compozycontract.JobsResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/automation/jobs", nil, &jobs); err != nil {
		t.Fatalf("list automation jobs after disable error = %v", err)
	}
	for _, job := range jobs.Jobs {
		if strings.Contains(job.ID, "extension/"+extensionName+"/") {
			t.Fatalf("automation job remained live after extension disable: %#v", job)
		}
	}
	var triggers compozycontract.TriggersResponse
	if err := harness.HTTPJSON(ctx, http.MethodGet, "/api/automation/triggers", nil, &triggers); err != nil {
		t.Fatalf("list automation triggers after disable error = %v", err)
	}
	for _, trigger := range triggers.Triggers {
		if strings.Contains(trigger.ID, "extension/"+extensionName+"/") {
			t.Fatalf("automation trigger remained live after extension disable: %#v", trigger)
		}
	}
}

func assertDistributionNativeKitJourney(
	t *testing.T,
	ctx context.Context,
	binaryPath string,
	configSeed e2etest.ConfigSeedOptions,
	networkDigest string,
) {
	t.Helper()
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		BinaryPath: binaryPath,
		ConfigSeed: configSeed,
	})
	transcript := make([]json.RawMessage, 0, 4)
	transcript = append(transcript, invokeDistributionNativeTool(
		t,
		ctx,
		harness,
		toolspkg.ToolIDExtensionsInstall,
		map[string]any{"source": "github", "ref": "acme/hello", "allow_unverified": true},
	))
	setExtensionKitE2ESecret(t, ctx, harness, "hello")
	preview := invokeDistributionNativeTool(
		t,
		ctx,
		harness,
		toolspkg.ToolIDExtensionsPreview,
		map[string]any{"name": "hello"},
	)
	if !strings.Contains(string(preview), `"network_confirmation_required":true`) ||
		!strings.Contains(string(preview), networkDigest) ||
		!strings.Contains(string(preview), `"change":"added"`) {
		t.Fatalf("extensions_preview structured output = %s, want candidate digest and added kit resources", preview)
	}
	transcript = append(transcript, preview)
	transcript = append(transcript, invokeDistributionNativeTool(
		t,
		ctx,
		harness,
		toolspkg.ToolIDExtensionsEnable,
		map[string]any{"name": "hello", "confirm_network_digest": networkDigest},
	))
	inventory := invokeDistributionNativeTool(
		t,
		ctx,
		harness,
		toolspkg.ToolIDExtensionsInventory,
		map[string]any{"name": "hello"},
	)
	if !strings.Contains(string(inventory), `"enabled":true`) ||
		!strings.Contains(string(inventory), `"kind":"automation.job"`) ||
		!strings.Contains(string(inventory), `"live":true`) {
		t.Fatalf("extensions_inventory structured output = %s, want live kit inventory", inventory)
	}
	transcript = append(transcript, inventory)
	for _, output := range transcript {
		if strings.Contains(string(output), extensionKitE2ESecret) {
			t.Fatalf("native extension tool transcript leaked the operator secret: %s", output)
		}
	}
	invokeDistributionNativeTool(
		t,
		ctx,
		harness,
		toolspkg.ToolIDExtensionsDisable,
		map[string]any{"name": "hello"},
	)
}

func invokeDistributionNativeTool(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	toolID toolspkg.ToolID,
	input any,
) json.RawMessage {
	t.Helper()
	rawInput, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("json.Marshal(%s input) error = %v", toolID, err)
	}
	var response compozycontract.ToolInvokeResponse
	if err := harness.HTTPJSON(
		ctx,
		http.MethodPost,
		"/api/tools/"+url.PathEscape(string(toolID))+"/invoke",
		compozycontract.ToolInvokeRequest{WorkspaceID: harness.WorkspaceID, Input: rawInput},
		&response,
	); err != nil {
		t.Fatalf("invoke native tool %q error = %v", toolID, err)
	}
	if !json.Valid(response.Result.Structured) || response.Result.Preview == "" {
		t.Fatalf("native tool %q result = %#v, want structured JSON and preview", toolID, response.Result)
	}
	return append(json.RawMessage(nil), response.Result.Structured...)
}

func configureExtensionAuthoringSDKReplaceWithoutShell(t *testing.T, sourceDir, repoRoot string) {
	t.Helper()
	goModPath := filepath.Join(sourceDir, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", goModPath, err)
	}
	replace := "\nreplace github.com/compozy/compozy/sdk/go => " + filepath.Join(repoRoot, "sdk", "go") + "\n"
	if err := os.WriteFile(goModPath, append(data, replace...), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", goModPath, err)
	}
}

type distributionGitHubServer struct {
	*httptest.Server
	credential  string
	mu          sync.Mutex
	nextRelease int64
	nextAsset   int64
	releases    []*distributionGitHubRelease
	assets      map[int64]distributionGitHubAsset
}

type distributionGitHubRelease struct {
	ID         int64                     `json:"id"`
	Name       string                    `json:"name"`
	TagName    string                    `json:"tag_name"`
	Draft      bool                      `json:"draft"`
	Prerelease bool                      `json:"prerelease"`
	HTMLURL    string                    `json:"html_url"`
	UploadURL  string                    `json:"upload_url"`
	Author     map[string]string         `json:"author"`
	Assets     []distributionGitHubAsset `json:"assets"`
}

type distributionGitHubAsset struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
	Size               int64  `json:"size"`
	payload            []byte
}

func newDistributionGitHubServer(t *testing.T, credential string) *distributionGitHubServer {
	t.Helper()
	fixture := &distributionGitHubServer{
		credential: credential,
		assets:     make(map[int64]distributionGitHubAsset),
	}
	fixture.Server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		fixture.handle(t, writer, request)
	}))
	return fixture
}

func (s *distributionGitHubServer) handle(t *testing.T, writer http.ResponseWriter, request *http.Request) {
	t.Helper()
	if request.Method != http.MethodGet && request.Header.Get("Authorization") != "Bearer "+s.credential {
		http.Error(writer, "unauthorized", http.StatusUnauthorized)
		return
	}
	path := request.URL.Path
	switch {
	case request.Method == http.MethodGet && path == "/repos/acme/hello/releases/latest":
		s.writeLatest(t, writer)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/repos/acme/hello/releases/tags/"):
		s.writeTag(t, writer, strings.TrimPrefix(path, "/repos/acme/hello/releases/tags/"))
	case request.Method == http.MethodGet && path == "/repos/acme/hello/releases":
		s.writeReleases(t, writer)
	case request.Method == http.MethodPost && path == "/repos/acme/hello/releases":
		s.createRelease(t, writer, request)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/uploads/"):
		s.uploadAsset(t, writer, request)
	case request.Method == http.MethodGet && strings.HasPrefix(path, "/assets/"):
		s.downloadAsset(t, writer, path)
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/repos/acme/hello/releases/assets/"):
		s.deleteAsset(writer, path)
	default:
		http.NotFound(writer, request)
	}
}

func (s *distributionGitHubServer) writeLatest(t *testing.T, writer http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.releases) == 0 {
		http.Error(writer, "release not found", http.StatusNotFound)
		return
	}
	writeDistributionGitHubJSON(t, writer, s.releases[0], http.StatusOK)
}

func (s *distributionGitHubServer) writeTag(t *testing.T, writer http.ResponseWriter, escapedTag string) {
	tag, err := url.PathUnescape(escapedTag)
	if err != nil {
		http.Error(writer, "invalid tag", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, release := range s.releases {
		if release.TagName == tag {
			writeDistributionGitHubJSON(t, writer, release, http.StatusOK)
			return
		}
	}
	http.Error(writer, "release not found", http.StatusNotFound)
}

func (s *distributionGitHubServer) writeReleases(t *testing.T, writer http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeDistributionGitHubJSON(t, writer, s.releases, http.StatusOK)
}

func (s *distributionGitHubServer) createRelease(
	t *testing.T,
	writer http.ResponseWriter,
	request *http.Request,
) {
	var input struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Draft   bool   `json:"draft"`
	}
	if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
		http.Error(writer, "invalid release", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextRelease++
	release := &distributionGitHubRelease{
		ID: s.nextRelease, Name: input.Name, TagName: input.TagName, Draft: input.Draft,
		HTMLURL:   s.URL + "/releases/" + url.PathEscape(input.TagName),
		UploadURL: fmt.Sprintf("%s/uploads/%d/assets{?name,label}", s.URL, s.nextRelease),
		Author:    map[string]string{"login": "acme"},
		Assets:    []distributionGitHubAsset{},
	}
	s.releases = append([]*distributionGitHubRelease{release}, s.releases...)
	writeDistributionGitHubJSON(t, writer, release, http.StatusCreated)
}

func (s *distributionGitHubServer) uploadAsset(
	t *testing.T,
	writer http.ResponseWriter,
	request *http.Request,
) {
	parts := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.Error(writer, "invalid upload path", http.StatusBadRequest)
		return
	}
	releaseID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		http.Error(writer, "invalid release id", http.StatusBadRequest)
		return
	}
	payload, err := io.ReadAll(request.Body)
	if err != nil {
		http.Error(writer, "read asset", http.StatusInternalServerError)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var release *distributionGitHubRelease
	for _, candidate := range s.releases {
		if candidate.ID == releaseID {
			release = candidate
			break
		}
	}
	if release == nil {
		http.NotFound(writer, request)
		return
	}
	s.nextAsset++
	asset := distributionGitHubAsset{
		ID: s.nextAsset, Name: request.URL.Query().Get("name"),
		URL:                s.URL + "/assets/" + strconv.FormatInt(s.nextAsset, 10),
		BrowserDownloadURL: s.URL + "/assets/" + strconv.FormatInt(s.nextAsset, 10),
		ContentType:        request.Header.Get("Content-Type"), Size: int64(len(payload)), payload: payload,
	}
	release.Assets = append(release.Assets, asset)
	s.assets[asset.ID] = asset
	writeDistributionGitHubJSON(t, writer, asset, http.StatusCreated)
}

func (s *distributionGitHubServer) downloadAsset(t *testing.T, writer http.ResponseWriter, path string) {
	id, err := strconv.ParseInt(strings.TrimPrefix(path, "/assets/"), 10, 64)
	if err != nil {
		http.Error(writer, "invalid asset id", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	asset, ok := s.assets[id]
	s.mu.Unlock()
	if !ok {
		http.Error(writer, "asset not found", http.StatusNotFound)
		return
	}
	writer.Header().Set("Content-Type", asset.ContentType)
	writer.WriteHeader(http.StatusOK)
	if _, err := writer.Write(asset.payload); err != nil {
		t.Errorf("write asset response error = %v", err)
	}
}

func (s *distributionGitHubServer) deleteAsset(writer http.ResponseWriter, path string) {
	id, err := strconv.ParseInt(strings.TrimPrefix(path, "/repos/acme/hello/releases/assets/"), 10, 64)
	if err != nil {
		http.Error(writer, "invalid asset id", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.assets, id)
	for _, release := range s.releases {
		filtered := release.Assets[:0]
		for _, asset := range release.Assets {
			if asset.ID != id {
				filtered = append(filtered, asset)
			}
		}
		release.Assets = filtered
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (s *distributionGitHubServer) requireReleaseCount(t *testing.T, want int) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.releases) != want {
		t.Fatalf("mock GitHub releases = %d, want %d", len(s.releases), want)
	}
}

func writeDistributionGitHubJSON(t *testing.T, writer http.ResponseWriter, value any, status int) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Errorf("json.Encode(mock GitHub response) error = %v", err)
	}
}
