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
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
	var firstBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(t, ctx, publisher, &firstBuild, "extension", "build", sourceDir, "-o", "json")
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
	var enabled compozycontract.ExtensionEnableResult
	runExtensionAuthoringCLI(
		t,
		ctx,
		consumer,
		&enabled,
		"extension", "enable", "hello", "-o", "json",
	)
	if !enabled.Extension.Enabled {
		t.Fatalf("extension enable result = %#v, want enabled extension", enabled)
	}
	assertDistributionExtensionInvocation(t, ctx, consumer, "published-v1:alpha")

	rewriteExtensionAuthoringGeneration(t, sourceDir, "published-v1:", "published-v2:", "published-v2")
	var secondBuild extensionpkg.BuildResult
	runExtensionAuthoringCLI(t, ctx, publisher, &secondBuild, "extension", "build", sourceDir, "-o", "json")
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

	var updates []compozycontract.ManagedExtensionUpdatePayload
	runExtensionAuthoringCLI(
		t,
		ctx,
		consumer,
		&updates,
		"extension", "update", "hello", "--allow-unverified", "--yes", "-o", "json",
	)
	if len(updates) != 1 || updates[0].Status != extensionpkg.MarketplaceUpdateStatusUpdated ||
		updates[0].LatestVersion != "v0.2.0" {
		t.Fatalf("extension update result = %#v, want one update to v0.2.0", updates)
	}
	assertDistributionExtensionInvocation(t, ctx, consumer, "published-v2:alpha")

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
