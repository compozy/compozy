package daemon

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
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	extensionpkg "github.com/compozy/agh/internal/extension"
	marketplacepkg "github.com/compozy/agh/internal/marketplace"
	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type nativeExtensionSource struct {
	latestVersion string
	downloads     map[string]*registrypkg.DownloadResult
}

type nativeExtensionCatalog struct {
	entry *marketplacepkg.Entry
	err   error
}

func (nativeExtensionCatalog) Close(context.Context) error { return nil }

func (c nativeExtensionCatalog) Browse(
	context.Context,
	marketplacepkg.Kind,
	string,
	int,
	int,
) (marketplacepkg.BrowseResult, error) {
	return marketplacepkg.BrowseResult{}, errors.New("unexpected catalog browse")
}

func (c nativeExtensionCatalog) Detail(context.Context, marketplacepkg.Kind, string) (*marketplacepkg.Entry, error) {
	return nil, errors.New("unexpected catalog detail")
}

func (c nativeExtensionCatalog) ResolveExtensionInstall(
	context.Context,
	string,
	string,
) (*marketplacepkg.Entry, error) {
	return c.entry, c.err
}

func (c nativeExtensionCatalog) Refresh(context.Context, ...marketplacepkg.Kind) (marketplacepkg.RefreshReport, error) {
	return marketplacepkg.RefreshReport{}, errors.New("unexpected catalog refresh")
}

func (c nativeExtensionCatalog) Status(context.Context) ([]marketplacepkg.KindState, error) {
	return nil, errors.New("unexpected catalog status")
}

var _ registrypkg.Source = (*nativeExtensionSource)(nil)

func (s *nativeExtensionSource) Name() string {
	return "github"
}

func (s *nativeExtensionSource) Capabilities() registrypkg.SourceCaps {
	return registrypkg.SourceCaps{Search: true}
}

func (s *nativeExtensionSource) Search(
	context.Context,
	string,
	registrypkg.SearchOpts,
) ([]registrypkg.Listing, error) {
	return []registrypkg.Listing{
		{
			Slug:    "acme/tool-ext",
			Name:    "tool-ext",
			Version: s.latestVersion,
			Source:  s.Name(),
			Type:    registrypkg.PackageTypeExtension,
		},
	}, nil
}

func (s *nativeExtensionSource) Info(context.Context, string) (*registrypkg.Detail, error) {
	return &registrypkg.Detail{
		Listing: registrypkg.Listing{
			Slug:    "acme/tool-ext",
			Name:    "tool-ext",
			Version: s.latestVersion,
			Source:  s.Name(),
			Type:    registrypkg.PackageTypeExtension,
		},
	}, nil
}

func (s *nativeExtensionSource) Download(
	_ context.Context,
	_ string,
	opts registrypkg.DownloadOpts,
) (*registrypkg.DownloadResult, error) {
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = s.latestVersion
	}
	result := s.downloads[version]
	if result == nil {
		return nil, fmt.Errorf("test source missing version %q", version)
	}
	return result, nil
}

func (s *nativeExtensionSource) Close() error {
	return nil
}

