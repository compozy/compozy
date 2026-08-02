package cli

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func TestExtensionSearchCommandUsesDaemonClient(t *testing.T) {
	t.Parallel()

	called := false
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		searchExtensionsFn: func(
			_ context.Context,
			request ExtensionSearchRequest,
		) (ExtensionSearchRecord, error) {
			called = true
			if request.Query != "bridge" || request.Limit != 7 {
				t.Fatalf("SearchExtensions(%#v), want bridge limit 7", request)
			}
			if request.Cursor != "next" {
				t.Fatalf("SearchExtensions cursor = %q, want next", request.Cursor)
			}
			if !reflect.DeepEqual(request.Sources, []string{"curated", "github"}) {
				t.Fatalf("SearchExtensions sources = %#v, want curated/github", request.Sources)
			}
			return ExtensionSearchRecord{
				Items: []contract.ExtensionSearchItem{{
					Slug:        "acme/bridge-ext",
					Name:        "bridge-ext",
					Description: "Bridge extension",
					Author:      "acme",
					Version:     "1.0.0",
					Source:      "curated",
					Tier:        "official",
					Integrity:   "catalog_digest",
				}},
				NextCursor:      "page-two",
				SourcesDegraded: []string{"github"},
			}, nil
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
		"--cursor",
		"next",
		"-o",
		"json",
	)
	if err != nil {
		t.Fatalf("extension search error = %v", err)
	}
	if !called {
		t.Fatal("SearchExtensions was not called")
	}
	var payload ExtensionSearchRecord
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("json.Unmarshal(search) error = %v; stdout=%s", err, stdout)
	}
	if len(payload.Items) != 1 || payload.Items[0].Slug != "acme/bridge-ext" ||
		payload.NextCursor != "page-two" || !reflect.DeepEqual(payload.SourcesDegraded, []string{"github"}) {
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
		"github:acme/install-ext",
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
		Source:          contract.InstallExtensionSourceGitHub,
		Ref:             "acme/install-ext",
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

	stdout, _, err := executeRootCommand(
		t,
		deps,
		"extension",
		"remove",
		"remove-ext",
		"--global",
		"-o",
		"json",
	)
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

	var capturedName string
	var captured UpdateExtensionRequest
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		updateExtensionFn: func(
			_ context.Context,
			name string,
			request UpdateExtensionRequest,
		) (ExtensionUpdateRecord, error) {
			capturedName = name
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
	if capturedName != "update-ext" {
		t.Fatalf("UpdateExtension name = %q, want update-ext", capturedName)
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

func TestExtensionUpdateAllUsesOneDaemonBatch(t *testing.T) {
	t.Parallel()

	calls := 0
	deps, _ := newExtensionLocalDeps(t, &stubClient{
		updateExtensionsFn: func(
			_ context.Context,
			request UpdateExtensionsRequest,
		) ([]ExtensionUpdateRecord, error) {
			calls++
			if !request.All || !request.CheckOnly || len(request.Names) != 0 {
				t.Fatalf("UpdateExtensions request = %#v, want all check-only", request)
			}
			return []ExtensionUpdateRecord{{
				Name:           "market-ext",
				Slug:           "acme/market-ext",
				Registry:       "github",
				CurrentVersion: "1.0.0",
				LatestVersion:  "1.2.0",
				Status:         extensionpkg.MarketplaceUpdateStatusAvailable,
			}, {
				Name: "failed-ext", Status: extensionpkg.MarketplaceUpdateStatusFailed,
			}}, nil
		},
	})
	markExtensionDaemonRunning(&deps)

	if _, _, err := executeRootCommand(t, deps, "extension", "update", "--all", "--check", "-o", "json"); err != nil {
		t.Fatalf("extension update --all --check error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("UpdateExtensions calls = %d, want one batch", calls)
	}
}

func markExtensionDaemonRunning(deps *commandDeps) {
	deps.readDaemonInfo = func(string) (compozydaemon.Info, error) {
		return compozydaemon.Info{PID: 999, StartedAt: fixedTestNow}, nil
	}
	deps.processAlive = func(int) bool { return true }
}
