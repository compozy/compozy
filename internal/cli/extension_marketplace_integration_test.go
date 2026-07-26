//go:build integration

package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnosticcontract"
	extensionpkg "github.com/compozy/agh/internal/extension"
	registrypkg "github.com/compozy/agh/internal/registry"
	registrygithub "github.com/compozy/agh/internal/registry/github"
)

type extensionRegistryTestEnv struct {
	deps      commandDeps
	homePaths aghconfig.HomePaths
}

type extensionRegistryTestOptions struct {
	allowUnverified bool
	trust           *extensionpkg.MarketplaceTrustEvidence
}

type extensionRegistrySourceStub struct {
	name         string
	caps         registrypkg.SourceCaps
	infoFunc     func(context.Context, string) (*registrypkg.Detail, error)
	downloadFunc func(context.Context, string, registrypkg.DownloadOpts) (*registrypkg.DownloadResult, error)
}

var _ registrypkg.Source = (*extensionRegistrySourceStub)(nil)

func newExtensionRegistryTestEnv(
	t *testing.T,
	opts extensionRegistryTestOptions,
	sources ...registrypkg.Source,
) extensionRegistryTestEnv {
	t.Helper()

	h := newIntegrationHarness(t)
	h.runner.cfg.Extensions.Marketplace.AllowUnverified = opts.allowUnverified
	h.runner.extensionTrust = opts.trust
	h.runner.extensionSources = append([]registrypkg.Source(nil), sources...)
	mustExecuteRoot(t, h.deps, "daemon", "start", "-o", "json")
	waitForCondition(t, h.deps.startTimeout, func() bool {
		_, err := os.Stat(h.homePaths.DaemonInfo)
		return err == nil
	})
	t.Cleanup(func() {
		if _, _, err := executeRootCommand(t, h.deps, "daemon", "stop", "-o", "json"); err != nil {
			t.Fatalf("daemon stop cleanup error = %v", err)
		}
	})
	return extensionRegistryTestEnv{deps: h.deps, homePaths: h.homePaths}
}

func (s *extensionRegistrySourceStub) Name() string {
	if strings.TrimSpace(s.name) == "" {
		return "github"
	}
	return s.name
}

func (s *extensionRegistrySourceStub) Capabilities() registrypkg.SourceCaps {
	return s.caps
}

func (s *extensionRegistrySourceStub) Search(
	context.Context,
	string,
	registrypkg.SearchOpts,
) ([]registrypkg.Listing, error) {
	return nil, errors.New("extension registry search is not configured")
}

func (s *extensionRegistrySourceStub) Info(ctx context.Context, slug string) (*registrypkg.Detail, error) {
	if s.infoFunc != nil {
		return s.infoFunc(ctx, slug)
	}
	return nil, errors.New("extension registry info is not configured")
}

func (s *extensionRegistrySourceStub) Download(
	ctx context.Context,
	slug string,
	opts registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	if s.downloadFunc != nil {
		return s.downloadFunc(ctx, slug, opts)
	}
	return nil, errors.New("extension registry download is not configured")
}

func (s *extensionRegistrySourceStub) Close() error {
	return nil
}

func newExtensionDownloadResult(
	t *testing.T,
	slug string,
	version string,
	files map[string]string,
) *registrypkg.DownloadResult {
	t.Helper()

	archive := extensionArchive(t, files)
	return &registrypkg.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(archive)),
		Slug:        slug,
		Version:     version,
		ContentSize: int64(len(archive)),
		ContentType: "application/gzip",
	}
}

func remoteExtensionArchiveFiles(name string, version string) map[string]string {
	return map[string]string{
		filepath.Join(name, "extension.toml"): fmt.Sprintf(
			"[extension]\nname = %q\nversion = %q\ndescription = \"CLI marketplace integration fixture\"\nmin_agh_version = \"0.5.0\"\n",
			name,
			version,
		),
		filepath.Join(name, "README.md"): "# " + name + "\n",
	}
}

func extensionArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for name, content := range files {
		header := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatalf("tarWriter.WriteHeader(%q) error = %v", name, err)
		}
		if _, err := tarWriter.Write([]byte(content)); err != nil {
			t.Fatalf("tarWriter.Write(%q) error = %v", name, err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tarWriter.Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzipWriter.Close() error = %v", err)
	}
	return buffer.Bytes()
}

