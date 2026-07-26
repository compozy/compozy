package cli

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	aghdaemon "github.com/compozy/agh/internal/daemon"
	extensionpkg "github.com/compozy/agh/internal/extension"
)

func TestExtensionSearchCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()

	called := false
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		browseMarketplaceFn: func(
			_ context.Context,
			kind string,
			query string,
			limit int,
			cursor string,
			scope MarketplaceReadScope,
		) (MarketplaceKindRecord, error) {
			called = true
			if kind != "extension" || query != "bridge" || limit != 7 {
				t.Fatalf("BrowseMarketplace(%q, %q, %d), want extension bridge 7", kind, query, limit)
			}
			if cursor != "" {
				t.Fatalf("BrowseMarketplace cursor = %q, want empty", cursor)
			}
			if scope.Scope != contract.SettingsWorkspaceScopeGlobal || scope.WorkspaceID != "" {
				t.Fatalf("Marketplace read scope = %#v, want global", scope)
			}
			return MarketplaceKindRecord{Kind: "extension", Items: []MarketplaceListingRecord{{
				Kind:        "extension",
				EntryID:     "bridge-ext",
				Name:        "bridge-ext",
				Description: "Bridge extension",
				Author:      "acme",
				Version:     "1.0.0",
				InstallSlug: "acme/bridge-ext",
				Source:      "registry",
			}}}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"search",
		"bridge",
		"--limit",
		"7",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension search error = %v", err)
	}
	if !called {
		t.Fatal("BrowseMarketplace was not called")
	}
	var payload []marketplaceExtensionSearchItem
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(search) error = %v; stdout=%s", err, stdout)
	}
	if len(payload) != 1 || payload[0].Slug != "acme/bridge-ext" {
		t.Fatalf("search payload = %#v, want bridge-ext", payload)
	}
}

func TestExtensionSearchCommandRejectsNonPositiveLimit(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	markExtensionDaemonRunning(&deps)
	_, _, err := executeRootCommand(t, deps, "extension", "search", "bridge", "--limit", "0")
	if err == nil || !strings.Contains(err.Error(), "search limit must be positive") {
		t.Fatalf("extension search --limit=0 error = %v, want limit validation", err)
	}
}

func TestExtensionInstallCommandUsesDaemonForMarketplaceInstalls(t *testing.T) {
	t.Parallel()

	var captured InstallExtensionRequest
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		installExtensionFn: func(_ context.Context, request InstallExtensionRequest) (ExtensionRecord, error) {
			captured = request
			return ExtensionRecord{
				Name:          "install-ext",
				Version:       "1.2.0",
				Source:        extensionpkg.SourceMarketplace.String(),
				State:         "active",
				DaemonRunning: true,
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		"acme/install-ext",
		"--from",
		"github",
		"--version",
		"1.2.0",
		"--asset",
		"darwin-arm64",
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension install marketplace error = %v", err)
	}
	want := InstallExtensionRequest{
		Slug:            "acme/install-ext",
		Source:          "github",
		Version:         "1.2.0",
		Asset:           "darwin-arm64",
		AllowUnverified: true,
	}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("InstallExtension request = %#v, want %#v", captured, want)
	}
	var payload ExtensionRecord
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(install) error = %v; stdout=%s", err, stdout)
	}
	if payload.Name != "install-ext" || payload.Source != extensionpkg.SourceMarketplace.String() {
		t.Fatalf("install payload = %#v, want marketplace extension", payload)
	}
}

func TestExtensionInstallUsesReachableDaemonWhenProcessTimestampMetadataLags(t *testing.T) {
	t.Parallel()

	installCalled := false
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		daemonStatusFn: func(context.Context) (DaemonStatus, error) {
			return DaemonStatus{Status: "running", PID: 999}, nil
		},
		installExtensionFn: func(_ context.Context, request InstallExtensionRequest) (ExtensionRecord, error) {
			installCalled = true
			return ExtensionRecord{
				Name:          "late-boot-ext",
				Version:       request.Version,
				Source:        extensionpkg.SourceMarketplace.String(),
				State:         "active",
				DaemonRunning: true,
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)
	deps.processMatchesStartTime = func(int, time.Time) bool { return false }

	_, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"install",
		"acme/late-boot-ext",
		"--version",
		"1.0.0",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension install marketplace error = %v", err)
	}
	if !installCalled {
		t.Fatal("InstallExtension was not called through the reachable daemon")
	}
}

