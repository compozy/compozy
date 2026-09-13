//go:build integration && !windows

package daemon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensioninput"
	marketplacepkg "github.com/compozy/compozy/internal/marketplace"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	"github.com/kballard/go-shellquote"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/sys/execabs"
)

func TestDaemonE2EExtensionDistributionAcrossIsolatedHomes(t *testing.T) {
	t.Run("Should isolate manual and extension OAuth through public transports [IT-021]", testDaemonExtensionMCPOwners)
	t.Run("Should consume real v3 publication and reject root-only sources [IT-017]", testDaemonCatalogPublication)
	t.Run("Should join curated installs update releases and reject changed artifacts [IT-003 IT-004]",
		testDaemonCuratedCatalogLifecycle)
	t.Run("Should install checked-in required and optional secret inputs through public transports [IT-005]",
		testDaemonCatalogSecretInputs)
	t.Run("Should restore typed inputs through public transports after daemon restart [IT-020]",
		testDaemonExtensionInputsRestart)

	t.Run(
		"Should publish install update and remove across isolated homes",
		testDaemonE2EExtensionDistributionAcrossIsolatedHomes,
	)
	t.Run(
		"Should install a declared profile and clear its setup requirement [E2E-008]",
		testDaemonE2EExtensionDeclaredProfileSetup,
	)
}

