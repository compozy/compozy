package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleaseAPIURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("update: create latest release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", m.userAgent())

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("update: request latest release: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("update: latest release request returned %s", resp.Status)
	}

	var payload githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("update: decode latest release response: %w", err)
	}
	if payload.Draft || payload.Prerelease {
		return nil, fmt.Errorf("update: latest release %q is not a stable release", payload.TagName)
	}

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
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("update: close download response %q: %w", url, closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update: download %q returned %s", url, resp.Status)
	}
	if resp.ContentLength > maxBytes {
		return fmt.Errorf(
			"update: download %q size %d exceeds limit %d",
			url,
			resp.ContentLength,
			maxBytes,
		)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("update: create download target %q: %w", path, err)
	}
	removePartial := true
	closed := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil && err == nil {
				err = fmt.Errorf("update: close download target %q: %w", path, closeErr)
			}
		}
		if removePartial {
			if removeErr := os.Remove(path); removeErr != nil &&
				!errors.Is(removeErr, os.ErrNotExist) &&
				err == nil {
				err = fmt.Errorf("update: remove partial download %q: %w", path, removeErr)
			}
		}
	}()

	limited := &io.LimitedReader{R: resp.Body, N: maxBytes + 1}
	written, err := io.Copy(file, limited)
	if err != nil {
		return fmt.Errorf("update: write download %q: %w", path, err)
	}
	if written > maxBytes {
		return fmt.Errorf("update: download %q exceeds limit %d", url, maxBytes)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("update: close download target %q: %w", path, err)
	}
	closed = true
	removePartial = false
	return nil
}

func (m *Manager) userAgent() string {
	version := strings.TrimSpace(m.currentVersion)
	if version == "" {
		version = githubDevKey
	}
	return "agh/" + version
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
		return "agh_" + runtimeOS + "_" + arch + ".tar.gz", nil
	case runtimeOSWindows:
		return "agh_" + runtimeOSWindows + "_" + arch + ".zip", nil
	default:
		return "", fmt.Errorf("update: unsupported operating system %q", runtimeOS)
	}
}
