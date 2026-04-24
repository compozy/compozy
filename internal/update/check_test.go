package update

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	selfupdate "github.com/creativeprojects/go-selfupdate"
)

func TestCheckForUpdateSkipsDevBuild(t *testing.T) {
	restore := stubUpdaterClient(t, fakeUpdaterClient{})
	defer restore()

	path := filepath.Join(t.TempDir(), "state.yml")
	got, err := CheckForUpdate(context.Background(), "dev", path)
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil release for dev build, got %#v", got)
	}
}

func TestCheckForUpdateSkipsWhenNotifierEnvIsSet(t *testing.T) {
	restore := stubUpdaterClient(t, fakeUpdaterClient{})
	defer restore()

	t.Setenv(noUpdateNotifierEnv, "1")

	path := filepath.Join(t.TempDir(), "state.yml")
	got, err := CheckForUpdate(context.Background(), "1.0.0", path)
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil release when notifier is disabled, got %#v", got)
	}
}

func TestCheckForUpdateUsesFreshCacheWithoutNetworkCall(t *testing.T) {
	unsetUpdateNotifierEnv(t)

	fake := fakeUpdaterClient{
		detectLatestFn: func(context.Context) (*ReleaseInfo, error) {
			t.Fatal("DetectLatest should not be called on cache hit")
			return nil, nil
		},
	}
	restoreUpdater := stubUpdaterClient(t, fake)
	defer restoreUpdater()

	restoreNow := stubNowFunc(t, time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC))
	defer restoreNow()

	path := filepath.Join(t.TempDir(), "state.yml")
	if err := WriteState(path, &StateEntry{
		CheckedForUpdateAt: time.Date(2026, time.April, 7, 11, 0, 0, 0, time.UTC),
		LatestRelease: ReleaseInfo{
			Version:     "1.1.0",
			URL:         "https://example.com/v1.1.0",
			PublishedAt: time.Date(2026, time.April, 6, 11, 0, 0, 0, time.UTC),
		},
	}); err != nil {
		t.Fatalf("WriteState returned error: %v", err)
	}

	got, err := CheckForUpdate(context.Background(), "1.0.0", path)
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if got == nil || got.Version != "1.1.0" {
		t.Fatalf("expected cached release info, got %#v", got)
	}
}

func TestCheckForUpdateQueriesWhenCacheIsStale(t *testing.T) {
	unsetUpdateNotifierEnv(t)

	fake := fakeUpdaterClient{
		detectLatestFn: func(context.Context) (*ReleaseInfo, error) {
			return &ReleaseInfo{
				Version:     "1.1.0",
				URL:         "https://example.com/v1.1.0",
				PublishedAt: time.Date(2026, time.April, 7, 9, 0, 0, 0, time.UTC),
			}, nil
		},
	}
	restoreUpdater := stubUpdaterClient(t, fake)
	defer restoreUpdater()

	now := time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC)
	restoreNow := stubNowFunc(t, now)
	defer restoreNow()

	path := filepath.Join(t.TempDir(), "state.yml")
	if err := WriteState(path, &StateEntry{
		CheckedForUpdateAt: now.Add(-25 * time.Hour),
		LatestRelease: ReleaseInfo{
			Version: "1.0.1",
		},
	}); err != nil {
		t.Fatalf("WriteState returned error: %v", err)
	}

	got, err := CheckForUpdate(context.Background(), "1.0.0", path)
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if got == nil || got.Version != "1.1.0" {
		t.Fatalf("expected fresh release info, got %#v", got)
	}

	state, err := ReadState(path)
	if err != nil {
		t.Fatalf("ReadState returned error: %v", err)
	}
	if state == nil {
		t.Fatal("expected persisted state after cache miss")
	}
	if !state.CheckedForUpdateAt.Equal(now) {
		t.Fatalf("unexpected checked time: want %s, got %s", now, state.CheckedForUpdateAt)
	}
	if state.LatestRelease.Version != "1.1.0" {
		t.Fatalf("unexpected persisted latest release: %#v", state.LatestRelease)
	}
}

func TestCheckForUpdateReturnsNilWhenCurrentVersionIsLatest(t *testing.T) {
	unsetUpdateNotifierEnv(t)

	restoreUpdater := stubUpdaterClient(t, fakeUpdaterClient{
		detectLatestFn: func(context.Context) (*ReleaseInfo, error) {
			return &ReleaseInfo{Version: "1.0.0"}, nil
		},
	})
	defer restoreUpdater()

	restoreNow := stubNowFunc(t, time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC))
	defer restoreNow()

	got, err := CheckForUpdate(context.Background(), "1.0.0", filepath.Join(t.TempDir(), "state.yml"))
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil release info, got %#v", got)
	}
}