// Invariant: the daemon reads the publisher's complete v3 family and reports root-only source failure without fallback.
// Owner: daemon distribution integration; canonical suite: TestDaemonE2EExtensionDistributionAcrossIsolatedHomes.
func testDaemonCatalogPublication(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()
	root := extensionAuthoringE2ERepoRoot(t)
	published := filepath.Join(t.TempDir(), "published")
	command := execabs.CommandContext(ctx, "go", "run", "./cmd/compozy-catalog", "publish",
		filepath.Join(root, "catalog"), published)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("publish catalog: %v\n%s", err, output)
	}
	raw, err := os.ReadFile(filepath.Join(published, "v3", "extensions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var feed struct {
		Entries []compozycontract.MarketplaceListingPayload `json:"entries"`
	}
	if err := json.Unmarshal(raw, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Entries) != 20 {
		t.Fatalf("published entries = %d, want 3 preserved extensions and 17 packaged servers", len(feed.Entries))
	}
	for _, tc := range []struct {
		name     string
		rootOnly bool
	}{
		{"Should list every published extension through HTTP and UDS", false},
		{"Should report a root-only source as failed without reading retired paths", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := published
			if tc.rootOnly {
				directory = t.TempDir()
				if err := os.WriteFile(filepath.Join(directory, "extensions.json"), raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var mu sync.Mutex
			requests := make(map[string]int)
			files := http.FileServer(http.Dir(directory))
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				requests[r.URL.Path]++
				mu.Unlock()
				files.ServeHTTP(w, r)
			}))
			t.Cleanup(server.Close)
			options := &e2etest.RuntimeHarnessOptions{
				ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
					cfg.Marketplace.Catalog.BaseURL = server.URL
				}},
			}
			runtime := e2etest.StartRuntimeHarness(t, options)
			var refresh compozycontract.MarketplaceRefreshResponse
			if err := runtime.HTTPJSON(
				t.Context(),
				http.MethodPost,
				"/api/marketplace/refresh",
				nil,
				&refresh,
			); err != nil {
				t.Fatal(err)
			}
			if len(refresh.Sources) != 1 {
				t.Fatalf("refresh sources = %#v", refresh.Sources)
			}
			outcome := refresh.Sources[0]
			if tc.rootOnly {
				if outcome.Outcome != "failed" || !outcome.Stale || outcome.ErrorClass == "" ||
					outcome.EntryCount != 0 {
					t.Fatalf("root-only refresh = %#v", outcome)
				}
			} else if outcome.Outcome != "succeeded" || outcome.Stale || outcome.EntryCount != len(feed.Entries) {
				t.Fatalf("published refresh = %#v", outcome)
			}
			if !tc.rootOnly {
				if err := runtime.Stop(t.Context()); err != nil {
					t.Fatal(err)
				}
				options.HomePaths, options.BinaryPath = runtime.HomePaths, runtime.BinaryPath
				options.Workspace.Root = runtime.WorkspaceRoot
				runtime = e2etest.StartRuntimeHarness(t, options)
			}
			for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
				var listing compozycontract.MarketplaceListResponse
				if err := read(t.Context(), http.MethodGet, "/api/marketplace?limit=100", nil, &listing); err != nil {
					t.Fatal(err)
				}
				if tc.rootOnly {
					if len(listing.Items) != 0 || !listing.Stale || listing.ErrorClass == "" {
						t.Fatalf("root-only listing = %#v", listing)
					}
					continue
				}
				if len(listing.Items) != len(feed.Entries) || listing.Stale {
					t.Fatalf("published listing count/stale = %d/%t", len(listing.Items), listing.Stale)
				}
				got := make(map[string]compozycontract.MarketplaceListingPayload, len(listing.Items))
				for _, item := range listing.Items {
					got[item.EntryID] = item
				}
				for _, expected := range feed.Entries {
					item, exists := got[expected.EntryID]
					if !exists || item.InstallSlug != "compozy/"+expected.EntryID || item.Version != expected.Version ||
						item.DigestSHA256 != expected.DigestSHA256 {
						t.Fatalf("published identity %s = %#v, want %#v", expected.EntryID, item, expected)
					}
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if requests["/v3/extensions.json"] == 0 {
				t.Fatal("daemon never requested the v3 publication")
			}
			for path := range requests {
				if path != "/v3/extensions.json" && path != "/v3/marketplaces.json" {
					t.Fatalf("daemon requested a non-v3 catalog path: %s", path)
				}
			}
		})
	}
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
		cfg.Marketplace.Catalog.BaseURL = githubServer.URL
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
		"extension", "install", "github:acme/hello", "--allow-unverified", "--yes",
		"--confirm-network-requirement", firstNetworkDigest, "-o", "json",
	)
	if installed.Name != "hello" || installed.Provenance == nil || !installed.Provenance.DigestMatched ||
		installed.Provenance.ChecksumVerified || installed.Provenance.InstalledFrom != extensionpkg.ExtensionInstalledFromGitHub {
		t.Fatalf("installed extension = %#v, want digest-verified integrity-only GitHub provenance", installed)
	}
	if !installed.Enabled {
		t.Fatalf("installed extension = %#v, want default-on enablement", installed)
	}
	setExtensionKitE2ESecret(t, ctx, consumer, "hello")
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

	var disabled compozycontract.ExtensionEnablementPayload
	runExtensionAuthoringCLI(t, ctx, consumer, &disabled, "extension", "disable", "hello", "-o", "json")
	if disabled.Enabled || disabled.Profile != "default" {
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
		map[string]any{
			"source": "github", "ref": "acme/hello", "allow_unverified": true,
			"confirm_network_digest": networkDigest,
		},
	))
	setExtensionKitE2ESecret(t, ctx, harness, "hello")
	transcript = append(transcript, invokeDistributionNativeTool(
		t,
		ctx,
		harness,
		toolspkg.ToolIDExtensionsEnable,
		map[string]any{"name": "hello"},
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
		if manifest, manifestErr := harness.RuntimeManifest(); manifestErr == nil {
			if processLog, readErr := os.ReadFile(manifest.Logs.ProcessLogFile); readErr == nil {
				t.Logf("native extension tool daemon process log:\n%s", processLog)
			} else {
				t.Logf("read native extension tool daemon process log error = %v", readErr)
			}
		} else {
			t.Logf("read native extension tool runtime manifest error = %v", manifestErr)
		}
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
	credential     string
	mu             sync.Mutex
	nextRelease    int64
	nextAsset      int64
	releases       []*distributionGitHubRelease
	assets         map[int64]distributionGitHubAsset
	catalogEntries map[string]map[string]any
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
		credential:     credential,
		assets:         make(map[int64]distributionGitHubAsset),
		catalogEntries: make(map[string]map[string]any),
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
	case request.Method == http.MethodGet && path == "/v3/extensions.json":
		s.mu.Lock()
		defer s.mu.Unlock()
		entries := make([]map[string]any, 0, len(s.catalogEntries))
		for _, entry := range s.catalogEntries {
			entries = append(entries, entry)
		}
		writeDistributionGitHubJSON(t, writer, map[string]any{
			"manifest_version": 3, "generated_at": "2026-09-13T00:00:00Z", "entries": entries,
		}, http.StatusOK)
	case request.Method == http.MethodGet && path == "/v3/marketplaces.json":
		writeDistributionGitHubJSON(t, writer, map[string]any{
			"manifest_version": 3,
			"generated_at":     "2026-08-17T00:00:00Z",
			"entries":          []any{},
		}, http.StatusOK)
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

// Invariant: real CLI installation persists typed values which HTTP/UDS publish again after a daemon restart.
// Owner: daemon distribution integration; canonical suite: TestDaemonE2EExtensionDistributionAcrossIsolatedHomes.
func testDaemonExtensionInputsRestart(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	defer cancel()
	catalog := newDistributionGitHubServer(t, "input-release-fixture")
	t.Cleanup(catalog.Close)
	options := &e2etest.RuntimeHarnessOptions{ConfigSeed: e2etest.ConfigSeedOptions{
		Mutate: func(cfg *compozyconfig.Config) {
			cfg.Extensions.Trust.AllowUnverified = true
			cfg.Marketplace.Catalog.BaseURL = catalog.URL + "/unavailable"
			cfg.Extensions.Sources.GitHub.Enabled = true
			cfg.Extensions.Sources.GitHub.BaseURL = catalog.URL
		},
	}, Env: map[string]string{"WORKSPACE_ID": "", "READ_ONLY": ""}}
	runtime := e2etest.StartRuntimeHarness(t, options)
	remote := mcpfixture.MustNew(mcpfixture.ProfileModern2026).StartHTTP(t)
	reportPath := filepath.Join(t.TempDir(), "input-probe.json")
	manifest := fmt.Sprintf(`name = "durable-input-kit"
version = "1.0.0"
description = "Typed input restart fixture"
min_compozy_version = "0.0.0"

[[profiles]]
name = "input-isolated"
icon = "circle"
color = "#5fbf85"

[[inputs]]
id = "workspace_id"
prompt = "Workspace"
type = "identifier"
required = true
binding = { type = "url_query", name = "workspace" }

[[inputs]]
id = "read_only"
prompt = "Read only"
type = "boolean"
required = true
binding = { type = "env", name = "READ_ONLY" }

[resources.mcp_servers.remote]
transport = "http"
url = %q
default_scope = "global"

[resources.mcp_servers.probe]
transport = "stdio"
command = %q
args = ["-test.run=^TestExtensionInputStdioHelperProcess$"]
env = { READ_ONLY = "read_only", COMPOZY_TEST_DAEMON_EXTENSION_HELPER = "1", COMPOZY_TEST_INPUT_REPORT = %q }
default_scope = "global"
`, remote.URL+"/mcp?workspace=", os.Args[0], reportPath)
	digest := catalog.setInputRelease(t, "1.0.0", manifest)
	request := compozycontract.InstallExtensionRequest{
		Source: compozycontract.InstallExtensionSourceGitHub, Ref: "acme/hello", ExpectedDigest: digest,
		Scope: "global", AllowUnverified: true,
	}
	for _, transport := range []struct {
		client *http.Client
		url    string
	}{
		{runtime.HTTPClient, runtime.HTTPURL("/api/extensions")},
		{runtime.UDSClient, runtime.UDSURL("/api/extensions")},
	} {
		assertDistributionMissingInputs(t, ctx, transport.client, transport.url, request)
	}
	var installed compozycontract.ExtensionPayload
	if err := runtime.CLI.RunJSON(ctx, &installed, "extension", "install", "github:acme/hello",
		"--scope", "global", "--allow-unverified", "--yes", "--input", "workspace_id=team-a",
		"--input", "read_only=false", "-o", "json"); err != nil {
		t.Fatal(err)
	}
	if installed.Name != "durable-input-kit" || len(installed.MissingInputs) != 0 {
		t.Fatalf("installed input readiness = %#v", installed)
	}
	assertDistributionUnconfiguredProfile(t, ctx, runtime)
	firstPID := assertDistributionPublishedInputs(t, ctx, runtime, reportPath)
	if err := runtime.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	options.HomePaths, options.BinaryPath = runtime.HomePaths, runtime.BinaryPath
	options.Workspace.Root = runtime.WorkspaceRoot
	runtime = e2etest.StartRuntimeHarness(t, options)
	assertDistributionUnconfiguredProfile(t, ctx, runtime)
	secondPID := assertDistributionPublishedInputs(t, ctx, runtime, reportPath)
	if secondPID == firstPID {
		t.Fatal("restart did not launch a new MCP process from persisted inputs")
	}
	assertDistributionInputUpdates(t, ctx, runtime, catalog, manifest, reportPath)
}

func assertDistributionMissingInputs(
	t *testing.T,
	ctx context.Context,
	client *http.Client,
	target string,
	install compozycontract.InstallExtensionRequest,
) {
	t.Helper()
	body := requestDistributionInstall(t, ctx, client, target, install, http.StatusUnprocessableEntity)
	var payload compozycontract.ExtensionOperationErrorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != "extension_inputs_required" || len(payload.Inputs) != 2 || len(payload.InputDefinitions) != 2 {
		t.Fatalf("input refusal payload=%#v", payload)
	}
}

func assertDistributionPublishedInputs(
	t *testing.T,
	ctx context.Context,
	runtime *e2etest.RuntimeHarness,
	reportPath string,
) int {
	t.Helper()
	for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
		var inventory struct {
			Extensions []compozycontract.ExtensionPayload `json:"extensions"`
		}
		if err := read(ctx, http.MethodGet, "/api/extensions", nil, &inventory); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, extension := range inventory.Extensions {
			if extension.Name == "durable-input-kit" {
				found = true
				if extension.Provenance == nil ||
					extension.Provenance.InstalledFrom != extensionpkg.ExtensionInstalledFromGitHub {
					t.Fatalf("update changed acquisition origin: %#v", extension.Provenance)
				}
				activeInputs := 0
				for _, input := range extension.Inputs {
					if input.Active && input.Set {
						activeInputs++
					}
				}
				if len(extension.MissingInputs) != 0 || len(extension.MissingEnv) != 0 || activeInputs != 2 {
					t.Fatalf("reopened input readiness = %#v", extension)
				}
			}
		}
		if !found {
			t.Fatal("installed input fixture missing from inventory")
		}
		var servers compozycontract.SettingsMCPServersResponse
		if err := read(ctx, http.MethodGet, "/api/settings/mcp-servers", nil, &servers); err != nil {
			t.Fatal(err)
		}
		foundRemote, foundProbe := false, false
		for _, server := range servers.MCPServers {
			if server.Owner != "extension:durable-input-kit" {
				continue
			}
			switch server.Name {
			case "remote":
				parsed, err := url.Parse(server.URL)
				if err != nil || parsed.Query().Get("workspace") != "team-a" {
					t.Fatalf("published URL = %s, error=%v", server.URL, err)
				}
				foundRemote = true
			case "probe":
				foundProbe = true
			}
		}
		if !foundRemote || !foundProbe {
			t.Fatalf("published input servers missing: %#v", servers.MCPServers)
		}
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		PID      int
		ReadOnly string
	}
	if err := json.Unmarshal(data, &observed); err != nil {
		t.Fatal(err)
	}
	if observed.PID == 0 || observed.ReadOnly != "false" {
		t.Fatalf("MCP process input = %#v", observed)
	}
	return observed.PID
}

func TestExtensionInputStdioHelperProcess(t *testing.T) {
	if os.Getenv("COMPOZY_TEST_DAEMON_EXTENSION_HELPER") != "1" || os.Getenv("COMPOZY_TEST_INPUT_REPORT") == "" {
		return
	}
	observed := struct {
		PID      int
		ReadOnly string
	}{os.Getpid(), os.Getenv("READ_ONLY")}
	data, err := json.Marshal(observed)
	if err == nil {
		err = os.WriteFile(os.Getenv("COMPOZY_TEST_INPUT_REPORT"), data, 0o600)
	}
	if err == nil {
		err = mcpfixture.MustNew(mcpfixture.ProfileModern2026).RunStdio(context.Background(), os.Stdin, os.Stdout)
	}
	if err != nil {
		if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
			os.Exit(3)
		}
		os.Exit(2)
	}
	os.Exit(0)
}