func TestDaemonNativeExtensionTools(t *testing.T) {
	t.Run("Should install local extension sources through managed install", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, _, runtime := newNativeExtensionToolDeps(t)
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
		sourceDir := writeNativeLocalExtensionFixture(t, "local-tool-ext", "1.0.0")

		installResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInstall,
				Input:  json.RawMessage(fmt.Sprintf(`{"source":"local","path":%q,"allow_unverified":true}`, sourceDir)),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_install local) error = %v", err)
		}
		requireNativeStructuredContains(t, installResult, []byte(`"local-tool-ext"`))
		info, err := extRegistry.Get("local-tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(local) error = %v", err)
		}
		if info.Source != extensionpkg.SourceUser {
			t.Fatalf("local install source = %s, want user", info.Source)
		}
		if info.ManifestPath == filepath.Join(sourceDir, "extension.toml") {
			t.Fatalf("local install manifest path = %q, want managed copy", info.ManifestPath)
		}
		if runtime.reloadCount != 1 {
			t.Fatalf("reload count after local install = %d, want 1", runtime.reloadCount)
		}

		_, err = registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInstall,
				Input:  json.RawMessage(fmt.Sprintf(`{"source":"local","path":%q,"slug":"acme/bad"}`, sourceDir)),
			},
		)
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonExtensionValidationFailed)
	})

	t.Run("Should require live policy plus request consent for local side-loads", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, _, runtime := newNativeExtensionToolDeps(t)
		deps.ExtensionMarket.AllowUnverified = false
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())
		sourceDir := writeNativeLocalExtensionFixture(t, "blocked-local-ext", "1.0.0")

		_, err := registry.Call(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsInstall,
			Input:  json.RawMessage(fmt.Sprintf(`{"source":"local","path":%q,"allow_unverified":true}`, sourceDir)),
		})
		requireToolReason(t, err, toolspkg.ErrToolDenied, toolspkg.ReasonExtensionSourceForbidden)
		if _, err := extRegistry.Get("blocked-local-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("extension registry Get(blocked local) error = %v, want ErrExtensionNotFound", err)
		}
		if runtime.reloadCount != 0 {
			t.Fatalf("reload count after policy block = %d, want 0", runtime.reloadCount)
		}
	})

	t.Run("Should manage marketplace extension lifecycle through native tools", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, source, runtime := newNativeExtensionToolDeps(t)
		source.latestVersion = "1.0.0"
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())

		installResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInstall,
				Input: json.RawMessage(
					`{"source":"marketplace","slug":"acme/tool-ext","registry":"github","allow_unverified":true}`,
				),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_install) error = %v", err)
		}
		requireNativeStructuredContains(t, installResult, []byte(`"tool-ext"`))
		if runtime.reloadCount != 1 {
			t.Fatalf("reload count after install = %d, want 1", runtime.reloadCount)
		}
		installed, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(installed) error = %v", err)
		}
		if installed.Source != extensionpkg.SourceMarketplace ||
			derefNativeExtensionString(installed.RegistrySlug) != "acme/tool-ext" {
			t.Fatalf("installed extension = %#v, want marketplace provenance", installed)
		}

		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsDisable,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_disable) error = %v", err)
		}
		disabled, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(disabled) error = %v", err)
		}
		if disabled.Enabled {
			t.Fatal("extension enabled after disable = true, want false")
		}

		if _, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsEnable,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		); err != nil {
			t.Fatalf("Registry.Call(extensions_enable) error = %v", err)
		}

		source.latestVersion = "2.0.0"
		checkResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsUpdate,
				Input:  json.RawMessage(`{"name":"tool-ext","check_only":true}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_update check) error = %v", err)
		}
		requireNativeStructuredContains(t, checkResult, []byte(`"available"`))

		updateResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsUpdate,
				Input:  json.RawMessage(`{"name":"tool-ext","allow_unverified":true}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_update apply) error = %v", err)
		}
		requireNativeStructuredContains(t, updateResult, []byte(`"updated"`))
		updated, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(updated) error = %v", err)
		}
		if updated.Version != "2.0.0" {
			t.Fatalf("updated version = %q, want 2.0.0", updated.Version)
		}

		listResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{ToolID: toolspkg.ToolIDExtensionsList},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_list) error = %v", err)
		}
		requireNativeStructuredContains(t, listResult, []byte(`"tool-ext"`))

		infoResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInfo,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_info) error = %v", err)
		}
		requireNativeStructuredContains(t, infoResult, []byte(`"tool-ext"`))

		removeResult, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsRemove,
				Input:  json.RawMessage(`{"name":"tool-ext"}`),
			},
		)
		if err != nil {
			t.Fatalf("Registry.Call(extensions_remove) error = %v", err)
		}
		requireNativeStructuredContains(t, removeResult, []byte(`"removed"`))
		if _, err := extRegistry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("extension registry Get(after remove) error = %v, want ErrExtensionNotFound", err)
		}
		if runtime.reloadCount < 5 {
			t.Fatalf("reload count = %d, want lifecycle mutations to reload runtime", runtime.reloadCount)
		}
	})

	t.Run("Should derive curated digest authority from the catalog", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, source, runtime := newNativeExtensionToolDeps(t)
		archive := nativeExtensionTarGz(t, "1.0.0")
		digest := fmt.Sprintf("%x", sha256.Sum256(archive))
		source.downloads["1.0.0"] = &registrypkg.DownloadResult{
			Reader: io.NopCloser(bytes.NewReader(archive)), Slug: "acme/tool-ext", Version: "1.0.0",
			ContentSize: -1, ContentType: "application/gzip",
		}
		catalog := nativeExtensionCatalog{entry: &marketplacepkg.Entry{
			Kind: marketplacepkg.KindExtension, EntryID: "extension.acme.tool-ext",
			InstallSlug: "acme/tool-ext", Version: "1.0.0", DigestSHA256: digest, Tier: "official",
			Payload: json.RawMessage(
				`{"install_slug":"acme/tool-ext","repository":"https://github.com/acme/tool-ext"}`,
			),
		}}
		service := newDaemonExtensionService(
			extRegistry,
			runtime,
			nil,
			nil,
			nil,
			nil,
			nil,
			deps.HomePaths,
			nil,
			nil,
			withDaemonExtensionMarketplace(
				aghconfig.ExtensionsMarketplaceConfig{Registry: "github"},
				deps.ExtensionSources,
			),
			withDaemonExtensionCatalog(catalog),
			withDaemonExtensionEventWriter(deps.ExtensionEvents),
		)
		deps.Extensions = func() core.ExtensionService { return service }
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())

		result, err := registry.Call(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsInstall,
			Input:  json.RawMessage(`{"source":"marketplace","slug":"acme/tool-ext","registry":"github"}`),
		})
		if err != nil {
			t.Fatalf("Registry.Call(curated extension install) error = %v", err)
		}
		requireNativeStructuredContains(t, result, []byte(`"catalog_entry_id":"extension.acme.tool-ext"`))
		installed, err := extRegistry.Get("tool-ext")
		if err != nil {
			t.Fatalf("extension registry Get(curated) error = %v", err)
		}
		if installed.Provenance.CatalogEntryID != "extension.acme.tool-ext" ||
			installed.Provenance.ArchiveDigestSHA256 != digest ||
			installed.Provenance.ChecksumSHA256 == digest ||
			!installed.Provenance.ChecksumVerified {
			t.Fatalf("curated provenance = %#v, want separate verified archive and tree digests", installed.Provenance)
		}
		summaries, err := deps.ExtensionEvents.ListEventSummaries(t.Context(), store.EventSummaryQuery{
			Type: eventspkg.ExtensionDigestVerify,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries(digest verification) error = %v", err)
		}
		if len(summaries) != 1 ||
			summaries[0].Outcome != string(eventspkg.OutcomeSuccess) ||
			!bytes.Contains(summaries[0].Content, []byte(`"verification_outcome":"verified"`)) {
			t.Fatalf("digest verification summaries = %#v, want one verified event", summaries)
		}
	})

	t.Run("Should fail closed when catalog refresh and lookup both fail", func(t *testing.T) {
		t.Parallel()

		refreshErr := errors.New("catalog unavailable")
		catalog := nativeExtensionCatalog{err: &marketplacepkg.ExtensionInstallResolutionError{
			RefreshErr: refreshErr,
			LookupErr:  marketplacepkg.ErrEntryNotFound,
		}}
		service := &daemonExtensionService{marketplaceCatalog: catalog}
		trust, err := service.resolveMarketplaceExtensionTrust(t.Context(), "acme/tool-ext", "1.0.0")
		if trust != nil || !errors.Is(err, refreshErr) || !errors.Is(err, marketplacepkg.ErrEntryNotFound) {
			t.Fatalf("resolveMarketplaceExtensionTrust() = (%#v, %v), want fail-closed combined error", trust, err)
		}
	})

	t.Run("Should reject catalog digest mismatch even when request allows unverified", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, _, runtime := newNativeExtensionToolDeps(t)
		catalog := nativeExtensionCatalog{entry: &marketplacepkg.Entry{
			Kind: marketplacepkg.KindExtension, EntryID: "extension.acme.tool-ext",
			InstallSlug: "acme/tool-ext", Version: "1.0.0",
			DigestSHA256: strings.Repeat("0", sha256.Size*2), Tier: "official",
			Payload: json.RawMessage(
				`{"install_slug":"acme/tool-ext","repository":"https://github.com/acme/tool-ext"}`,
			),
		}}
		service := newDaemonExtensionService(
			extRegistry, runtime, nil, nil, nil, nil, nil, deps.HomePaths, nil, nil,
			withDaemonExtensionMarketplace(
				aghconfig.ExtensionsMarketplaceConfig{Registry: "github", AllowUnverified: true},
				deps.ExtensionSources,
			),
			withDaemonExtensionCatalog(catalog),
			withDaemonExtensionEventWriter(deps.ExtensionEvents),
		)
		deps.Extensions = func() core.ExtensionService { return service }
		registry := newDaemonNativeRegistry(t, deps, nativeApproveAllPolicyInputs())

		_, err := registry.Call(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDExtensionsInstall,
			Input: json.RawMessage(
				`{"source":"marketplace","slug":"acme/tool-ext","registry":"github","allow_unverified":true}`,
			),
		})
		requireToolReason(t, err, toolspkg.ErrToolInvalidInput, toolspkg.ReasonExtensionValidationFailed)
		if _, err := extRegistry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("extension registry Get(after digest mismatch) error = %v, want ErrExtensionNotFound", err)
		}
		summaries, listErr := deps.ExtensionEvents.ListEventSummaries(t.Context(), store.EventSummaryQuery{
			Type: eventspkg.ExtensionDigestVerify,
		})
		if listErr != nil {
			t.Fatalf("ListEventSummaries(digest mismatch) error = %v", listErr)
		}
		if len(summaries) != 1 ||
			summaries[0].Outcome != string(eventspkg.OutcomeFailure) ||
			!bytes.Contains(summaries[0].Content, []byte(`"verification_outcome":"mismatch"`)) {
			t.Fatalf("digest verification summaries = %#v, want one mismatch event", summaries)
		}
	})

	t.Run("Should require approval before extension mutations reach lifecycle dependencies", func(t *testing.T) {
		t.Parallel()

		deps, extRegistry, _, runtime := newNativeExtensionToolDeps(t)
		sourceCalls := 0
		deps.ExtensionSources = func(context.Context, aghconfig.ExtensionsMarketplaceConfig) ([]registrypkg.Source, error) {
			sourceCalls++
			return nil, errors.New("source should not be called")
		}
		registry := newDaemonNativeRegistry(t, deps, toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveReads,
			ApprovalAvailable:    false,
		})

		_, err := registry.Call(
			t.Context(),
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDExtensionsInstall,
				Input:  json.RawMessage(`{"source":"marketplace","slug":"acme/tool-ext"}`),
			},
		)
		if !errors.Is(err, toolspkg.ErrToolApprovalRequired) {
			t.Fatalf("Registry.Call(extensions_install approve-reads) error = %v, want approval required", err)
		}
		if sourceCalls != 0 {
			t.Fatalf("extension source calls = %d, want 0 before approval", sourceCalls)
		}
		if _, err := extRegistry.Get("tool-ext"); !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
			t.Fatalf("extension registry Get(after denied install) error = %v, want ErrExtensionNotFound", err)
		}
		if runtime.reloadCount != 0 {
			t.Fatalf("reload count after denied install = %d, want 0", runtime.reloadCount)
		}
	})
}

