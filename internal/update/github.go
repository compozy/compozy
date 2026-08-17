package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const (
	githubDevKey = "dev"
)

type githubReleaseResponse struct {
	TagName     string                `json:"tag_name"`
	HTMLURL     string                `json:"html_url"`
	Draft       bool                  `json:"draft"`
	Prerelease  bool                  `json:"prerelease"`
	PublishedAt time.Time             `json:"published_at"`
	Assets      []githubAssetResponse `json:"assets"`
}

type githubAssetResponse struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (m *Manager) fetchLatestRelease(ctx context.Context) (*Release, error) {
	if m.releaseTrack == releaseTrackBeta {
		return m.fetchLatestBetaRelease(ctx)
	}
	return m.fetchLatestStableRelease(ctx)
}

// ResolveReleaseByTag fetches the immutable release identity recorded by an operation.
func (m *Manager) ResolveReleaseByTag(ctx context.Context, tag string) (*Release, error) {
	cleanTag := strings.TrimSpace(tag)
	if cleanTag == "" {
		return nil, errors.New("update: release tag is required")
	}
	endpoint := "https://api.github.com/repos/" + githubRepositorySlug + "/releases/tags/" + url.PathEscape(cleanTag)
	var payload githubReleaseResponse
	if err := m.fetchGitHubJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	if payload.Draft {
		return nil, fmt.Errorf("update: release %q is a draft", cleanTag)
	}
	release, err := releaseFromGitHubResponse(payload)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(release.Version) != cleanTag {
		return nil, fmt.Errorf("update: release tag mismatch: got %q, want %q", release.Version, cleanTag)
	}
	return release, nil
}

func (m *Manager) fetchLatestStableRelease(ctx context.Context) (*Release, error) {
	var payload githubReleaseResponse
	if err := m.fetchGitHubJSON(ctx, githubLatestReleaseAPIURL, &payload); err != nil {
		return nil, err
	}
	if payload.Draft || payload.Prerelease {
		return nil, fmt.Errorf("update: latest release %q is not a stable release", payload.TagName)
	}
	return releaseFromGitHubResponse(payload)
}

func (m *Manager) fetchLatestBetaRelease(ctx context.Context) (*Release, error) {
	_, current, err := releaseTrackForVersion(m.currentVersion)
	if err != nil {
		return nil, fmt.Errorf("update: parse current beta version: %w", err)
	}

	var payloads []githubReleaseResponse
	if err := m.fetchGitHubJSON(ctx, githubReleasesAPIURL, &payloads); err != nil {
		return nil, err
	}

	var (
		selected        *githubReleaseResponse
		selectedVersion *semver.Version
	)
	for index := range payloads {
		payload := &payloads[index]
		if payload.Draft || !payload.Prerelease {
			continue
		}
		candidate, parseErr := parseVersion(payload.TagName)
		if parseErr != nil || !isBetaVersionForLine(candidate, current) {
			continue
		}
		if selectedVersion == nil || candidate.GreaterThan(selectedVersion) {
			selected = payload
			selectedVersion = candidate
		}
	}
	if selected == nil {
		return nil, fmt.Errorf(
			"update: no beta release found for v%d.%d",
			current.Major(),
			current.Minor(),
		)
	}
	return releaseFromGitHubResponse(*selected)
}

func (m *Manager) fetchGitHubJSON(ctx context.Context, url string, target any) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("update: create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", m.userAgent())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: request release metadata: %w", err)
	}
	defer func() {
		err = errors.Join(err, drainAndCloseUpdateResponseBody(resp.Body))
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: release request returned %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("update: decode release response: %w", err)
	}
	return nil
}

func releaseFromGitHubResponse(payload githubReleaseResponse) (*Release, error) {
	release := &Release{
		Version:     strings.TrimSpace(payload.TagName),
		ReleaseURL:  strings.TrimSpace(payload.HTMLURL),
		PublishedAt: payload.PublishedAt.UTC(),
		Assets:      make([]ReleaseAsset, 0, len(payload.Assets)),
	}
	if release.Version == "" {
		return nil, fmt.Errorf("update: latest release is missing a tag name")
	}
	for _, asset := range payload.Assets {
		release.Assets = append(release.Assets, ReleaseAsset{
			Name:        strings.TrimSpace(asset.Name),
			DownloadURL: strings.TrimSpace(asset.BrowserDownloadURL),
		})
	}
	return release, nil
}

func (m *Manager) downloadFile(ctx context.Context, url string, path string, maxBytes int64) (err error) {
	if maxBytes <= 0 {
		return fmt.Errorf("update: invalid download limit %d for %q", maxBytes, url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("update: create download request for %q: %w", url, err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", m.userAgent())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("update: download %q: %w", url, err)
	}
	defer func() {
		err = errors.Join(err, drainAndCloseUpdateResponseBody(resp.Body))
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download %q returned %s", url, resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return &downloadSizeError{Path: url, Size: resp.ContentLength, Limit: maxBytes}
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("update: create download target %q: %w", path, err)
	}
	removePartial := true
	defer func() {
		if removePartial {
			if removeErr := os.Remove(path); removeErr != nil &&
				!errors.Is(removeErr, os.ErrNotExist) {
				err = errors.Join(err, fmt.Errorf("update: remove partial download %q: %w", path, removeErr))
			}
		}
	}()

	if err := writeDownloadTarget(path, file, resp.Body, maxBytes); err != nil {
		return err
	}
	removePartial = false
	return nil
}

func (m *Manager) userAgent() string {
	version := strings.TrimSpace(m.currentVersion)
	if version == "" {
		version = githubDevKey
	}
	return "compozy/" + version
}

func (r *Release) findAsset(name string) (ReleaseAsset, bool) {
	for _, asset := range r.Assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), strings.TrimSpace(name)) {
			return asset, true
		}
	}
	return ReleaseAsset{}, false
}

func archiveAssetName(runtimeOS string, runtimeArch string) (string, error) {
	var arch string
	switch runtimeArch {
	case runtimeArchAMD64:
		arch = "x86_64"
	case runtimeArchARM64:
		arch = "arm64"
	default:
		return "", fmt.Errorf("update: unsupported architecture %q", runtimeArch)
	}

	switch runtimeOS {
	case runtimeOSDarwin, runtimeOSLinux:
		return "compozy_" + runtimeOS + "_" + arch + ".tar.gz", nil
	case runtimeOSWindows:
		return "compozy_" + runtimeOSWindows + "_" + arch + ".zip", nil
	default:
		return "", fmt.Errorf("update: unsupported operating system %q", runtimeOS)
	}
}