func TestExtensionMarketplaceInstallRequiresDaemon(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	_, _, err := executeRootCommand(t, deps, "extension", "install", "acme/install-ext", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "running daemon") {
		t.Fatalf("extension install marketplace offline error = %v, want running daemon requirement", err)
	}
}

func TestExtensionAllowUnverifiedStructuredOutputRequiresYes(t *testing.T) {
	t.Parallel()

	deps, _ := newExtensionLocalDeps(t, &stubClient{})
	dir := writeExtensionFixture(t, "trust-ext", extensionFixtureOptions{})
	_, _, err := executeRootCommand(t, deps, "extension", "install", dir, "--allow-unverified", "-o", "json")
	if err == nil || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("extension install --allow-unverified without --yes error = %v, want --yes requirement", err)
	}
}

func TestExtensionRemoveCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()

	var removedName string
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		removeExtensionFn: func(_ context.Context, name string) (ManagedExtensionRemoveRecord, error) {
			removedName = name
			return ManagedExtensionRemoveRecord{Name: name, Path: "/tmp/remove-ext", Status: "removed"}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(t, deps, "extension", "remove", "remove-ext", "-o", "json")
	if err != nil {
		t.Fatalf("extension remove error = %v", err)
	}
	if removedName != "remove-ext" {
		t.Fatalf("RemoveExtension name = %q, want remove-ext", removedName)
	}
	var payload ManagedExtensionRemoveRecord
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(remove) error = %v; stdout=%s", err, stdout)
	}
	if payload.Status != "removed" {
		t.Fatalf("remove payload = %#v, want removed", payload)
	}
}

func TestExtensionUpdateCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()

	var capturedName string
	var capturedRequest UpdateExtensionRequest
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		extensionStatusFn: func(_ context.Context, name string) (ExtensionRecord, error) {
			return marketplaceExtensionRecord(name, "1.0.0"), nil
		},
		updateExtensionFn: func(
			_ context.Context,
			name string,
			request UpdateExtensionRequest,
		) (ExtensionUpdateRecord, error) {
			capturedName = name
			capturedRequest = request
			return ExtensionUpdateRecord{
				Name:           name,
				Slug:           "acme/" + name,
				Registry:       "github",
				CurrentVersion: "1.0.0",
				LatestVersion:  "1.2.0",
				Path:           "/tmp/" + name,
				Status:         extensionpkg.MarketplaceUpdateStatusUpdated,
				Warnings: []contract.DiagnosticItem{{
					Code: contract.CodeExtensionUpdateCleanupFailed,
				}},
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"update",
		"update-ext",
		"--version",
		"1.2.0",
		"--allow-unverified",
		"--yes",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension update error = %v", err)
	}
	if capturedName != "update-ext" {
		t.Fatalf("UpdateExtension name = %q, want update-ext", capturedName)
	}
	want := UpdateExtensionRequest{Version: "1.2.0", AllowUnverified: true}
	if !reflect.DeepEqual(capturedRequest, want) {
		t.Fatalf("UpdateExtension request = %#v, want %#v", capturedRequest, want)
	}
	var payload []extensionUpdateItem
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(update) error = %v; stdout=%s", err, stdout)
	}
	if len(payload) != 1 || payload[0].Status != extensionpkg.MarketplaceUpdateStatusUpdated {
		t.Fatalf("update payload = %#v, want updated", payload)
	}
	if len(payload[0].Warnings) != 1 || payload[0].Warnings[0].Code != contract.CodeExtensionUpdateCleanupFailed {
		t.Fatalf("update payload warnings = %#v, want cleanup warning", payload[0].Warnings)
	}
	humanOut, _, err := executeRootCommand(t, deps, "extension", "update", "update-ext")
	if err != nil {
		t.Fatalf("extension update human output error = %v", err)
	}
	if !strings.Contains(humanOut, contract.CodeExtensionUpdateCleanupFailed) {
		t.Fatalf("extension update human output = %q, want cleanup warning code", humanOut)
	}
}