func newNativeExtensionToolDeps(
	t *testing.T,
) (*daemonNativeToolsDeps, *extensionpkg.Registry, *nativeExtensionSource, *fakeExtensionRuntime) {
	t.Helper()

	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	db, err := globaldb.OpenGlobalDB(t.Context(), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(context.Background()); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	})

	source := newNativeExtensionSource(t, "1.0.0", "2.0.0")
	runtime := &fakeExtensionRuntime{}
	extRegistry := extensionpkg.NewRegistry(db.DB())
	deps := daemonNativeToolsDeps{
		HomePaths:         homePaths,
		ExtensionRegistry: extRegistry,
		ExtensionRuntime: func() extensionRuntime {
			return runtime
		},
		ExtensionSources: func(context.Context, aghconfig.ExtensionsMarketplaceConfig) ([]registrypkg.Source, error) {
			return []registrypkg.Source{source}, nil
		},
		ExtensionMarket: aghconfig.ExtensionsMarketplaceConfig{Registry: "github", AllowUnverified: true},
		ExtensionEvents: db,
	}
	return &deps, extRegistry, source, runtime
}

func newNativeExtensionSource(t *testing.T, versions ...string) *nativeExtensionSource {
	t.Helper()

	downloads := make(map[string]*registrypkg.DownloadResult, len(versions))
	for _, version := range versions {
		downloads[version] = nativeExtensionDownloadResult(t, version)
	}
	return &nativeExtensionSource{
		latestVersion: versions[len(versions)-1],
		downloads:     downloads,
	}
}