func TestCheckForUpdateReturnsReleaseWhenLatestIsNewer(t *testing.T) {
	unsetUpdateNotifierEnv(t)

	restoreUpdater := stubUpdaterClient(t, fakeUpdaterClient{
		detectLatestFn: func(context.Context) (*ReleaseInfo, error) {
			return &ReleaseInfo{
				Version:     "1.2.0",
				URL:         "https://example.com/v1.2.0",
				PublishedAt: time.Date(2026, time.April, 7, 9, 0, 0, 0, time.UTC),
			}, nil
		},
	})
	defer restoreUpdater()

	restoreNow := stubNowFunc(t, time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC))
	defer restoreNow()

	got, err := CheckForUpdate(context.Background(), "1.0.0", filepath.Join(t.TempDir(), "state.yml"))
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if got == nil || got.Version != "1.2.0" {
		t.Fatalf("expected newer release info, got %#v", got)
	}
}

func TestShouldCheckForUpdate(t *testing.T) {
	now := time.Date(2026, time.April, 7, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		state *StateEntry
		want  bool
	}{
		{name: "nil state", state: nil, want: true},
		{name: "zero time", state: &StateEntry{}, want: true},
		{name: "fresh state", state: &StateEntry{CheckedForUpdateAt: now.Add(-time.Hour)}, want: false},
		{name: "stale state", state: &StateEntry{CheckedForUpdateAt: now.Add(-25 * time.Hour)}, want: true},
	}

	for _, tt := range tests {
		if got := ShouldCheckForUpdate(tt.state, now); got != tt.want {
			t.Fatalf("%s: want %t, got %t", tt.name, tt.want, got)
		}
	}
}

func TestNewSelfUpdaterClientBuildsDefaultClient(t *testing.T) {
	client, err := newSelfUpdaterClient()
	if err != nil {
		t.Fatalf("newSelfUpdaterClient returned error: %v", err)
	}
	if _, ok := client.(*selfUpdaterClient); !ok {
		t.Fatalf("expected *selfUpdaterClient, got %T", client)
	}
}

func TestSelfUpdaterClientDetectLatestReturnsReleaseInfo(t *testing.T) {
	publishedAt := time.Date(2026, time.April, 7, 9, 0, 0, 0, time.UTC)
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: fakeSelfupdateSource{
			releases: []selfupdate.SourceRelease{
				fakeSelfupdateRelease{
					id:          10,
					tagName:     "v1.2.3",
					name:        "Compozy v1.2.3",
					url:         "https://github.com/compozy/compozy/releases/tag/v1.2.3",
					publishedAt: publishedAt,
					assets: []selfupdate.SourceAsset{
						fakeSelfupdateAsset{
							id:   1,
							name: "compozy_linux_amd64.tar.gz",
							url:  "https://downloads.example.com/compozy_linux_amd64.tar.gz",
						},
						fakeSelfupdateAsset{
							id:   2,
							name: "checksums.txt",
							url:  "https://downloads.example.com/checksums.txt",
						},
					},
				},
			},
		},
		Validator: &selfupdate.ChecksumValidator{UniqueFilename: "checksums.txt"},
		OS:        "linux",
		Arch:      "amd64",
	})
	if err != nil {
		t.Fatalf("NewUpdater returned error: %v", err)
	}

	client := &selfUpdaterClient{updater: updater}
	got, err := client.DetectLatest(context.Background())
	if err != nil {
		t.Fatalf("DetectLatest returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected release info, got nil")
	}
	if got.Version != "1.2.3" {
		t.Fatalf("unexpected version: %q", got.Version)
	}
	if got.URL != "https://github.com/compozy/compozy/releases/tag/v1.2.3" {
		t.Fatalf("unexpected url: %q", got.URL)
	}
	if !got.PublishedAt.Equal(publishedAt) {
		t.Fatalf("unexpected published time: %s", got.PublishedAt)
	}
}

func TestNewerReleaseRejectsInvalidVersions(t *testing.T) {
	if _, err := newerRelease("broken", &ReleaseInfo{Version: "1.0.0"}); err == nil {
		t.Fatal("expected current-version parse error")
	}
	if _, err := newerRelease("1.0.0", &ReleaseInfo{Version: "broken"}); err == nil {
		t.Fatal("expected latest-version parse error")
	}
}

