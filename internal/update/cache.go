package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/agh/internal/fileutil"
)

func readCache(path string) (*cacheEntry, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return nil, errors.New("update: cache path is required")
	}

	data, err := os.ReadFile(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNoCachedRelease
		}
		return nil, fmt.Errorf("update: read cache %q: %w", trimmed, err)
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, fmt.Errorf("update: decode cache %q: %w", trimmed, err)
	}
	if strings.TrimSpace(entry.LatestVersion) == "" || entry.CheckedAt.IsZero() {
		return nil, ErrNoCachedRelease
	}
	if len(entry.Assets) == 0 {
		return nil, ErrNoCachedRelease
	}
	return &entry, nil
}

func writeCache(path string, entry cacheEntry) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return errors.New("update: cache path is required")
	}
	if strings.TrimSpace(entry.LatestVersion) == "" {
		return errors.New("update: cached latest version is required")
	}
	if entry.CheckedAt.IsZero() {
		return errors.New("update: cached checked_at is required")
	}
	if len(entry.Assets) == 0 {
		return errors.New("update: cached release assets are required")
	}

	if err := os.MkdirAll(filepath.Dir(trimmed), 0o755); err != nil {
		return fmt.Errorf("update: create cache directory %q: %w", filepath.Dir(trimmed), err)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("update: encode cache entry: %w", err)
	}
	data = append(data, '\n')
	if err := fileutil.AtomicWriteFile(trimmed, data, 0o600); err != nil {
		return fmt.Errorf("update: write cache %q: %w", trimmed, err)
	}
	return nil
}

func cacheEntryFromRelease(release *Release, checkedAt time.Time) (cacheEntry, error) {
	if release == nil {
		return cacheEntry{}, errors.New("update: release metadata is required")
	}
	entry := cacheEntry{
		LatestVersion: strings.TrimSpace(release.Version),
		ReleaseURL:    strings.TrimSpace(release.ReleaseURL),
		PublishedAt:   release.PublishedAt.UTC(),
		Assets:        cloneReleaseAssets(release.Assets),
		CheckedAt:     checkedAt.UTC(),
	}
	if strings.TrimSpace(entry.LatestVersion) == "" {
		return cacheEntry{}, errors.New("update: cached latest version is required")
	}
	if entry.CheckedAt.IsZero() {
		return cacheEntry{}, errors.New("update: cached checked_at is required")
	}
	if len(entry.Assets) == 0 {
		return cacheEntry{}, errors.New("update: cached release assets are required")
	}
	return entry, nil
}

func (entry cacheEntry) release() *Release {
	return &Release{
		Version:     strings.TrimSpace(entry.LatestVersion),
		ReleaseURL:  strings.TrimSpace(entry.ReleaseURL),
		PublishedAt: entry.PublishedAt.UTC(),
		Assets:      cloneReleaseAssets(entry.Assets),
	}
}

func cloneReleaseAssets(assets []ReleaseAsset) []ReleaseAsset {
	if len(assets) == 0 {
		return nil
	}
	cloned := make([]ReleaseAsset, 0, len(assets))
	for _, asset := range assets {
		cloned = append(cloned, ReleaseAsset{
			Name:        strings.TrimSpace(asset.Name),
			DownloadURL: strings.TrimSpace(asset.DownloadURL),
		})
	}
	return cloned
}