func TestExtensionInstallCommandIntegrationCreatesManagedInstallAndRegistryRecord(t *testing.T) {
	t.Parallel()
	t.Run("Should create a managed install and registry record", func(t *testing.T) {
		t.Parallel()

		env := newExtensionRegistryTestEnv(
			t,
			extensionRegistryTestOptions{allowUnverified: true},
			&extensionRegistrySourceStub{
				name: "github",
				infoFunc: func(_ context.Context, slug string) (*registrypkg.Detail, error) {
					return &registrypkg.Detail{Listing: registrypkg.Listing{
						Slug:    slug,
						Name:    "integration-ext",
						Version: "1.0.0",
						Source:  "github",
					}}, nil
				},
				downloadFunc: func(_ context.Context, slug string, _ registrypkg.DownloadOpts) (*registrypkg.DownloadResult, error) {
					return newExtensionDownloadResult(
						t,
						slug,
						"1.0.0",
						remoteExtensionArchiveFiles("integration-ext", "1.0.0"),
					), nil
				},
			},
		)

		stdout, stderr, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"install",
			"acme/integration-ext",
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension install integration error = %v", err)
		}

		var payload ExtensionRecord
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(extension install integration) error = %v; stdout=%s", err, stdout)
		}
		if payload.Source != "marketplace" {
			t.Fatalf("extension install integration payload = %#v, want marketplace source", payload)
		}
		if strings.TrimSpace(stderr) != "" {
			t.Fatalf("extension install integration stderr = %q, want empty daemon-managed lifecycle guidance", stderr)
		}

		info := getInstalledExtension(t, env.homePaths, "integration-ext")
		if info.Source != extensionpkg.SourceMarketplace {
			t.Fatalf("installed source = %v, want marketplace", info.Source)
		}
	})
}

func TestExtensionUpdateAndRemoveIntegration(t *testing.T) {
	t.Parallel()
	t.Run("Should update and remove a managed marketplace extension", func(t *testing.T) {
		t.Parallel()

		latestVersion := "1.0.0"
		env := newExtensionRegistryTestEnv(
			t,
			extensionRegistryTestOptions{allowUnverified: true},
			&extensionRegistrySourceStub{
				name: "github",
				infoFunc: func(_ context.Context, slug string) (*registrypkg.Detail, error) {
					return &registrypkg.Detail{Listing: registrypkg.Listing{
						Slug:    slug,
						Name:    "integration-update-ext",
						Version: latestVersion,
						Source:  "github",
					}}, nil
				},
				downloadFunc: func(_ context.Context, slug string, opts registrypkg.DownloadOpts) (*registrypkg.DownloadResult, error) {
					version := firstNonEmpty(opts.Version, latestVersion)
					return newExtensionDownloadResult(
						t,
						slug,
						version,
						remoteExtensionArchiveFiles("integration-update-ext", version),
					), nil
				},
			},
		)

		if _, _, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"install",
			"acme/integration-update-ext",
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("extension install before update integration error = %v", err)
		}

		latestVersion = "1.3.0"
		checkOut, _, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"update",
			"integration-update-ext",
			"--check",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension update --check integration error = %v", err)
		}

		var checkItems []extensionUpdateItem
		if err := json.Unmarshal([]byte(checkOut), &checkItems); err != nil {
			t.Fatalf("json.Unmarshal(extension update --check integration) error = %v; stdout=%s", err, checkOut)
		}
		if len(checkItems) != 1 || checkItems[0].Status != "available" {
			t.Fatalf("extension update --check integration items = %#v, want update available", checkItems)
		}

		updateOut, _, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"update",
			"integration-update-ext",
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension update integration error = %v", err)
		}
		var updateItems []extensionUpdateItem
		if err := json.Unmarshal([]byte(updateOut), &updateItems); err != nil {
			t.Fatalf("json.Unmarshal(extension update integration) error = %v; stdout=%s", err, updateOut)
		}
		if len(updateItems) != 1 || updateItems[0].Status != "updated" {
			t.Fatalf("extension update integration items = %#v, want updated", updateItems)
		}

		info := getInstalledExtension(t, env.homePaths, "integration-update-ext")
		if info.Version != "1.3.0" {
			t.Fatalf("installed version after update = %q, want %q", info.Version, "1.3.0")
		}

		if _, _, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"remove",
			"integration-update-ext",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("extension remove integration error = %v", err)
		}

		installDir := extensionpkg.ManagedInstallPath(env.homePaths, "integration-update-ext")
		if _, err := os.Stat(installDir); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("removed install dir stat error = %v, want not exist", err)
		}
	})
}

func TestExtensionRemoveMissingIntegrationReturnsClearError(t *testing.T) {
	t.Parallel()
	t.Run("Should return the canonical error for a missing extension", func(t *testing.T) {
		t.Parallel()

		env := newExtensionRegistryTestEnv(t, extensionRegistryTestOptions{})

		_, _, err := executeRootCommand(t, env.deps, "extension", "remove", "missing-ext", "-o", "json")
		if err == nil {
			t.Fatal("extension remove missing integration error = nil, want failure")
		}
		if !strings.Contains(err.Error(), extensionpkg.ErrExtensionNotFound.Error()) {
			t.Fatalf("extension remove missing integration error = %v, want clear not-found error", err)
		}
	})
}