func TestNewerReleaseComparesGitDescribeVersionByBaseRelease(t *testing.T) {
	t.Parallel()

	t.Run("Should not report update when current build is ahead of latest release tag", func(t *testing.T) {
		t.Parallel()

		got, err := newerRelease("v0.1.12-15-g834fec6", &ReleaseInfo{Version: "0.1.12"})
		if err != nil {
			t.Fatalf("newerRelease returned error: %v", err)
		}
		if got != nil {
			t.Fatalf("expected no update for matching base release, got %#v", got)
		}
	})

	t.Run("Should report update when latest release is newer than current base tag", func(t *testing.T) {
		t.Parallel()

		got, err := newerRelease("v0.1.12-15-g834fec6", &ReleaseInfo{Version: "0.1.13"})
		if err != nil {
			t.Fatalf("newerRelease returned error: %v", err)
		}
		if got == nil || got.Version != "0.1.13" {
			t.Fatalf("expected newer release info, got %#v", got)
		}
	})
}

func TestReleaseInfoPtrReturnsNilForEmptyInfo(t *testing.T) {
	if got := releaseInfoPtr(ReleaseInfo{}); got != nil {
		t.Fatalf("expected nil pointer, got %#v", got)
	}
}

type fakeUpdaterClient struct {
	detectLatestFn func(context.Context) (*ReleaseInfo, error)
	updateSelfFn   func(context.Context, string) (*ReleaseInfo, error)
}

func (f fakeUpdaterClient) DetectLatest(ctx context.Context) (*ReleaseInfo, error) {
	if f.detectLatestFn == nil {
		return nil, nil
	}
	return f.detectLatestFn(ctx)
}

func (f fakeUpdaterClient) UpdateSelf(ctx context.Context, currentVersion string) (*ReleaseInfo, error) {
	if f.updateSelfFn == nil {
		return nil, nil
	}
	return f.updateSelfFn(ctx, currentVersion)
}

func stubUpdaterClient(t *testing.T, client updaterClient) func() {
	t.Helper()

	previous := newUpdaterClient
	newUpdaterClient = func() (updaterClient, error) {
		return client, nil
	}
	return func() {
		newUpdaterClient = previous
	}
}

func stubNowFunc(t *testing.T, now time.Time) func() {
	t.Helper()

	previous := nowFunc
	nowFunc = func() time.Time { return now }
	return func() {
		nowFunc = previous
	}
}

func unsetUpdateNotifierEnv(t *testing.T) {
	t.Helper()

	previous, hadValue := os.LookupEnv(noUpdateNotifierEnv)
	if err := os.Unsetenv(noUpdateNotifierEnv); err != nil {
		t.Fatalf("unset %s: %v", noUpdateNotifierEnv, err)
	}
	t.Cleanup(func() {
		if hadValue {
			_ = os.Setenv(noUpdateNotifierEnv, previous)
			return
		}
		_ = os.Unsetenv(noUpdateNotifierEnv)
	})
}

type fakeSelfupdateSource struct {
	releases []selfupdate.SourceRelease
	assets   map[int64][]byte
}

func (f fakeSelfupdateSource) ListReleases(context.Context, selfupdate.Repository) ([]selfupdate.SourceRelease, error) {
	return f.releases, nil
}

func (f fakeSelfupdateSource) DownloadReleaseAsset(
	_ context.Context,
	_ *selfupdate.Release,
	assetID int64,
) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.assets[assetID])), nil
}

type fakeSelfupdateRelease struct {
	id           int64
	tagName      string
	name         string
	url          string
	releaseNotes string
	publishedAt  time.Time
	draft        bool
	prerelease   bool
	assets       []selfupdate.SourceAsset
}

func (f fakeSelfupdateRelease) GetID() int64                        { return f.id }
func (f fakeSelfupdateRelease) GetTagName() string                  { return f.tagName }
func (f fakeSelfupdateRelease) GetDraft() bool                      { return f.draft }
func (f fakeSelfupdateRelease) GetPrerelease() bool                 { return f.prerelease }
func (f fakeSelfupdateRelease) GetPublishedAt() time.Time           { return f.publishedAt }
func (f fakeSelfupdateRelease) GetReleaseNotes() string             { return f.releaseNotes }
func (f fakeSelfupdateRelease) GetName() string                     { return f.name }
func (f fakeSelfupdateRelease) GetURL() string                      { return f.url }
func (f fakeSelfupdateRelease) GetAssets() []selfupdate.SourceAsset { return f.assets }

type fakeSelfupdateAsset struct {
	id   int64
	name string
	size int
	url  string
}

func (f fakeSelfupdateAsset) GetID() int64                  { return f.id }
func (f fakeSelfupdateAsset) GetName() string               { return f.name }
func (f fakeSelfupdateAsset) GetSize() int                  { return f.size }
func (f fakeSelfupdateAsset) GetBrowserDownloadURL() string { return f.url }