func assertDistributionUnconfiguredProfile(t *testing.T, ctx context.Context, runtime *e2etest.RuntimeHarness) {
	t.Helper()
	for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
		var inventory struct {
			Extensions []compozycontract.ExtensionPayload `json:"extensions"`
		}
		if err := read(ctx, http.MethodGet, "/api/extensions?profile=input-isolated", nil, &inventory); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, extension := range inventory.Extensions {
			if extension.Name == "durable-input-kit" {
				found = true
				if len(extension.MissingInputs) == 0 || len(extension.MissingEnv) == 0 {
					t.Fatalf("unconfigured profile inherited inputs: %#v", extension)
				}
			}
		}
		if !found {
			t.Fatal("unconfigured extension missing from profile inventory")
		}
		var servers compozycontract.SettingsMCPServersResponse
		if err := read(
			ctx,
			http.MethodGet,
			"/api/settings/mcp-servers?scope=profile&profile=input-isolated",
			nil,
			&servers,
		); err != nil {
			t.Fatal(err)
		}
		for _, server := range servers.MCPServers {
			if server.Owner == "extension:durable-input-kit" {
				t.Fatalf("unconfigured profile published MCP: %#v", server)
			}
		}
	}
}

// Invariant: checked-in input declarations drive transport refusals, secret storage and optional installation.
// Owner: daemon distribution integration; canonical suite: TestDaemonE2EExtensionDistributionAcrossIsolatedHomes.
func testDaemonCatalogSecretInputs(t *testing.T) {
	t.Run("Should store a supplied secret and omit an optional input", func(t *testing.T) {
		testDaemonCatalogSecretInputMode(t, false)
	})
	t.Run("Should reuse an existing owned vault reference", func(t *testing.T) {
		testDaemonCatalogSecretInputMode(t, true)
	})
}

func testDaemonCatalogSecretInputMode(t *testing.T, reuseRef bool) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 120*time.Second)
	defer cancel()
	catalog := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(catalog.Close)
	runtime := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Extensions.Trust.AllowUnverified = true
			cfg.Marketplace.Catalog.BaseURL = catalog.URL
		}},
		Env: map[string]string{"BRAVE_API_KEY": "", "CONTEXT7_API_KEY": ""},
	})
	root := extensionAuthoringE2ERepoRoot(t)
	const syntheticSecret = "catalog-input-integration-secret"
	request := compozycontract.InstallExtensionRequest{
		Source: compozycontract.InstallExtensionSourceLocalPath,
		Ref:    filepath.Join(root, "catalog", "packages", "brave-search"), Scope: "global", AllowUnverified: true,
	}
	for _, transport := range []struct {
		client *http.Client
		url    string
	}{
		{runtime.HTTPClient, runtime.HTTPURL("/api/extensions")},
		{runtime.UDSClient, runtime.UDSURL("/api/extensions")},
	} {
		payload := requestDistributionInstall(
			t,
			ctx,
			transport.client,
			transport.url,
			request,
			http.StatusUnprocessableEntity,
		)
		var refused compozycontract.ExtensionOperationErrorPayload
		if err := json.Unmarshal(payload, &refused); err != nil {
			t.Fatal(err)
		}
		if refused.Code != "extension_inputs_required" || len(refused.Inputs) != 1 ||
			refused.Inputs[0] != "brave_api_key" || len(refused.InputDefinitions) != 1 {
			t.Fatalf("required Brave input refusal = %#v", refused)
		}
	}
	request.Inputs = map[string]extensioninput.Value{
		"brave_api_key": {Value: json.RawMessage(strconv.Quote(syntheticSecret))},
	}
	ref := vault.ExtensionProfileSecretRef("brave-search", store.DefaultProfileID, "", "BRAVE_API_KEY")
	var before compozycontract.VaultSecretPayload
	if reuseRef {
		stdout, stderr, err := runtime.CLI.RunInDirWithInput(ctx, runtime.WorkspaceRoot,
			strings.NewReader(syntheticSecret), "vault", "put", ref, "--value-stdin", "-o", "json")
		if err != nil {
			t.Fatalf("seed owned vault reference: %v; stderr=%s", err, stderr)
		}
		if err := json.Unmarshal([]byte(stdout), &before); err != nil {
			t.Fatal(err)
		}
		if !before.Present {
			t.Fatal("seeded vault reference is absent")
		}
		request.Inputs = map[string]extensioninput.Value{"brave_api_key": {VaultRef: &ref}}
	}
	payload := requestDistributionInstall(
		t,
		ctx,
		runtime.HTTPClient,
		runtime.HTTPURL("/api/extensions"),
		request,
		http.StatusCreated,
	)
	if bytes.Contains(payload, []byte(syntheticSecret)) || bytes.Contains(payload, []byte("vault:extensions/")) {
		t.Fatal("install response exposed a secret or reference")
	}
	for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
		var bindings compozycontract.ExtensionSecretsPayload
		if err := read(ctx, http.MethodGet, "/api/extensions/brave-search/secrets", nil, &bindings); err != nil {
			t.Fatal(err)
		}
		if len(bindings.BoundEnvKeys) != 1 || bindings.BoundEnvKeys[0] != "BRAVE_API_KEY" ||
			len(bindings.Bindings) != 1 || bindings.Bindings[0].Stale {
			t.Fatalf("stored Brave secret binding = %#v", bindings)
		}
	}
	var after compozycontract.VaultSecretPayload
	if err := runtime.CLI.RunJSON(ctx, &after, "vault", "get", ref, "-o", "json"); err != nil {
		t.Fatal(err)
	}
	if !after.Present || (reuseRef && !after.UpdatedAt.Equal(before.UpdatedAt)) {
		t.Fatal("installation failed to retain the stored vault entry")
	}
	if reuseRef {
		return
	}
	request.Ref = filepath.Join(root, "catalog", "packages", "context7")
	request.Inputs = nil
	payload = requestDistributionInstall(
		t,
		ctx,
		runtime.UDSClient,
		runtime.UDSURL("/api/extensions"),
		request,
		http.StatusCreated,
	)
	var response compozycontract.ExtensionResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	installed := response.Extension
	if installed.Name != "context7" || len(installed.MissingInputs) != 0 || len(installed.MissingEnv) != 0 {
		t.Fatalf("optional Context7 input readiness = %#v", installed)
	}
}

