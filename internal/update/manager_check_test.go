package update

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/iotest"
	"testing/synctest"
	"time"

	"github.com/compozy/compozy/internal/appstatecontract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestManagerCheck(t *testing.T) {
	t.Run("Should keep deadline failures visible and throttle subsequent manual checks", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			var requests atomic.Int32
			manager, _ := newManagerWithExecutable(t, &Config{
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if requests.Add(1) > 1 {
						return nil, errors.New("unexpected duplicate check")
					}
					<-request.Context().Done()
					return nil, request.Context().Err()
				})},
			})
			checkedAt := time.Now().UTC().Add(-time.Hour)
			if err := writeCache(manager.cachePath(), testCacheEntry(t, manager, "v1.0.0",
				"https://github.com/compozy/compozy/releases/tag/v1.0.0", checkedAt)); err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Refresh(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("timed-out Refresh error = %v", err)
			}
			for range 2 {
				snapshot, err := manager.Refresh(t.Context())
				if err != nil || !strings.Contains(snapshot.Runtime.LastError, context.DeadlineExceeded.Error()) ||
					snapshot.Runtime.CheckedAt == nil || !snapshot.Runtime.CheckedAt.Equal(checkedAt) {
					t.Fatalf("Refresh after timeout = %#v, %v", snapshot, err)
				}
			}
			if requests.Load() != 1 {
				t.Fatalf("requests after timeout = %d, want one throttled attempt", requests.Load())
			}
		})
	})
	t.Run("Should complete a bounded manual refresh after the client disconnects", func(t *testing.T) {
		t.Parallel()
		manager, _ := newManagerWithExecutable(t, &Config{
			HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if err := request.Context().Err(); err != nil {
					return nil, err
				}
				deadline, ok := request.Context().Deadline()
				if !ok || time.Until(deadline) > defaultHTTPTimeout {
					t.Errorf("manual refresh deadline = %v, present %t", deadline, ok)
				}
				return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
					TagName: "v1.2.0", HTMLURL: "https://github.com/compozy/compozy/releases/tag/v1.2.0",
					Assets: testGitHubAssets(t, nil),
				}), nil
			})},
		})
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		state, err := manager.Refresh(ctx)
		if err != nil || state.Runtime.LatestVersion != "v1.2.0" || state.Runtime.CheckedAt == nil {
			t.Fatalf("Refresh after disconnect = %#v, %v", state, err)
		}
	})
	t.Run("Should retry immediately after cancellation without hiding upstream failures", func(t *testing.T) {
		t.Parallel()
		var requests atomic.Int32
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		manager, _ := newManagerWithExecutable(t, &Config{
			HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if requests.Add(1) == 1 {
					cancel()
					return nil, request.Context().Err()
				}
				return nil, errors.New("upstream unavailable")
			})},
		})
		if _, _, err := manager.Check(ctx, CheckOptions{ForceRefresh: true}); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled Check error = %v", err)
		}
		state, _, err := manager.Check(
			t.Context(),
			CheckOptions{ForceRefresh: true, MinimumRefreshInterval: time.Minute},
		)
		if requests.Load() != 2 || err == nil || !strings.Contains(state.LastError, "upstream unavailable") {
			t.Fatalf("retry = %#v, %v, requests %d", state, err, requests.Load())
		}
	})
	t.Run("Should honor GitHub reset deadlines across manual and background checks", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 18, 14, 18, 0, 0, time.UTC)
		reset := now.Add(time.Hour)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			Now: func() time.Time { return now },
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return &http.Response{
					StatusCode: http.StatusForbidden,
					Status:     "403 Forbidden",
					Header: http.Header{
						"X-Ratelimit-Remaining": {"0"},
						"X-Ratelimit-Reset":     {strconv.FormatInt(reset.Unix(), 10)},
						"Retry-After":           {"120"},
					},
					Body: io.NopCloser(strings.NewReader("{}")),
				}, nil
			})},
		})
		if err := writeCache(manager.cachePath(), testCacheEntry(t, manager, "v1.0.0",
			"https://github.com/compozy/compozy/releases/tag/v1.0.0", now.Add(-time.Hour))); err != nil {
			t.Fatal(err)
		}
		if _, err := manager.Refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		now = now.Add(15 * time.Minute)
		state, _, err := manager.CheckAll(t.Context(), CheckOptions{ForceRefresh: true, AllowCachedOnFailure: true})
		if err != nil || state.Runtime.LastError == "" || requests.Load() != 1 {
			t.Fatalf("rate limited snapshot = %#v, %v, requests %d", state, err, requests.Load())
		}
		now = reset
		if _, err := manager.Refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if requests.Load() != 2 {
			t.Fatalf("requests after reset = %d, want 2", requests.Load())
		}
	})
	t.Run("Should discover a published release on manual refresh and coalesce repeated clients", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 18, 14, 18, 0, 0, time.UTC)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			Now: func() time.Time { return now },
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return jsonHTTPResponse(t, http.StatusOK, githubReleaseResponse{
					TagName: "v1.2.0", HTMLURL: "https://github.com/compozy/compozy/releases/tag/v1.2.0",
					PublishedAt: now.Add(-18 * time.Minute), Assets: testGitHubAssets(t, nil),
				}), nil
			})},
		})
		if err := writeCache(manager.cachePath(), testCacheEntry(t, manager, "v1.0.0",
			"https://github.com/compozy/compozy/releases/tag/v1.0.0", now.Add(-time.Hour))); err != nil {
			t.Fatal(err)
		}
		cached, err := manager.Snapshot(t.Context())
		if err != nil || cached.Runtime.LatestVersion != "v1.0.0" || requests.Load() != 0 {
			t.Fatalf("Snapshot = %#v, %v, requests %d", cached, err, requests.Load())
		}
		var clients sync.WaitGroup
		for range 3 {
			clients.Go(func() {
				fresh, err := manager.Refresh(t.Context())
				if err != nil || fresh.Runtime.LatestVersion != "v1.2.0" || fresh.Runtime.CheckedAt == nil ||
					!fresh.Runtime.CheckedAt.Equal(now) {
					t.Errorf("Refresh = %#v, %v", fresh, err)
				}
			})
		}
		clients.Wait()
		if requests.Load() != 1 {
			t.Fatalf("requests = %d, want one shared refresh", requests.Load())
		}
		now = now.Add(time.Minute)
		if _, err := manager.Refresh(t.Context()); err != nil {
			t.Fatal(err)
		}
		if requests.Load() != 2 {
			t.Fatalf("requests after cooldown = %d, want 2", requests.Load())
		}
	})

	t.Run("Should preserve the successful check time and failure during manual refresh cooldown", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 10, 18, 14, 18, 0, 0, time.UTC)
		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			Now: func() time.Time { return now },
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return nil, errors.New("upstream unavailable")
			})},
		})
		checkedAt := now.Add(-time.Hour)
		if err := writeCache(manager.cachePath(), testCacheEntry(t, manager, "v1.0.0",
			"https://github.com/compozy/compozy/releases/tag/v1.0.0", checkedAt)); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			state, err := manager.Refresh(t.Context())
			if err != nil || state.Runtime.LastError == "" || state.Runtime.CheckedAt == nil ||
				!state.Runtime.CheckedAt.Equal(checkedAt) {
				t.Fatalf("Refresh = %#v, %v", state, err)
			}
		}
		if requests.Load() != 1 {
			t.Fatalf("failed requests = %d, want 1", requests.Load())
		}
	})

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

		state, release, err := manager.Check(t.Context(), CheckOptions{})
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

		_, freshRelease, err := manager.Check(t.Context(), CheckOptions{})
		if err != nil {
			t.Fatalf("Check(fresh) error = %v", err)
		}
		if _, err := manager.resolveReleaseAssets(freshRelease); err != nil {
			t.Fatalf("resolveReleaseAssets(fresh) error = %v", err)
		}
		_, cachedRelease, err := manager.Check(t.Context(), CheckOptions{})
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

		state, release, err := manager.Check(t.Context(), CheckOptions{})
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

			state, release, err := manager.Check(t.Context(), CheckOptions{
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

		manager := &Manager{currentVersion: "dev"}

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

		state, release, err := manager.Check(t.Context(), CheckOptions{})
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

		_, err := manager.fetchLatestRelease(t.Context())
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

		state, release, err := manager.Check(t.Context(), CheckOptions{ForceRefresh: true})
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

	t.Run("Should move a beta install onto a newer stable release", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)
		manager, _ := newManagerWithExecutable(t, &Config{
			CurrentVersion: "v0.3.0-beta.38",
			Now:            func() time.Time { return now },
			HTTPClient: &http.Client{
				Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return jsonHTTPResponse(t, http.StatusOK, []githubReleaseResponse{
						{
							TagName:     "v0.4.0",
							HTMLURL:     "https://github.com/compozy/compozy/releases/tag/v0.4.0",
							PublishedAt: now.Add(-time.Hour),
							Assets:      testGitHubAssets(t, nil),
						},
						{
							TagName:     "v0.3.0-beta.38",
							Prerelease:  true,
							PublishedAt: now.Add(-24 * time.Hour),
						},
						{
							TagName:     "v0.2.15",
							PublishedAt: now.Add(-48 * time.Hour),
						},
					}), nil
				}),
			},
		})

		state, release, err := manager.Check(t.Context(), CheckOptions{ForceRefresh: true})
		if err != nil {
			t.Fatalf("Check() error = %v", err)
		}
		if release == nil || release.Version != "v0.4.0" {
			t.Fatalf("release = %#v, want stable v0.4.0", release)
		}
		if state.Status != StatusAvailable || !state.Available {
			t.Fatalf("state = %#v, want available stable update", state)
		}
	})

	t.Run("Should mark Windows direct-binary installs as manual-only", func(t *testing.T) {
		t.Parallel()

		manager := &Manager{currentVersion: "v1.0.0", runtimeOS: runtimeOSWindows, runtimeArch: runtimeArchAMD64}

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

		manager := &Manager{currentVersion: "v1.0.0", runtimeOS: runtimeOSLinux, runtimeArch: runtimeArchAMD64}
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

		manager := &Manager{currentVersion: "v1.0.0", runtimeOS: runtimeOSLinux, runtimeArch: runtimeArchAMD64}
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

		manager := &Manager{
			currentVersion: "v0.3.0-beta.1", releaseTrack: releaseTrackBeta,
			runtimeOS: runtimeOSLinux, runtimeArch: runtimeArchAMD64,
		}
		tests := []struct {
			name      string
			method    InstallMethod
			want      string
			forbidden string
		}{
			{
				name:   "Should recommend the plain npm package that carries the stable release",
				method: InstallMethodNPM,
				want:   "npm install -g @compozy/cli`",
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
				ToVersion:           "v1.2.0",
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
				{Name: "CompozyOS-1.2.0-mac-arm64-debug.zip"},
				{Name: "CompozyOS-1.2.0-mac-arm64.dmg"},
				{Name: "CompozyOS-1.2.0-mac-arm64.zip"},
				{Name: "CompozyOS-1.2.0-mac-x64.zip"},
				{Name: "CompozyOS-1.2.0-linux-x64.AppImage"},
				{Name: "compozy_darwin_x86_64.tar.gz"},
				{Name: checksumsAssetName},
				{Name: checksumsBundleAssetName},
				{Name: compatibilityAssetName},
			},
		}
		mac := &Manager{runtimeOS: runtimeOSDarwin, runtimeArch: runtimeArchARM64}
		macAsset, err := mac.resolveAppReleaseAsset(t.Context(), release)
		if err != nil {
			t.Fatalf("resolveAppReleaseAsset(macOS) error = %v", err)
		}
		if macAsset.Name != "CompozyOS-1.2.0-mac-arm64.zip" {
			t.Fatalf("resolveAppReleaseAsset(macOS) = %q, want zip", macAsset.Name)
		}
		for _, tc := range []struct {
			name   string
			output string
			err    error
			asset  string
		}{
			{name: "Should match Apple Silicon hardware under Rosetta", output: "1\n", asset: "arm64"},
			{name: "Should retain Intel app on Intel hardware", output: "0\n", asset: "x64"},
			{name: "Should retain Intel app when hardware cannot be probed", err: errors.New("sysctl unavailable"), asset: "x64"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				manager := &Manager{
					runtimeOS: runtimeOSDarwin, runtimeArch: runtimeArchAMD64,
					runCommand: func(ctx context.Context, name string, args ...string) (string, error) {
						if name != "/usr/sbin/sysctl" || !slices.Equal(args, []string{"-n", "hw.optional.arm64"}) {
							t.Fatalf("hardware probe = %s %v, want Apple Silicon capability", name, args)
						}
						if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > time.Second {
							t.Fatal("hardware probe must have a bounded lifetime")
						}
						return tc.output, tc.err
					},
				}
				asset, err := manager.resolveAppReleaseAsset(t.Context(), release)
				if err != nil || asset.Name != "CompozyOS-1.2.0-mac-"+tc.asset+".zip" {
					t.Fatalf("resolveAppReleaseAsset() = %q, %v, want %s app", asset.Name, err, tc.asset)
				}
				assets, err := manager.resolvePlanReleaseAssets(release, true)
				if err != nil || assets.archive.Name != "compozy_darwin_x86_64.tar.gz" {
					t.Fatalf("runtime archive = %q, %v, want unchanged amd64", assets.archive.Name, err)
				}
			})
		}

		linux := &Manager{runtimeOS: runtimeOSLinux, runtimeArch: runtimeArchAMD64}
		linuxAsset, err := linux.resolveAppReleaseAsset(t.Context(), release)
		if err != nil {
			t.Fatalf("resolveAppReleaseAsset(Linux) error = %v", err)
		}
		if linuxAsset.Name != "CompozyOS-1.2.0-linux-x64.AppImage" {
			t.Fatalf("resolveAppReleaseAsset(Linux) = %q, want AppImage", linuxAsset.Name)
		}

		linuxARM := &Manager{runtimeOS: runtimeOSLinux, runtimeArch: runtimeArchARM64}
		if _, err := linuxARM.resolveAppReleaseAsset(t.Context(), release); err == nil ||
			!strings.Contains(err.Error(), "unsupported on linux/arm64") {
			t.Fatalf("resolveAppReleaseAsset(Linux ARM) error = %v, want unsupported", err)
		}

		windows := &Manager{runtimeOS: runtimeOSWindows, runtimeArch: runtimeArchAMD64}
		if _, err := windows.resolveAppReleaseAsset(t.Context(), release); err == nil {
			t.Fatal("resolveAppReleaseAsset(Windows) error = nil, want unsupported")
		}
	})

	t.Run("Should omit the runtime payload from app-only planning downloads", func(t *testing.T) {
		t.Parallel()

		var requested []string
		manager, _ := newManagerWithExecutable(t, &Config{
			RuntimeOS: runtimeOSLinux, RuntimeArch: runtimeArchAMD64,
			HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				requested = append(requested, request.URL.String())
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("verified support artifact")),
				}, nil
			})},
		})
		release := &Release{Version: "v1.2.0", Assets: []ReleaseAsset{
			{Name: checksumsAssetName, DownloadURL: "https://downloads.example/checksums"},
			{Name: checksumsBundleAssetName, DownloadURL: "https://downloads.example/bundle"},
			{Name: compatibilityAssetName, DownloadURL: "https://downloads.example/compatibility"},
			{Name: "CompozyOS-1.2.0-linux-x64.AppImage", DownloadURL: "https://downloads.example/app"},
		}}
		assets, err := manager.resolvePlanReleaseAssets(release, false)
		if err != nil {
			t.Fatalf("resolvePlanReleaseAssets(app-only) error = %v", err)
		}
		if assets.archive.Name != "" {
			t.Fatalf("app-only archive = %#v, want omitted", assets.archive)
		}
		if _, err := manager.downloadReleaseArtifacts(t.Context(), t.TempDir(), assets); err != nil {
			t.Fatalf("downloadReleaseArtifacts(app-only) error = %v", err)
		}
		if len(requested) != 3 {
			t.Fatalf("app-only download requests = %v, want three support artifacts", requested)
		}
		for _, requestURL := range requested {
			if requestURL == "https://downloads.example/app" {
				t.Fatalf("app-only planning downloaded install payload: %s", requestURL)
			}
		}
	})

	t.Run("Should plan desktop-owned joint updates through the app bundle", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name           string
			desktopOwned   bool
			managed        string
			currentVersion string
			appVersion     string
			targets        []Target
			wantTargets    []Target
			wantError      string
		}{
			{
				name: "Should plan only the app when its bundle owns both requested updates", desktopOwned: true,
				targets: []Target{TargetRuntime, TargetApp}, wantTargets: []Target{TargetApp},
			},
			{
				name: "Should preserve an explicit desktop runtime-only request", desktopOwned: true,
				targets: []Target{TargetRuntime}, wantTargets: []Target{TargetRuntime},
			},
			{
				name: "Should preserve a joint update for an independently installed runtime", appVersion: "v1.0.0",
				targets: []Target{TargetRuntime, TargetApp}, wantTargets: []Target{TargetRuntime, TargetApp},
			},
			{
				name:    "Should preserve a standalone runtime update",
				targets: []Target{TargetRuntime}, wantTargets: []Target{TargetRuntime},
			},
			{
				name: "Should preserve managed runtime refusal before collapsing targets", desktopOwned: true,
				managed: string(InstallMethodDesktopApp), targets: []Target{TargetRuntime, TargetApp},
				wantError: "runtime is not eligible",
			},
			{
				name: "Should preserve app eligibility checks before collapsing targets", desktopOwned: true,
				appVersion: "v1.1.0", targets: []Target{TargetRuntime, TargetApp},
				wantError: "desktop app update is not available",
			},
			{
				name: "Should preserve runtime eligibility checks before collapsing targets", desktopOwned: true,
				currentVersion: "v1.1.0", appVersion: "v1.0.0", targets: []Target{TargetRuntime, TargetApp},
				wantError: "runtime is not eligible",
			},
			{
				name: "Should preserve target order validation before collapsing targets", desktopOwned: true,
				targets: []Target{TargetApp, TargetRuntime}, wantError: "runtime must be the first target",
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				paths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
				if err != nil {
					t.Fatal(err)
				}
				binaryPath := filepath.Join(paths.BinDir, compozyBinaryName)
				if err := os.MkdirAll(paths.BinDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(binaryPath, []byte("current-runtime"), 0o700); err != nil {
					t.Fatal(err)
				}
				if tc.desktopOwned {
					if err := WriteDesktopProvenance(paths, binaryPath, DesktopProvenanceMetadata{
						AppVersion: "v1.0.0", Channel: "beta", RuntimeVersion: "v1.0.0",
					}); err != nil {
						t.Fatal(err)
					}
				}
				if tc.appVersion != "" {
					appState, err := json.Marshal(appStateSnapshot{
						SchemaVersion: appstatecontract.SchemaVersion, AppVersion: tc.appVersion,
					})
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(paths.AppStateFile, appState, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				verifier := &stubBundleVerifier{}
				manager, _ := newManagerWithExecutable(t, &Config{
					HomePaths: paths, CurrentVersion: tc.currentVersion,
					RuntimeOS: runtimeOSDarwin, RuntimeArch: runtimeArchARM64,
					ExecutablePath: func() (string, error) { return binaryPath, nil },
					Getenv: func(key string) string {
						if key == ManagedEnvName {
							return tc.managed
						}
						return ""
					},
					Now: func() time.Time { return testOperationNow }, BundleVerifier: verifier,
				})
				manager.operationStore.now = manager.now
				archiveBody := createTarGzBinary(t, compozyBinaryName, []byte("new-runtime"), 0o755)
				archiveName, err := archiveAssetName(manager.runtimeOS, manager.runtimeArch)
				if err != nil {
					t.Fatal(err)
				}
				appAsset := "CompozyOS-1.1.0-mac-arm64.zip"
				appDigest := sha256Hex([]byte("new-app"))
				release, _, server := newReleaseFixtureServer(t, manager, assetFixture{
					archiveBody: archiveBody, bundleBody: []byte(`{}`),
					checksumsBody: fmt.Appendf(nil, "%s  %s\n%s  %s\n",
						sha256Hex(archiveBody), archiveName, appDigest, appAsset),
				})
				t.Cleanup(server.Close)
				release.Assets = append(release.Assets, ReleaseAsset{Name: appAsset, DownloadURL: server.URL + "/app"})
				payload := githubReleaseResponse{TagName: release.Version, HTMLURL: release.ReleaseURL}
				for _, asset := range release.Assets {
					payload.Assets = append(payload.Assets, githubAssetResponse{
						Name: asset.Name, BrowserDownloadURL: asset.DownloadURL,
					})
				}
				var archiveRequests, supportRequests atomic.Int32
				manager.httpClient = &http.Client{
					Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
						if request.URL.Host == "api.github.com" {
							return jsonHTTPResponse(t, http.StatusOK, payload), nil
						}
						if request.URL.Path == "/archive" {
							archiveRequests.Add(1)
						} else {
							supportRequests.Add(1)
						}
						return server.Client().Transport.RoundTrip(request)
					}),
				}
				targets := slices.Clone(tc.targets)
				request, err := manager.PlanOperation(
					t.Context(),
					ActorWeb,
					targets,
					operationTestRequest(testOperationNow).Holder,
				)
				if !slices.Equal(targets, tc.targets) {
					t.Fatalf("caller targets = %v, want unchanged %v", targets, tc.targets)
				}
				if tc.wantError != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantError) {
						t.Fatalf("PlanOperation() error = %v, want %q", err, tc.wantError)
					}
					if verifier.calls != 0 || archiveRequests.Load() != 0 || supportRequests.Load() != 0 {
						t.Fatal("invalid plan acquired release artifacts")
					}
					return
				}
				if err != nil {
					t.Fatalf("PlanOperation() error = %v", err)
				}
				if !slices.Equal(request.Targets, tc.wantTargets) {
					t.Errorf("planned targets = %v, want %v", request.Targets, tc.wantTargets)
				}
				wantRuntime := slices.Contains(tc.wantTargets, TargetRuntime)
				var wantArchiveRequests int32
				if wantRuntime {
					wantArchiveRequests = 1
				}
				if (request.Runtime != nil) != wantRuntime || archiveRequests.Load() != wantArchiveRequests {
					t.Errorf("runtime plan = %#v, archive requests = %d, want runtime=%t",
						request.Runtime, archiveRequests.Load(), wantRuntime)
				}
				if verifier.calls != 1 || supportRequests.Load() != 3 {
					t.Errorf(
						"verification calls = %d, support requests = %d, want 1 and 3",
						verifier.calls,
						supportRequests.Load(),
					)
				}
				if slices.Contains(tc.wantTargets, TargetApp) &&
					(request.App == nil || request.App.Digest != "sha256:"+appDigest) {
					t.Fatalf("app plan = %#v, want verified release digest", request.App)
				}
				operation, err := manager.AcquireOperation(t.Context(), request)
				if err != nil {
					t.Fatalf("AcquireOperation() error = %v", err)
				}
				if !slices.Equal(operation.Targets, tc.wantTargets) || operation.ActiveTarget != tc.wantTargets[0] {
					t.Errorf(
						"acquired targets = %v, active = %q, want %v",
						operation.Targets,
						operation.ActiveTarget,
						tc.wantTargets,
					)
				}
			})
		}
	})

	t.Run("Should reject invalid planner identity before network work", func(t *testing.T) {
		t.Parallel()

		var requests atomic.Int32
		manager, _ := newManagerWithExecutable(t, &Config{
			HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				requests.Add(1)
				return nil, errors.New("unexpected network request")
			})},
		})
		_, err := manager.PlanOperation(t.Context(), Actor("invalid"), []Target{TargetRuntime}, Holder{})
		if err == nil || !strings.Contains(err.Error(), "invalid requester") {
			t.Fatalf("PlanOperation(invalid identity) error = %v, want requester refusal", err)
		}
		if got := requests.Load(); got != 0 {
			t.Fatalf("planner network requests = %d, want zero", got)
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
		err := manager.fetchGitHubJSON(t.Context(), "https://example.invalid/releases", &release)
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
		err := manager.fetchGitHubJSON(t.Context(), "https://example.invalid/releases", &release)
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