func TestExtensionInstallCommandIntegrationReturnsStructuredPolicyBlock(t *testing.T) {
	t.Parallel()
	t.Run("Should return a structured diagnostic when side-load policy blocks install", func(t *testing.T) {
		t.Parallel()

		env := newExtensionRegistryTestEnv(t, extensionRegistryTestOptions{})
		exitCode, stdout, stderr := executeRootCommandWithExit(
			t,
			env.deps,
			"extension",
			"install",
			"acme/blocked-ext",
			"--allow-unverified",
			"--yes",
			"-o",
			"json",
		)
		if exitCode == 0 {
			t.Fatalf("extension install policy exit code = %d, want non-zero", exitCode)
		}
		if strings.TrimSpace(stdout) != "" {
			t.Fatalf("extension install policy stdout = %q, want empty", stdout)
		}
		for _, want := range []string{
			diagnosticcontract.CodeExtensionUnverifiedPolicyBlocked,
			"Settings › Extensions",
			"extensions.marketplace.allow_unverified",
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("extension install policy stderr = %q, want %q", stderr, want)
			}
		}
	})
}

func TestExtensionInstallCommandIntegrationVerifiesCuratedGitHubArchive(t *testing.T) {
	t.Parallel()
	t.Run("Should verify a curated GitHub archive without unverified consent", func(t *testing.T) {
		t.Parallel()

		archive := extensionArchive(t, remoteExtensionArchiveFiles("curated-ext", "1.0.0"))
		digest := fmt.Sprintf("%x", sha256.Sum256(archive))
		server := newCuratedExtensionGitHubServer(t, archive)
		env := newExtensionRegistryTestEnv(
			t,
			extensionRegistryTestOptions{trust: &extensionpkg.MarketplaceTrustEvidence{
				CatalogEntryID:      "extension.acme.curated-ext",
				Version:             "1.0.0",
				ArchiveDigestSHA256: digest,
				RegistryTier:        "official",
			}},
			registrygithub.NewClient(server.URL),
		)

		if _, _, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"install",
			"acme/curated-ext",
			"--yes",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("extension curated install error = %v", err)
		}

		stdout, stderr, err := executeRootCommand(
			t,
			env.deps,
			"extension",
			"provenance",
			"curated-ext",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("extension provenance error = %v; stderr=%s", err, stderr)
		}
		var provenance ExtensionProvenanceRecord
		if err := json.Unmarshal([]byte(stdout), &provenance); err != nil {
			t.Fatalf("json.Unmarshal(extension provenance) error = %v; stdout=%s", err, stdout)
		}
		if provenance.CatalogEntryID != "extension.acme.curated-ext" ||
			provenance.ArchiveDigestSHA256 != digest ||
			provenance.ChecksumSHA256 == digest ||
			!provenance.ChecksumVerified ||
			provenance.RegistryTier != "official" ||
			provenance.Trust == nil ||
			provenance.Trust.Decision != extensionpkg.ExtensionTrustDecisionVerified {
			t.Fatalf("extension curated provenance = %#v, want verified catalog/archive/tree identity", provenance)
		}
	})
}

func newCuratedExtensionGitHubServer(t *testing.T, archive []byte) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/repos/acme/curated-ext/releases/latest", "/repos/acme/curated-ext/releases/tags/1.0.0":
			writer.Header().Set("Content-Type", "application/json")
			_, err := fmt.Fprintf(writer, `{
				"tag_name":"1.0.0",
				"draft":false,
				"prerelease":false,
				"assets":[{
					"name":"curated-ext-1.0.0.tar.gz",
					"url":%q,
					"content_type":"application/gzip",
					"size":%d
				}]
			}`, server.URL+"/assets/curated-ext-1.0.0.tar.gz", len(archive))
			if err != nil {
				t.Errorf("write GitHub release response error = %v", err)
			}
		case "/repos/acme/curated-ext/releases":
			writer.Header().Set("Content-Type", "application/json")
			if _, err := writer.Write([]byte(`[{"tag_name":"1.0.0","draft":false,"prerelease":false}]`)); err != nil {
				t.Errorf("write GitHub releases response error = %v", err)
			}
		case "/assets/curated-ext-1.0.0.tar.gz":
			writer.Header().Set("Content-Type", "application/gzip")
			if _, err := writer.Write(archive); err != nil {
				t.Errorf("write GitHub archive response error = %v", err)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}