func requestDistributionInstall(
	t *testing.T, ctx context.Context, client *http.Client, target string,
	install compozycontract.InstallExtensionRequest, wantStatus int,
) []byte {
	t.Helper()
	return requestDistributionJSON(t, ctx, client, http.MethodPost, target, install, wantStatus)
}

func requestDistributionJSON(
	t *testing.T, ctx context.Context, client *http.Client, method, target string, input any, wantStatus int,
) []byte {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read install response: read=%v close=%v", readErr, closeErr)
	}
	if response.StatusCode != wantStatus {
		t.Fatalf("install response status=%d, want %d; body=%s", response.StatusCode, wantStatus, payload)
	}
	return payload
}

func (s *distributionGitHubServer) setInputRelease(t *testing.T, version, manifest string) string {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	if err := archive.WriteHeader(
		&tar.Header{Name: "extension.toml", Mode: 0o600, Size: int64(len(manifest))},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.Write([]byte(manifest)); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAsset++
	asset := distributionGitHubAsset{
		ID: s.nextAsset, Name: "hello.tar.gz", URL: s.URL + "/assets/" + strconv.FormatInt(s.nextAsset, 10),
		ContentType: "application/gzip", Size: int64(buffer.Len()), payload: buffer.Bytes(),
	}
	asset.BrowserDownloadURL = asset.URL
	s.assets[asset.ID] = asset
	s.releases = append([]*distributionGitHubRelease{{
		ID: s.nextAsset, Name: version, TagName: "v" + version, Assets: []distributionGitHubAsset{asset},
	}}, s.releases...)
	return fmt.Sprintf("%x", sha256.Sum256(buffer.Bytes()))
}

func assertDistributionInputUpdates(
	t *testing.T, ctx context.Context, runtime *e2etest.RuntimeHarness,
	catalog *distributionGitHubServer, original, reportPath string,
) {
	t.Helper()
	databaseURL := url.URL{Scheme: "file", Path: runtime.HomePaths.DatabaseFile, RawQuery: "mode=rw"}
	db, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	newInput := `
[[inputs]]
id = "region"
prompt = "Region"
type = "identifier"
required = true
binding = { type = "env", name = "REGION" }
`
	second := strings.Replace(original, `version = "1.0.0"`, `version = "2.0.0"`, 1)
	booleanStart := strings.Index(second, "[[inputs]]\nid = \"read_only\"")
	booleanEnd := strings.Index(second[booleanStart:], "[resources.mcp_servers.remote]") + booleanStart
	second = second[:booleanStart] + second[booleanEnd:]
	second = strings.Replace(second, `READ_ONLY = "read_only",`, `REGION = "region",`, 1) + newInput
	catalog.setInputRelease(t, "2.0.0", second)
	update := compozycontract.UpdateExtensionRequest{AllowUnverified: true, Scope: "global"}
	target := runtime.HTTPURL("/api/extensions/durable-input-kit")
	body := requestDistributionJSON(
		t,
		ctx,
		runtime.HTTPClient,
		http.MethodPut,
		target,
		update,
		http.StatusUnprocessableEntity,
	)
	var failure compozycontract.ExtensionOperationErrorPayload
	if err := json.Unmarshal(body, &failure); err != nil {
		t.Fatal(err)
	}
	if failure.Code != "extension_inputs_required" || len(failure.Inputs) != 1 || failure.Inputs[0] != "region" {
		t.Fatalf("candidate input refusal = %#v", failure)
	}
	installedPath := filepath.Join(
		extensionpkg.ManagedInstallPath(runtime.HomePaths, "durable-input-kit"),
		"extension.toml",
	)
	before, err := os.ReadFile(installedPath)
	if err != nil || string(before) != original {
		t.Fatalf("missing-input update changed package: %v", err)
	}
	update.Inputs = map[string]extensioninput.Value{"region": {Value: json.RawMessage(`"us"`)}}
	requestDistributionJSON(t, ctx, runtime.UDSClient, http.MethodPut,
		runtime.UDSURL("/api/extensions/durable-input-kit"), update, http.StatusOK)
	assertDistributionInputRow(t, ctx, db, "read_only", "false", false)
	assertDistributionInputRow(t, ctx, db, "region", `"us"`, true)
	third := strings.Replace(original, `version = "1.0.0"`, `version = "3.0.0"`, 1)
	catalog.setInputRelease(t, "3.0.0", third)
	update.Inputs = nil
	requestDistributionJSON(t, ctx, runtime.HTTPClient, http.MethodPut, target, update, http.StatusOK)
	assertDistributionInputRow(t, ctx, db, "read_only", "false", true)
	assertDistributionInputRow(t, ctx, db, "region", `"us"`, false)
	assertDistributionPublishedInputs(t, ctx, runtime, reportPath)
	failed := strings.Replace(third, `version = "3.0.0"`, `version = "4.0.0"`, 1) + `
[resources.mcp_servers.fail-publication]
command = "server"
`
	catalog.setInputRelease(t, "4.0.0", failed)
	_, err = db.ExecContext(ctx, `CREATE TRIGGER fail_input_publication BEFORE INSERT ON extension_mcp_overrides
WHEN NEW.extension = 'durable-input-kit' AND NEW.server = 'fail-publication'
BEGIN SELECT RAISE(ABORT, 'injected input publication failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	update.Inputs = map[string]extensioninput.Value{"workspace_id": {Value: json.RawMessage(`"changed-team"`)}}
	body = requestDistributionJSON(t, ctx, runtime.UDSClient, http.MethodPut,
		runtime.UDSURL("/api/extensions/durable-input-kit"), update, http.StatusInternalServerError)
	if !bytes.Contains(body, []byte("injected input publication failure")) {
		t.Fatalf("update did not reach publication failure: %s", body)
	}
	if _, err := db.ExecContext(ctx, "DROP TRIGGER fail_input_publication"); err != nil {
		t.Fatal(err)
	}
	assertDistributionInputRow(t, ctx, db, "workspace_id", `"team-a"`, true)
	after, err := os.ReadFile(installedPath)
	if err != nil || string(after) != third {
		t.Fatalf("publication rollback did not restore package: %v", err)
	}
	assertDistributionPublishedInputs(t, ctx, runtime, reportPath)
}

func assertDistributionInputRow(t *testing.T, ctx context.Context, db *sql.DB, id, expected string, active bool) {
	t.Helper()
	var value string
	var storedActive bool
	err := db.QueryRowContext(ctx, `SELECT value_json, active FROM extension_inputs
WHERE extension = 'durable-input-kit' AND profile = ? AND workspace_id = '' AND input_id = ?`,
		store.DefaultProfileID, id).Scan(&value, &storedActive)
	if err != nil || value != expected || storedActive != active {
		t.Fatalf("stored input %s value=%s active=%t: %v", id, value, storedActive, err)
	}
}

// Invariant: curated installs join exact origin/version, updates preserve that origin, and changed bytes cannot install.
// Owner: daemon catalog and extension integration; canonical suite: TestDaemonE2EExtensionDistributionAcrossIsolatedHomes.
func testDaemonCuratedCatalogLifecycle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()
	root := extensionAuthoringE2ERepoRoot(t)
	catalog := newDistributionGitHubServer(t, "catalog-fixture")
	t.Cleanup(catalog.Close)
	contextEntry := distributionCatalogEntry(t, root, "context7")
	bridgeEntry := distributionCatalogEntry(t, root, "herdr-bridge")
	contextArchive := packageDistributionCatalog(t, ctx, root, "context7", contextEntry["version"].(string))
	bridgeArchive := packageDistributionCatalog(t, ctx, root, "herdr-bridge", "0.3.3")
	catalog.setCatalogArtifact(contextEntry, contextArchive)
	bridgeAsset := catalog.setCatalogArtifact(bridgeEntry, bridgeArchive)
	binDir := t.TempDir()
	shim := "#!/bin/sh\nexec " + shellquote.Join("env", "COMPOZY_TEST_DAEMON_EXTENSION_HELPER=1",
		"COMPOZY_TEST_INPUT_REPORT="+filepath.Join(t.TempDir(), "context7-probe.json"),
		os.Args[0], "-test.run=^TestExtensionInputStdioHelperProcess$") + "\n"
	if err := os.WriteFile(filepath.Join(binDir, "npx"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	runtime := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Marketplace.Catalog.BaseURL = catalog.URL
		}},
		Env: map[string]string{
			"PATH":             binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
			"CONTEXT7_API_KEY": "",
		},
	})
	refreshDistributionCatalog(t, ctx, runtime)
	request := compozycontract.InstallExtensionRequest{
		Source: compozycontract.InstallExtensionSourceCurated, Ref: "compozy/context7", Scope: "global",
		ExpectedDigest: contextEntry["digest_sha256"].(string),
	}
	requestDistributionInstall(
		t,
		ctx,
		runtime.HTTPClient,
		runtime.HTTPURL("/api/extensions"),
		request,
		http.StatusCreated,
	)
	assertDistributionCatalogListing(t, ctx, runtime, "context7", "3.2.3", "3.2.3", false)
	var servers compozycontract.SettingsMCPServersResponse
	if err := runtime.HTTPJSON(ctx, http.MethodGet, "/api/settings/mcp-servers", nil, &servers); err != nil {
		t.Fatal(err)
	}
	for _, server := range servers.MCPServers {
		if server.Owner == "extension:context7" &&
			(server.RuntimeStatus == nil || server.RuntimeStatus.State != "ready") {
			t.Fatalf("Context7 fixture probe failed: %#v", server.RuntimeStatus)
		}
	}
	for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
		var inventory struct {
			Extensions []compozycontract.ExtensionPayload `json:"extensions"`
		}
		if err := read(ctx, http.MethodGet, "/api/extensions", nil, &inventory); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, extension := range inventory.Extensions {
			if extension.Name != "context7" {
				continue
			}
			found = true
			if extension.Origin == nil || extension.Origin.Source != marketplacepkg.CompozyCatalogSource ||
				extension.Origin.SourceRef != marketplacepkg.CompozyCatalogRef ||
				extension.Origin.EntryID != "context7" || extension.Contents.MCPServers != 1 ||
				len(extension.Inputs) != 1 || extension.Inputs[0].Set || !extension.Inputs[0].Active ||
				len(
					extension.MissingInputs,
				) != 0 || len(extension.MCPServers) != 1 || extension.MCPServers[0].Status != "running" {
				t.Fatalf("installed catalog input/runtime projection = %#v", extension)
			}
		}
		if !found {
			t.Fatal("Context7 absent from installed inventory")
		}
	}
	catalog.mu.Lock()
	corrupt := catalog.assets[bridgeAsset]
	corrupt.payload = []byte("different artifact bytes")
	catalog.assets[bridgeAsset] = corrupt
	catalog.mu.Unlock()
	request.Ref = bridgeEntry["install_slug"].(string)
	request.ExpectedDigest = bridgeEntry["digest_sha256"].(string)
	body := requestDistributionInstall(
		t,
		ctx,
		runtime.UDSClient,
		runtime.UDSURL("/api/extensions"),
		request,
		http.StatusConflict,
	)
	var changed compozycontract.ExtensionOperationErrorPayload
	if err := json.Unmarshal(body, &changed); err != nil {
		t.Fatal(err)
	}
	if changed.Code != "extension_source_changed" || changed.ListedDigest != request.ExpectedDigest ||
		changed.FetchedDigest != fmt.Sprintf("%x", sha256.Sum256([]byte("different artifact bytes"))) {
		t.Fatalf("artifact mismatch = %#v", changed)
	}
	if _, err := os.Stat(extensionpkg.ManagedInstallPath(runtime.HomePaths, "herdr-bridge")); !os.IsNotExist(err) {
		t.Fatalf("digest mismatch left a partial installation: %v", err)
	}
	assertDistributionCatalogListing(t, ctx, runtime, "herdr-bridge", "0.3.3", "", false)
	catalog.mu.Lock()
	corrupt.payload = bridgeArchive
	catalog.assets[bridgeAsset] = corrupt
	catalog.mu.Unlock()
	requestDistributionInstall(
		t,
		ctx,
		runtime.HTTPClient,
		runtime.HTTPURL("/api/extensions"),
		request,
		http.StatusCreated,
	)
	assertDistributionCatalogListing(t, ctx, runtime, "herdr-bridge", "0.3.3", "0.3.3", false)
	bridgeEntry["version"] = "0.3.4"
	catalog.setCatalogArtifact(bridgeEntry, packageDistributionCatalog(t, ctx, root, "herdr-bridge", "0.3.4"))
	refreshDistributionCatalog(t, ctx, runtime)
	assertDistributionCatalogListing(t, ctx, runtime, "herdr-bridge", "0.3.4", "0.3.3", true)
	requestDistributionJSON(t, ctx, runtime.HTTPClient, http.MethodPost, runtime.HTTPURL("/api/extensions/update"),
		compozycontract.UpdateExtensionsRequest{Names: []string{"herdr-bridge"}}, http.StatusOK)
	assertDistributionCatalogListing(t, ctx, runtime, "herdr-bridge", "0.3.4", "0.3.4", false)
}

func distributionCatalogEntry(t *testing.T, root, id string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "catalog", "v3", "extensions.json"))
	if err != nil {
		t.Fatal(err)
	}
	var feed struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(data, &feed); err != nil {
		t.Fatal(err)
	}
	for _, entry := range feed.Entries {
		if entry["entry_id"] == id {
			return entry
		}
	}
	t.Fatalf("checked-in catalog entry %s missing", id)
	return nil
}

func packageDistributionCatalog(t *testing.T, ctx context.Context, root, name, version string) []byte {
	t.Helper()
	copyRoot := filepath.Join(t.TempDir(), name)
	if err := os.CopyFS(copyRoot, os.DirFS(filepath.Join(root, "catalog", "packages", name))); err != nil {
		t.Fatal(err)
	}
	manifest, err := extensionpkg.LoadManifest(copyRoot)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(copyRoot, "extension.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(
		data,
		[]byte("version = "+strconv.Quote(manifest.Version)),
		[]byte("version = "+strconv.Quote(version)),
		1,
	)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), name+".tar.gz")
	command := execabs.CommandContext(ctx, "go", "run", "./cmd/compozy-catalog", "package", copyRoot, archivePath)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("package catalog fixture: %v\n%s", err, output)
	}
	archive, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

func (s *distributionGitHubServer) setCatalogArtifact(entry map[string]any, archive []byte) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextAsset++
	assetURL := s.URL + "/assets/" + strconv.FormatInt(s.nextAsset, 10)
	entry["artifact_url"], entry["digest_sha256"] = assetURL, fmt.Sprintf("%x", sha256.Sum256(archive))
	s.assets[s.nextAsset] = distributionGitHubAsset{
		ID: s.nextAsset, Name: entry["entry_id"].(string) + ".tar.gz", URL: assetURL,
		BrowserDownloadURL: assetURL, ContentType: "application/gzip", Size: int64(len(archive)), payload: archive,
	}
	s.catalogEntries[entry["entry_id"].(string)] = maps.Clone(entry)
	return s.nextAsset
}

func refreshDistributionCatalog(t *testing.T, ctx context.Context, runtime *e2etest.RuntimeHarness) {
	t.Helper()
	var result compozycontract.MarketplaceRefreshResponse
	if err := runtime.HTTPJSON(ctx, http.MethodPost, "/api/marketplace/refresh", nil, &result); err != nil {
		t.Fatal(err)
	}
}

func assertDistributionCatalogListing(
	t *testing.T,
	ctx context.Context,
	runtime *e2etest.RuntimeHarness,
	id, version, installedVersion string,
	update bool,
) {
	t.Helper()
	for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
		var result compozycontract.MarketplaceListResponse
		if err := read(ctx, http.MethodGet, "/api/marketplace", nil, &result); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, item := range result.Items {
			if item.EntryID != id {
				continue
			}
			found = true
			if item.Installed != (installedVersion != "") || item.Version != version ||
				item.InstalledVersion != installedVersion || item.UpdateAvailable != update {
				t.Fatalf("installed catalog join = %#v", item)
			}
		}
		if !found {
			t.Fatalf("catalog item %s absent", id)
		}
	}
}

// Invariant: public authorization keeps manual and extension credentials separate through registration, exchange and logout.
// Owner: daemon distribution integration; canonical suite: TestDaemonE2EExtensionDistributionAcrossIsolatedHomes, IT-021.
func testDaemonExtensionMCPOwners(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 180*time.Second)
	defer cancel()
	authority := mcpfixture.StartOAuthHTTP(
		t,
		mcpfixture.MustNew(mcpfixture.ProfileModern2026),
		mcpfixture.OAuthConfig{},
	)
	catalog := newDistributionGitHubServer(t, "owner-fixture")
	t.Cleanup(catalog.Close)
	entry := distributionCatalogEntry(t, extensionAuthoringE2ERepoRoot(t), "github")
	entry["version"] = "1.0.0"
	manifest := fmt.Sprintf(`name = "github"
version = "1.0.0"
description = "Owner-isolated MCP fixture"
min_compozy_version = "0.0.0"

[resources.mcp_servers.github]
transport = "http"
url = %q
default_scope = "global"
[resources.mcp_servers.github.auth]
method = "oauth"
registration = "dynamic"
issuer_url = %q
scopes = ["tools.read"]
`, authority.Endpoints.MCPURL, authority.Endpoints.IssuerURL)
	publishDistributionManifest(t, catalog, entry, manifest)
	runtime := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Marketplace.Catalog.BaseURL = catalog.URL
			cfg.Tools.Policy.ExternalDefault = compozyconfig.ToolsExternalDefaultEnabled
		}},
	})
	requestDistributionJSON(t, ctx, runtime.HTTPClient, http.MethodPut,
		runtime.HTTPURL("/api/settings/mcp-servers/github?scope=user"),
		compozycontract.PutSettingsMCPServerRequest{Server: compozycontract.SettingsMCPServerPayload{
			Name: "github", Transport: "http", URL: authority.Endpoints.MCPURL,
			Auth: &compozycontract.SettingsMCPAuthConfigPayload{Registration: "auto",
				IssuerURL: authority.Endpoints.IssuerURL, Scopes: []string{"tools.read"}},
		}}, http.StatusOK)
	refreshDistributionCatalog(t, ctx, runtime)
	requestDistributionInstall(t, ctx, runtime.HTTPClient, runtime.HTTPURL("/api/extensions"),
		compozycontract.InstallExtensionRequest{
			Source:         compozycontract.InstallExtensionSourceCurated,
			Ref:            "compozy/github",
			Scope:          "global",
			ExpectedDigest: entry["digest_sha256"].(string),
		}, http.StatusCreated)
	for _, read := range []func(context.Context, string, string, any, any) error{runtime.HTTPJSON, runtime.UDSJSON} {
		var servers compozycontract.SettingsMCPServersResponse
		if err := read(ctx, http.MethodGet, "/api/settings/mcp-servers?scope=user", nil, &servers); err != nil {
			t.Fatal(err)
		}
		owners := map[string]bool{}
		for _, server := range servers.MCPServers {
			if server.Name != "github" {
				continue
			}
			owners[server.Owner] = true
			if server.Owner == "extension:github" && server.RuntimeName != "github.github" {
				t.Fatalf("extension allocation = %q", server.RuntimeName)
			}
		}
		if len(owners) != 2 || !owners["manual"] || !owners["extension:github"] {
			t.Fatalf("same-name owner definitions = %v", owners)
		}
	}
	databaseURL := url.URL{Scheme: "file", Path: runtime.HomePaths.DatabaseFile, RawQuery: "mode=rw"}
	db, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	authorizeDistributionMCP(t, ctx, runtime.HTTPJSON, authority, "")
	manual := distributionMCPOwnerState(t, ctx, db, "manual", true)
	authorizeDistributionMCP(t, ctx, runtime.UDSJSON, authority, "extension:github")
	if after := distributionMCPOwnerState(t, ctx, db, "manual", true); !slices.Equal(manual, after) {
		t.Fatal("extension authorization changed manual credentials")
	}
	extensionState := distributionMCPOwnerState(t, ctx, db, "extension:github", true)
	if requests := authority.Requests(); len(requests.Registration) != 2 || len(requests.Token) != 2 {
		t.Fatalf("OAuth exchanges/registrations = %d/%d", len(requests.Token), len(requests.Registration))
	}
	refreshDistributionExtensionMCP(t, ctx, runtime, db)
	requests := authority.Requests()
	if len(requests.Registration) != 2 || len(requests.Token) != 3 ||
		requests.Token[2].Get("grant_type") != "refresh_token" {
		t.Fatalf(
			"extension invocation produced %d token requests, want two exchanges and one refresh",
			len(requests.Token),
		)
	}
	if after := distributionMCPOwnerState(t, ctx, db, "extension:github", true); after[1] != extensionState[1] {
		t.Fatal("extension refresh changed its persisted client registration")
	}
	if after := distributionMCPOwnerState(t, ctx, db, "manual", true); !slices.Equal(manual, after) {
		t.Fatal("extension refresh changed manual credentials")
	}
	var status compozycontract.SettingsMCPAuthStatusPayload
	if err := runtime.HTTPJSON(ctx, http.MethodPost,
		"/api/settings/mcp-servers/github/auth/logout?scope=user&owner=extension:github", nil, &status); err != nil {
		t.Fatal(err)
	}
	if status.Owner != "extension:github" || status.TokenPresent {
		t.Fatalf("logout status = owner:%s token:%v", status.Owner, status.TokenPresent)
	}
	distributionMCPOwnerState(t, ctx, db, "extension:github", false)
	if after := distributionMCPOwnerState(t, ctx, db, "manual", true); !slices.Equal(manual, after) {
		t.Fatal("extension logout changed manual credentials")
	}
	assertDistributionMCPOwnerUpdates(t, ctx, runtime, db, catalog, entry, manifest)
}

// Invariant: owner-qualified overrides survive package updates and competitor removal without changing package bytes or names.
// Owner: daemon distribution integration; canonical suite: IT-021.
func assertDistributionMCPOwnerUpdates(
	t *testing.T, ctx context.Context, runtime *e2etest.RuntimeHarness, db *sql.DB,
	catalog *distributionGitHubServer, entry map[string]any, manifest string,
) {
	t.Helper()
	base := "/api/settings/mcp-servers/github?scope=user"
	for _, path := range []string{
		"/api/settings/mcp-servers/github.github?scope=user",
		"/api/settings/mcp-servers/github.github?scope=user&owner=extension:github",
		"/api/settings/mcp-servers/github.github/auth/status?scope=user",
		"/api/settings/mcp-servers/github.github/auth/status?scope=user&owner=extension:github",
	} {
		body := requestDistributionJSON(
			t,
			ctx,
			runtime.UDSClient,
			http.MethodGet,
			runtime.UDSURL(path),
			nil,
			http.StatusNotFound,
		)
		if !strings.Contains(string(body), "not found") {
			t.Fatalf("runtime alias error = %s", body)
		}
	}
	path := filepath.Join(extensionpkg.ManagedInstallPath(runtime.HomePaths, "github"), "extension.toml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	requestDistributionJSON(t, ctx, runtime.UDSClient, http.MethodPut, runtime.UDSURL(base+"&owner=extension:github"),
		compozycontract.PutSettingsMCPServerRequest{Server: compozycontract.SettingsMCPServerPayload{
			Name: "github", Headers: map[string]string{"X-Workspace": "override"},
		}}, http.StatusOK)
	if after, err := os.ReadFile(path); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("override changed installed package bytes: %v", err)
	}
	assertDistributionMCPOverride(t, ctx, runtime, db, "github", "github.github", "override")
	var manual compozycontract.SettingsMCPServerResponse
	if err := runtime.HTTPJSON(ctx, http.MethodGet, base, nil, &manual); err != nil {
		t.Fatal(err)
	}
	manualURL := manual.Server.URL + "?manual=1"
	requestDistributionJSON(t, ctx, runtime.HTTPClient, http.MethodPut, runtime.HTTPURL(base),
		compozycontract.PutSettingsMCPServerRequest{Server: compozycontract.SettingsMCPServerPayload{
			Name: "github", Transport: "http", URL: manualURL,
		}}, http.StatusOK)
	if err := runtime.HTTPJSON(ctx, http.MethodGet, base, nil, &manual); err != nil {
		t.Fatal(err)
	}
	if manual.Server.Owner != "manual" || manual.Server.URL != manualURL {
		t.Fatalf(
			"owner-less update did not target manual definition: owner=%s url=%s",
			manual.Server.Owner,
			manual.Server.URL,
		)
	}
	assertDistributionMCPOverride(t, ctx, runtime, db, "github", "github.github", "override")
	body := requestDistributionJSON(t, ctx, runtime.HTTPClient, http.MethodPut,
		runtime.HTTPURL("/api/settings/mcp-servers/github.github?scope=user"),
		compozycontract.PutSettingsMCPServerRequest{Server: compozycontract.SettingsMCPServerPayload{
			Name: "github.github", Transport: "http", URL: manualURL,
		}}, http.StatusUnprocessableEntity)
	if !strings.Contains(string(body), "mcp_server_name_taken") {
		t.Fatalf("manual name collision = %s", body)
	}
	entry["version"] = "2.0.0"
	updated := strings.Replace(manifest, `version = "1.0.0"`, `version = "2.0.0"`, 1)
	publishDistributionManifest(t, catalog, entry, updated)
	refreshDistributionCatalog(t, ctx, runtime)
	requestDistributionJSON(t, ctx, runtime.HTTPClient, http.MethodPost, runtime.HTTPURL("/api/extensions/update"),
		compozycontract.UpdateExtensionsRequest{Names: []string{"github"}}, http.StatusOK)
	if after, err := os.ReadFile(path); err != nil || string(after) != updated {
		t.Fatalf("package did not update: %v", err)
	}
	assertDistributionMCPOverride(t, ctx, runtime, db, "github", "github.github", "override")
	peer := maps.Clone(entry)
	peer["entry_id"], peer["install_slug"], peer["name"] = "github-peer", "compozy/github-peer", "GitHub peer"
	peerManifest := strings.Replace(updated, `name = "github"`, `name = "github-peer"`, 1)
	publishDistributionManifest(t, catalog, peer, peerManifest)
	refreshDistributionCatalog(t, ctx, runtime)
	requestDistributionInstall(t, ctx, runtime.HTTPClient, runtime.HTTPURL("/api/extensions"),
		compozycontract.InstallExtensionRequest{
			Source:         compozycontract.InstallExtensionSourceCurated,
			Ref:            "compozy/github-peer",
			Scope:          "global",
			ExpectedDigest: peer["digest_sha256"].(string),
		}, http.StatusCreated)
	assertDistributionMCPOverride(t, ctx, runtime, db, "github-peer", "github-peer.github", "")
	requestDistributionJSON(t, ctx, runtime.UDSClient, http.MethodDelete, runtime.UDSURL(base), nil, http.StatusOK)
	assertDistributionMCPOverride(t, ctx, runtime, db, "github", "github.github", "override")
	assertDistributionMCPOverride(t, ctx, runtime, db, "github-peer", "github-peer.github", "")
}

func assertDistributionMCPOverride(
	t *testing.T, ctx context.Context, runtime *e2etest.RuntimeHarness, db *sql.DB,
	extension, runtimeName, header string,
) {
	t.Helper()
	var storedName, headersJSON string
	if err := db.QueryRowContext(ctx, `SELECT runtime_name, headers_json FROM extension_mcp_overrides
 WHERE extension = ? AND profile = ? AND workspace_id = '' AND server = 'github'`,
		extension, store.DefaultProfileID).Scan(&storedName, &headersJSON); err != nil {
		t.Fatal(err)
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		t.Fatal(err)
	}
	if storedName != runtimeName || headers["X-Workspace"] != header {
		t.Fatalf("stored override = name:%s headers:%v", storedName, headers)
	}
	var response compozycontract.SettingsMCPServerResponse
	if err := runtime.HTTPJSON(
		ctx,
		http.MethodGet,
		"/api/settings/mcp-servers/github?scope=user&owner="+url.QueryEscape(
			"extension:"+extension,
		),
		nil,
		&response,
	); err != nil {
		t.Fatal(err)
	}
	if response.Server.Owner != "extension:"+extension || response.Server.RuntimeName != runtimeName ||
		response.Server.Override == nil || response.Server.Override.Headers["X-Workspace"] != header {
		t.Fatalf(
			"published override = owner:%s name:%s override:%v",
			response.Server.Owner,
			response.Server.RuntimeName,
			response.Server.Override,
		)
	}
}

// Invariant: execution by discovered resource identity refreshes only the expired extension target.
// Owner: daemon distribution integration; canonical suite: IT-021.
func refreshDistributionExtensionMCP(t *testing.T, ctx context.Context, runtime *e2etest.RuntimeHarness, db *sql.DB) {
	t.Helper()
	var inventory compozycontract.ToolsResponse
	if err := runtime.HTTPJSON(
		ctx,
		http.MethodGet,
		"/api/tools?workspace_id="+runtime.WorkspaceID,
		nil,
		&inventory,
	); err != nil {
		t.Fatal(err)
	}
	var id toolspkg.ToolID
	for _, tool := range inventory.Tools {
		if tool.Descriptor.Source.RawServerName == "github.github" && tool.Descriptor.Source.RawToolName == "echo" {
			id = tool.Descriptor.ToolID
		}
	}
	if id == "" {
		t.Fatal("authorized extension echo absent from discovered tools")
	}
	result, err := db.ExecContext(ctx, `UPDATE mcp_auth_tokens SET expires_at = '2000-01-01T00:00:00Z'
 WHERE scope = 'user' AND workspace_id = '' AND owner = 'extension:github' AND server_name = 'github'`)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := result.RowsAffected(); err != nil || rows != 1 {
		t.Fatalf("expire extension token: rows=%d error=%v", rows, err)
	}
	var response compozycontract.ToolInvokeResponse
	if err := runtime.UDSJSON(
		ctx,
		http.MethodPost,
		"/api/tools/"+url.PathEscape(string(id))+"/invoke",
		compozycontract.ToolInvokeRequest{
			WorkspaceID: runtime.WorkspaceID,
			Input:       json.RawMessage(`{"message":"owner-isolated"}`),
		},
		&response,
	); err != nil {
		t.Fatal(err)
	}
	if len(response.Result.Content) != 1 || response.Result.Content[0].Text != "echo: owner-isolated" {
		t.Fatalf("extension echo response = %#v", response.Result.Content)
	}
}

func publishDistributionManifest(
	t *testing.T,
	catalog *distributionGitHubServer,
	entry map[string]any,
	manifest string,
) {
	t.Helper()
	catalog.setInputRelease(t, entry["version"].(string), manifest)
	catalog.mu.Lock()
	archive := bytes.Clone(catalog.assets[catalog.nextAsset].payload)
	catalog.mu.Unlock()
	catalog.setCatalogArtifact(entry, archive)
}

func authorizeDistributionMCP(
	t *testing.T, ctx context.Context, request func(context.Context, string, string, any, any) error,
	authority *mcpfixture.OAuthHTTPServer, owner string,
) {
	t.Helper()
	query := url.Values{"scope": {"user"}}
	if owner != "" {
		query.Set("owner", owner)
	}
	base := "/api/settings/mcp-servers/github/auth/"
	var begin compozycontract.SettingsMCPAuthBeginResponse
	if err := request(
		ctx,
		http.MethodPost,
		base+"begin?"+query.Encode(),
		compozycontract.SettingsMCPAuthBeginRequest{
			Mode: compozycontract.SettingsMCPAuthBeginModeManual,
		},
		&begin,
	); err != nil {
		t.Fatal(err)
	}
	client := *authority.Server.Client()
	client.Timeout = 10 * time.Second
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	authorize, err := http.NewRequestWithContext(ctx, http.MethodGet, begin.AuthorizationURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(authorize)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Error(err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusFound || response.Header.Get("Location") == "" {
		t.Fatalf("OAuth authorization response = %d", response.StatusCode)
	}
	var status compozycontract.SettingsMCPAuthStatusPayload
	if err := request(
		ctx,
		http.MethodPost,
		base+"exchange?"+query.Encode(),
		compozycontract.SettingsMCPAuthExchangeRequest{
			RedirectURL: response.Header.Get("Location"),
		},
		&status,
	); err != nil {
		t.Fatal(err)
	}
	if owner == "" {
		owner = "manual"
	}
	if status.Owner != owner || status.ServerName != "github" || status.Status != "authenticated" ||
		!status.TokenPresent {
		t.Fatalf("authorization status = owner:%s name:%s status:%s token:%v",
			status.Owner, status.ServerName, status.Status, status.TokenPresent)
	}
}

func distributionMCPOwnerState(t *testing.T, ctx context.Context, db *sql.DB, owner string, present bool) []string {
	t.Helper()
	prefix, err := vault.MCPSecretOwnerPrefix(
		vault.MCPSecretTarget{Owner: owner, Scope: vault.MCPUserScope, ServerName: "github"},
	)
	if err != nil {
		t.Fatal(err)
	}
	queries := []struct {
		query    string
		argument string
		count    int
	}{
		{`SELECT json_group_array(json_array(scope,workspace_id,server_name,owner,definition_fingerprint,issuer,
 client_id,scopes_json,access_token_ref,refresh_token_ref,token_type,expires_at,obtained_at,updated_at))
 FROM mcp_auth_tokens WHERE owner = ?`, owner, 1},
		{`SELECT json_group_array(json_array(scope,workspace_id,server_name,owner,definition_fingerprint,resource_url,
 issuer,client_id,token_endpoint_auth_method,client_secret_ref,registration_access_token_ref,registration_client_uri,
 client_id_issued_at,client_secret_expires_at,redirect_uri,scopes_json,updated_at))
 FROM mcp_oauth_registrations WHERE owner = ?`, owner, 1},
		{`SELECT json_group_array(json_array(ref,kind,hex(encrypted_value),created_at,updated_at))
 FROM (SELECT * FROM vault_secrets WHERE ref LIKE ? ORDER BY ref)`, prefix + "%", 4},
	}
	result := make([]string, len(queries))
	for i, check := range queries {
		if err := db.QueryRowContext(ctx, check.query, check.argument).Scan(&result[i]); err != nil {
			t.Fatal(err)
		}
		var rows []json.RawMessage
		if err := json.Unmarshal([]byte(result[i]), &rows); err != nil {
			t.Fatal(err)
		}
		want := check.count
		if !present {
			want = 0
		}
		if len(rows) != want {
			t.Fatalf("owner %s state %d has %d records, want %d", owner, i, len(rows), want)
		}
	}
	return result
}