func nativeExtensionDownloadResult(t *testing.T, version string) *registrypkg.DownloadResult {
	t.Helper()

	return &registrypkg.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(nativeExtensionTarGz(t, version))),
		Slug:        "acme/tool-ext",
		Version:     version,
		ContentSize: -1,
		ContentType: "application/gzip",
	}
}

func nativeExtensionTarGz(t *testing.T, version string) []byte {
	t.Helper()

	files := map[string]string{
		filepath.Join("tool-ext", "extension.toml"): fmt.Sprintf(`[extension]
name = "tool-ext"
version = %q
description = "Native tool test extension"
min_agh_version = "0.5.0"

[capabilities]
provides = ["memory.backend"]

[actions]
requires = ["sessions/list"]
`, version),
		filepath.Join("tool-ext", "VERSION.txt"): version + "\n",
	}

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

func derefNativeExtensionString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func writeNativeLocalExtensionFixture(t *testing.T, name string, version string) string {
	t.Helper()

	dir := t.TempDir()
	content := fmt.Sprintf(`[extension]
name = %q
version = %q
description = "Native local tool test extension"
min_agh_version = "0.5.0"

[capabilities]
provides = ["memory.backend"]

[actions]
requires = ["sessions/list"]
`, name, version)
	if err := os.WriteFile(filepath.Join(dir, "extension.toml"), []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(extension.toml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "VERSION.txt"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(VERSION.txt) error = %v", err)
	}
	return dir
}