func TestExtensionUpdateCheckDoesNotRequireAllowUnverified(t *testing.T) {
	t.Parallel()

	var captured UpdateExtensionRequest
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		extensionStatusFn: func(_ context.Context, name string) (ExtensionRecord, error) {
			return marketplaceExtensionRecord(name, "1.0.0"), nil
		},
		updateExtensionFn: func(
			_ context.Context,
			name string,
			request UpdateExtensionRequest,
		) (ExtensionUpdateRecord, error) {
			captured = request
			return ExtensionUpdateRecord{
				Name:           name,
				Slug:           "acme/" + name,
				Registry:       "github",
				CurrentVersion: "1.0.0",
				LatestVersion:  "1.2.0",
				Status:         extensionpkg.MarketplaceUpdateStatusAvailable,
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	stdout, _, err := executeRootCommand(t, deps, "extension", "update", "update-ext", "--check", "-o", "json")
	if err != nil {
		t.Fatalf("extension update --check error = %v", err)
	}
	want := UpdateExtensionRequest{CheckOnly: true}
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("UpdateExtension check request = %#v, want %#v", captured, want)
	}
	var payload []extensionUpdateItem
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(update check) error = %v; stdout=%s", err, stdout)
	}
	if len(payload) != 1 || payload[0].Status != extensionpkg.MarketplaceUpdateStatusAvailable {
		t.Fatalf("update check payload = %#v, want available", payload)
	}
}

func TestExtensionUpdateAllSkipsLocalExtensions(t *testing.T) {
	t.Parallel()

	updated := []string{}
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		listExtensionsFn: func(context.Context) ([]ExtensionRecord, error) {
			return []ExtensionRecord{
				marketplaceExtensionRecord("market-ext", "1.0.0"),
				{Name: "local-ext", Version: "1.0.0", Source: extensionpkg.SourceUser.String()},
			}, nil
		},
		updateExtensionFn: func(
			_ context.Context,
			name string,
			request UpdateExtensionRequest,
		) (ExtensionUpdateRecord, error) {
			if !request.CheckOnly {
				t.Fatalf("UpdateExtension request.CheckOnly = false, want true")
			}
			updated = append(updated, name)
			return ExtensionUpdateRecord{
				Name:           name,
				Slug:           "acme/" + name,
				Registry:       "github",
				CurrentVersion: "1.0.0",
				LatestVersion:  "1.2.0",
				Status:         extensionpkg.MarketplaceUpdateStatusAvailable,
			}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	if _, _, err := executeRootCommand(t, deps, "extension", "update", "--all", "--check", "-o", "json"); err != nil {
		t.Fatalf("extension update --all --check error = %v", err)
	}
	if !reflect.DeepEqual(updated, []string{"market-ext"}) {
		t.Fatalf("updated names = %#v, want market-ext only", updated)
	}
}

func markExtensionDaemonRunning(deps *commandDeps) {
	deps.readDaemonInfo = func(string) (aghdaemon.Info, error) {
		return aghdaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }
}

func marketplaceExtensionRecord(name string, version string) ExtensionRecord {
	return ExtensionRecord{
		Name:    name,
		Version: version,
		Source:  extensionpkg.SourceMarketplace.String(),
		Provenance: &contract.ExtensionProvenancePayload{
			Slug:          "acme/" + name,
			InstalledFrom: extensionpkg.ExtensionInstalledFromMarketplace,
			RegistryTier:  extensionpkg.ExtensionRegistryTierCommunity,
		},
	}
}
