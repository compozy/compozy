package update

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestManagerCheck(t *testing.T) {
	t.Run("Should use the cached release snapshot while it is still fresh", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 5, 3, 21, 0, 0, 0, time.UTC)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, Config{
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
			"https://github.com/compozy/agh/releases/tag/v1.1.0",
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
		manager, _ := newManagerWithExecutable(t, Config{
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
					HTMLURL:     "https://github.com/compozy/agh/releases/tag/v1.1.0",
					PublishedAt: now.Add(-time.Hour),
					Assets: []githubAssetResponse{
						{Name: archiveName, BrowserDownloadURL: "https://downloads.example/archive"},
						{Name: checksumsAssetName, BrowserDownloadURL: "https://downloads.example/checksums.txt"},
						{
							Name:               checksumsBundleAssetName,
							BrowserDownloadURL: "https://downloads.example/checksums.txt.sigstore.json",
						},
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
		manager, _ := newManagerWithExecutable(t, Config{
			Now: func() time.Time {
				return now
			},
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					requests.Add(1)
					return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
						TagName:     "v1.2.0",
						HTMLURL:     "https://github.com/compozy/agh/releases/tag/v1.2.0",
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
			"https://github.com/compozy/agh/releases/tag/v1.1.0",
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
			manager, _ := newManagerWithExecutable(t, Config{
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
				"https://github.com/compozy/agh/releases/tag/v1.1.0",
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

		manager, _ := newManagerWithExecutable(t, Config{
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
		if !strings.Contains(state.Recommendation, "tagged AGH release") {
			t.Fatalf("state.Recommendation = %q, want tagged release guidance", state.Recommendation)
		}
	})

	t.Run("Should skip release refresh for untagged builds", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, Config{
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

		manager, _ := newManagerWithExecutable(t, Config{
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
					return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
						TagName:    "v1.2.0-rc.1",
						HTMLURL:    "https://github.com/compozy/agh/releases/tag/v1.2.0-rc.1",
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

	t.Run("Should mark Windows direct-binary installs as manual-only", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, Config{
			RuntimeOS:   runtimeOSWindows,
			RuntimeArch: runtimeArchAMD64,
		})

		state := manager.composeState(
			installInfo{Method: string(InstallMethodDirectBinary)},
			&Release{
				Version:    "v1.1.0",
				ReleaseURL: "https://github.com/compozy/agh/releases/tag/v1.1.0",
			},
			nil,
		)
		if state.Status != StatusUnsupported {
			t.Fatalf("state.Status = %q, want %q", state.Status, StatusUnsupported)
		}
		if !strings.Contains(state.Recommendation, "agh.exe") {
			t.Fatalf("state.Recommendation = %q, want Windows manual guidance", state.Recommendation)
		}
	})

	t.Run("Should defer managed npm and go-install updates with package-specific guidance", func(t *testing.T) {
		t.Parallel()

		manager, _ := newManagerWithExecutable(t, Config{})
		tests := []struct {
			name   string
			method InstallMethod
			want   string
		}{
			{
				name:   "Should recommend npm global update for npm installs",
				method: InstallMethodNPM,
				want:   "npm update -g @compozy/agh",
			},
			{
				name:   "Should recommend public go install path for Go installs",
				method: InstallMethodGoInstall,
				want:   "go install github.com/compozy/agh@latest",
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
				if state.Status != StatusDeferred || !state.Managed {
					t.Fatalf("state = %#v, want deferred managed update", state)
				}
				if !strings.Contains(state.Recommendation, tc.want) {
					t.Fatalf("state.Recommendation = %q, want %q", state.Recommendation, tc.want)
				}
			})
		}
	})
}
