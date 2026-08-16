package update

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"time"
)

func TestManagerCheck(t *testing.T) {
	t.Run("Should use the cached release snapshot while it is still fresh", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 3, 21, 0, 0, 0, time.UTC)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			RuntimeOS:   runtimeOSLinux,
			RuntimeArch: runtimeArchAMD64,
			Now: func() time.Time {
				return now
			},
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					requests.Add(1)
					return nil, errors.New("unexpected refresh")
				}),
			},
		})

		checkedAt := now.Add(-2 * time.Hour)
		err := writeCache(manager.cachePath(), testCacheEntry(
			t,
			manager,
			"v1.1.0",
			"https://github.com/compozy/compozy/releases/tag/v1.1.0",
			checkedAt,
		))
		if err != nil {
			t.Fatalf("writeCache() error = %v", err)
		}

		state, release, err := manager.Check(context.Background(), CheckOptions{})
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if requests.Load() != 0 {
			t.Fatalf("refresh requests = %d, want 0", requests.Load())
		}
		if release == nil || release.Version != "v1.1.0" {
			t.Fatalf("release = %#v, want cached v1.1.0", release)
		}
		if state.Status != StatusAvailable || !state.Available {
			t.Fatalf("state = %#v, want available cached snapshot", state)
		}
		if state.CheckedAt == nil || !state.CheckedAt.Equal(checkedAt) {
			t.Fatalf("state.CheckedAt = %v, want %s", state.CheckedAt, checkedAt)
		}
	})

	t.Run("Should preserve applyable release metadata across cache hits", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 3, 21, 30, 0, 0, time.UTC)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			RuntimeOS:   runtimeOSLinux,
			RuntimeArch: runtimeArchAMD64,
			Now: func() time.Time {
				return now
			},
		})
		archiveName, err := archiveAssetName(manager.runtimeOS, manager.runtimeArch)
		if err != nil {
			t.Fatalf("archiveAssetName() error = %v", err)
		}
		manager.httpClient = &http.Client{
			Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
				requests.Add(1)
				return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
					TagName:     "v1.1.0",
					HTMLURL:     "https://github.com/compozy/compozy/releases/tag/v1.1.0",
					PublishedAt: now.Add(-time.Hour),
					Assets: []githubAssetResponse{
						{Name: archiveName, BrowserDownloadURL: "https://downloads.example/archive"},
						{Name: checksumsAssetName, BrowserDownloadURL: "https://downloads.example/checksums.txt"},
						{
							Name:               checksumsBundleAssetName,
							BrowserDownloadURL: "https://downloads.example/checksums.txt.sigstore.json",
						},
						{Name: compatibilityAssetName, BrowserDownloadURL: "https://downloads.example/compat.json"},
					},
				}), nil
			}),
		}

		_, freshRelease, err := manager.Check(context.Background(), CheckOptions{})
		if err != nil {
			t.Fatalf("Check(fresh) error = %v", err)
		}
		if _, err := manager.resolveReleaseAssets(freshRelease); err != nil {
			t.Fatalf("resolveReleaseAssets(fresh) error = %v", err)
		}
		_, cachedRelease, err := manager.Check(context.Background(), CheckOptions{})
		if err != nil {
			t.Fatalf("Check(cached) error = %v", err)
		}
		if got := requests.Load(); got != 1 {
			t.Fatalf("release refresh requests = %d, want 1", got)
		}
		if _, err := manager.resolveReleaseAssets(cachedRelease); err != nil {
			t.Fatalf("resolveReleaseAssets(cached) error = %v", err)
		}
	})

	t.Run("Should refresh stale cache entries and persist the new latest release", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 3, 22, 0, 0, 0, time.UTC)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			Now: func() time.Time {
				return now
			},
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					requests.Add(1)
					return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
						TagName:     "v1.2.0",
						HTMLURL:     "https://github.com/compozy/compozy/releases/tag/v1.2.0",
						PublishedAt: now.Add(-time.Hour),
						Assets:      testGitHubAssets(t, nil),
					}), nil
				}),
			},
		})

		err := writeCache(manager.cachePath(), testCacheEntry(
			t,
			manager,
			"v1.1.0",
			"https://github.com/compozy/compozy/releases/tag/v1.1.0",
			now.Add(-(cacheTTL+time.Hour)),
		))
		if err != nil {
			t.Fatalf("writeCache() error = %v", err)
		}

		state, release, err := manager.Check(context.Background(), CheckOptions{})
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if requests.Load() != 1 {
			t.Fatalf("refresh requests = %d, want 1", requests.Load())
		}
		if release == nil || release.Version != "v1.2.0" {
			t.Fatalf("release = %#v, want refreshed v1.2.0", release)
		}
		if state.LatestVersion != "v1.2.0" || state.Status != StatusAvailable {
			t.Fatalf("state = %#v, want refreshed available snapshot", state)
		}

		cached, err := readCache(manager.cachePath())
		if err != nil {
			t.Fatalf("readCache() error = %v", err)
		}
		if cached.LatestVersion != "v1.2.0" {
			t.Fatalf("cached.LatestVersion = %q, want %q", cached.LatestVersion, "v1.2.0")
		}
	})

	t.Run(
		"Should fall back to the cached snapshot when refresh fails and cached fallback is allowed",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 5, 3, 23, 0, 0, 0, time.UTC)
			manager, _ := newManagerWithExecutable(t, &Config{
				Now: func() time.Time {
					return now
				},
				HTTPClient: &http.Client{
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						return nil, errors.New("network unavailable")
					}),
				},
			})

			err := writeCache(manager.cachePath(), testCacheEntry(
				t,
				manager,
				"v1.1.0",
				"https://github.com/compozy/compozy/releases/tag/v1.1.0",
				now.Add(-(cacheTTL+time.Hour)),
			))
			if err != nil {
				t.Fatalf("writeCache() error = %v", err)
			}

			state, release, err := manager.Check(context.Background(), CheckOptions{
				AllowCachedOnFailure: true,
			})
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}
			if release == nil || release.Version != "v1.1.0" {
				t.Fatalf("release = %#v, want cached release on refresh failure", release)
			}
			if state.Status != StatusAvailable || !state.Available {
				t.Fatalf("state = %#v, want cached available snapshot", state)
			}
			if !strings.Contains(state.LastError, "network unavailable") {
				t.Fatalf("state.LastError = %q, want refresh failure detail", state.LastError)
			}
		},
	)

	t.Run("Should refuse self-update for dev builds", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, &Config{
			CurrentVersion: "dev",
		})

		state := manager.composeState(
			installInfo{Method: string(InstallMethodDirectBinary)},
			&Release{Version: "v1.1.0"},
			nil,
		)
		if state.Status != StatusUnsupported {
			t.Fatalf("state.Status = %q, want %q", state.Status, StatusUnsupported)
		}
		if !strings.Contains(state.Recommendation, "tagged CompozyOS release") {
			t.Fatalf("state.Recommendation = %q, want tagged release guidance", state.Recommendation)
		}
	})

	t.Run("Should skip release refresh for untagged builds", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			CurrentVersion: "25bd6116",
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					requests.Add(1)
					return nil, errors.New("release refresh should not run")
				}),
			},
		})

		state, release, err := manager.Check(context.Background(), CheckOptions{})
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if requests.Load() != 0 {
			t.Fatalf("refresh requests = %d, want 0", requests.Load())
		}
		if release != nil {
			t.Fatalf("release = %#v, want nil for untagged build", release)
		}
		if state.Status != StatusUnsupported {
			t.Fatalf("state.Status = %q, want %q", state.Status, StatusUnsupported)
		}
		if state.LastError != "" {
			t.Fatalf("state.LastError = %q, want empty", state.LastError)
		}
	})

	t.Run("Should reject prerelease metadata from the latest-release endpoint", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, &Config{
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
						TagName:    "v1.2.0-rc.1",
						HTMLURL:    "https://github.com/compozy/compozy/releases/tag/v1.2.0-rc.1",
						Prerelease: true,
					}), nil
				}),
			},
		})

		_, err := manager.fetchLatestRelease(context.Background())
		if err == nil {
			t.Fatal("fetchLatestRelease() error = nil, want prerelease rejection")
		}
		if !strings.Contains(err.Error(), "not a stable release") {
			t.Fatalf("fetchLatestRelease() error = %v, want stable-release validation", err)
		}
	})

	t.Run("Should select the newest beta from the current release line", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
		manager, _ := newManagerWithExecutable(t, &Config{
			CurrentVersion: "v0.3.0-beta.1",
			Now:            func() time.Time { return now },
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if got := request.URL.String(); got != githubReleasesAPIURL {
						t.Fatalf("release URL = %q, want %q", got, githubReleasesAPIURL)
					}
					return jsonHTTPResponse(t, http.StatusOK, []githubReleaseResponse{
						{
							TagName:     "v0.2.15",
							HTMLURL:     "https://github.com/compozy/compozy/releases/tag/v0.2.15",
							PublishedAt: now.Add(-5 * time.Hour),
						},
						{
							TagName:     "v0.3.0-beta.2",
							HTMLURL:     "https://github.com/compozy/compozy/releases/tag/v0.3.0-beta.2",
							Prerelease:  true,
							PublishedAt: now.Add(-4 * time.Hour),
							Assets:      testGitHubAssets(t, nil),
						},
						{
							TagName:     "v0.4.0-beta.9",
							Prerelease:  true,
							PublishedAt: now.Add(-3 * time.Hour),
						},
						{
							TagName:     "v0.3.0-rc.1",
							Prerelease:  true,
							PublishedAt: now.Add(-2 * time.Hour),
						},
						{
							TagName:     "v0.3.0-beta.3",
							Draft:       true,
							Prerelease:  true,
							PublishedAt: now.Add(-time.Hour),
						},
					}), nil
				}),
			},
		})

		state, release, err := manager.Check(context.Background(), CheckOptions{ForceRefresh: true})
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if release == nil || release.Version != "v0.3.0-beta.2" {
			t.Fatalf("release = %#v, want current-line beta.2", release)
		}
		if state.Status != StatusAvailable || !state.Available {
			t.Fatalf("state = %#v, want available beta update", state)
		}
		if !strings.HasSuffix(manager.cachePath(), "update-state-beta.json") {
			t.Fatalf("cachePath() = %q, want beta-specific cache", manager.cachePath())
		}
	})

	t.Run("Should mark Windows direct-binary installs as manual-only", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, &Config{
			RuntimeOS:   runtimeOSWindows,
			RuntimeArch: runtimeArchAMD64,
		})

		state := manager.composeState(
			installInfo{Method: string(InstallMethodDirectBinary)},
			&Release{
				Version:    "v1.1.0",
				ReleaseURL: "https://github.com/compozy/compozy/releases/tag/v1.1.0",
			},
			nil,
		)
		if state.Status != StatusUnsupported {
			t.Fatalf("state.Status = %q, want %q", state.Status, StatusUnsupported)
		}
		if !strings.Contains(state.Recommendation, "compozy.exe") {
			t.Fatalf("state.Recommendation = %q, want Windows manual guidance", state.Recommendation)
		}
	})

	t.Run("Should defer managed npm and go-install updates with package-specific guidance", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, &Config{})
		tests := []struct {
			name   string
			method InstallMethod
			want   string
		}{
			{
				name:   "Should recommend npm global update for npm installs",
				method: InstallMethodNPM,
				want:   "npm update -g @compozy/cli",
			},
			{
				name:   "Should recommend public go install path for Go installs",
				method: InstallMethodGoInstall,
				want:   "go install github.com/compozy/compozy@latest",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				state := manager.composeState(
					installInfo{Method: string(tc.method), Managed: true},
					&Release{Version: "v1.1.0"},
					nil,
				)
				if state.Status != StatusAvailable || !state.Managed {
					t.Fatalf("state = %#v, want deferred managed update", state)
				}
				if !strings.Contains(state.Recommendation, tc.want) {
					t.Fatalf("state.Recommendation = %q, want %q", state.Recommendation, tc.want)
				}
			})
		}
	})

	t.Run("Should self-apply a desktop-owned runtime", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, &Config{})
		state := manager.composeState(
			installInfo{Method: string(InstallMethodDesktopApp)},
			&Release{Version: "v1.1.0"},
			nil,
		)
		if !state.Supported || state.Managed || state.Status != StatusAvailable ||
			state.Recommendation != "Run `compozy update`." {
			t.Fatalf("desktop state = %#v, want supported self-apply", state)
		}
	})

	t.Run("Should keep beta recommendations on beta-capable distribution paths", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, &Config{CurrentVersion: "v0.3.0-beta.1"})
		tests := []struct {
			name      string
			method    InstallMethod
			want      string
			forbidden string
		}{
			{
				name:   "Should recommend the npm beta tag",
				method: InstallMethodNPM,
				want:   "npm install -g @compozy/cli@beta",
			},
			{
				name:   "Should pin the exact beta for Go installs",
				method: InstallMethodGoInstall,
				want:   "go install github.com/compozy/compozy@v0.3.0-beta.2",
			},
			{
				name:      "Should route Homebrew beta users to the hosted installer",
				method:    InstallMethodHomebrew,
				want:      "curl -fsSL https://compozy.com/install.sh | sh",
				forbidden: "brew ",
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				state := manager.composeState(
					installInfo{Method: string(tc.method), Managed: true},
					&Release{Version: "v0.3.0-beta.2"},
					nil,
				)
				if state.Status != StatusAvailable || !state.Managed {
					t.Fatalf("state = %#v, want deferred managed beta update", state)
				}
				if !strings.Contains(state.Recommendation, tc.want) {
					t.Fatalf("state.Recommendation = %q, want %q", state.Recommendation, tc.want)
				}
				if tc.forbidden != "" && strings.Contains(state.Recommendation, tc.forbidden) {
					t.Fatalf("state.Recommendation = %q, must not contain %q", state.Recommendation, tc.forbidden)
				}
			})
		}
	})

	t.Run("Should suppress automatic app checks after an archived installer failure", func(t *testing.T) {
		t.Parallel()

		archived := &Operation{
			LastError: "installer exited before relaunch",
			App: &AppOperationState{
				ArtifactIdentity:    ArtifactIdentity{ToVersion: "v1.2.0"},
				AttemptID:           "attempt-2",
				Phase:               PhaseFailed,
				ConsecutiveFailures: 2,
			},
		}
		background := &AppTrackState{
			Status: StatusAvailable, CurrentVersion: "v1.1.0", LatestVersion: "v1.2.0",
		}
		applyArchivedAppFailure(background, archived, false)
		if background.Status != StatusFailed || background.AttemptID != "attempt-2" ||
			background.LastError != archived.LastError || !strings.Contains(background.Message, "paused") {
			t.Fatalf("background app state = %#v, want archived failure suppression", background)
		}

		manual := &AppTrackState{
			Status: StatusAvailable, CurrentVersion: "v1.1.0", LatestVersion: "v1.2.0",
		}
		applyArchivedAppFailure(manual, archived, true)
		if manual.Status != StatusAvailable || manual.AttemptID != "" || manual.LastError != "" {
			t.Fatalf("manual app state = %#v, want recovery to remain available", manual)
		}

		updated := &AppTrackState{
			Status: StatusUpToDate, CurrentVersion: "v1.2.0", LatestVersion: "v1.2.0",
		}
		applyArchivedAppFailure(updated, archived, false)
		if updated.Status != StatusUpToDate || updated.AttemptID != "" || updated.LastError != "" {
			t.Fatalf("updated app state = %#v, want stale failure ignored", updated)
		}
		if got := consecutiveAppFailures("v1.1.0", archived); got != 2 {
			t.Fatalf("consecutiveAppFailures(old version) = %d, want 2", got)
		}
		if got := consecutiveAppFailures("v1.2.0", archived); got != 0 {
			t.Fatalf("consecutiveAppFailures(updated version) = %d, want 0", got)
		}
	})

	t.Run("Should select only the platform-supported desktop updater asset", func(t *testing.T) {
		t.Parallel()

		release := &Release{
			Version: "v1.2.0",
			Assets: []ReleaseAsset{
				{Name: "CompozyOS-1.2.0-darwin-arm64.dmg"},
				{Name: "CompozyOS-1.2.0-darwin-arm64.zip"},
				{Name: "CompozyOS-1.2.0-linux-arm64.AppImage"},
			},
		}
		mac, _ := newManagerWithExecutable(t, &Config{RuntimeOS: runtimeOSDarwin, RuntimeArch: runtimeArchARM64})
		macAsset, err := mac.resolveAppReleaseAsset(release)
		if err != nil {
			t.Fatalf("resolveAppReleaseAsset(macOS) error = %v", err)
		}
		if macAsset.Name != "CompozyOS-1.2.0-darwin-arm64.zip" {
			t.Fatalf("resolveAppReleaseAsset(macOS) = %q, want zip", macAsset.Name)
		}

		linux, _ := newManagerWithExecutable(t, &Config{RuntimeOS: runtimeOSLinux, RuntimeArch: runtimeArchARM64})
		linuxAsset, err := linux.resolveAppReleaseAsset(release)
		if err != nil {
			t.Fatalf("resolveAppReleaseAsset(Linux) error = %v", err)
		}
		if linuxAsset.Name != "CompozyOS-1.2.0-linux-arm64.AppImage" {
			t.Fatalf("resolveAppReleaseAsset(Linux) = %q, want AppImage", linuxAsset.Name)
		}

		windows, _ := newManagerWithExecutable(t, &Config{RuntimeOS: runtimeOSWindows, RuntimeArch: runtimeArchAMD64})
		if _, err := windows.resolveAppReleaseAsset(release); err == nil {
			t.Fatal("resolveAppReleaseAsset(Windows) error = nil, want unsupported")
		}
	})
}

func TestManagerFetchGitHubJSONCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should bound early response draining and preserve status and close failures", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close response")
		body := &updateReadCloserProbe{
			reader:   strings.NewReader(strings.Repeat("x", int(maxUpdateResponseDrainBytes)+1)),
			closeErr: closeErr,
		}
		manager, _ := newManagerWithExecutable(t, &Config{
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Status:     "500 Internal Server Error",
						Body:       body,
					}, nil
				}),
			},
		})

		var release githubReleaseResponse
		err := manager.fetchGitHubJSON(context.Background(), "https://example.invalid/releases", &release)
		if err == nil {
			t.Fatal("fetchGitHubJSON() error = nil, want status and close failures")
		}
		if !strings.Contains(err.Error(), "500 Internal Server Error") {
			t.Fatalf("fetchGitHubJSON() error = %v, want status failure", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("fetchGitHubJSON() error = %v, want close failure", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false, want true")
		}
		if body.read != maxUpdateResponseDrainBytes {
			t.Fatalf("response body bytes drained = %d, want %d", body.read, maxUpdateResponseDrainBytes)
		}
	})

	t.Run("Should join early response drain and close failures", func(t *testing.T) {
		t.Parallel()

		drainErr := errors.New("drain response")
		closeErr := errors.New("close response")
		body := &updateReadCloserProbe{
			reader:   iotest.ErrReader(drainErr),
			closeErr: closeErr,
		}
		manager, _ := newManagerWithExecutable(t, &Config{
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: http.StatusInternalServerError,
						Status:     "500 Internal Server Error",
						Body:       body,
					}, nil
				}),
			},
		})

		var release githubReleaseResponse
		err := manager.fetchGitHubJSON(context.Background(), "https://example.invalid/releases", &release)
		if err == nil {
			t.Fatal("fetchGitHubJSON() error = nil, want status, drain, and close failures")
		}
		if !errors.Is(err, drainErr) {
			t.Fatalf("fetchGitHubJSON() error = %v, want drain failure", err)
		}
		if !errors.Is(err, closeErr) {
			t.Fatalf("fetchGitHubJSON() error = %v, want close failure", err)
		}
		if !body.closed {
			t.Fatal("response body closed = false, want true")
		}
	})
}
