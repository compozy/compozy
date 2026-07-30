package github

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxDigestSidecarBytes = 4 << 10

func (c *Client) fetchReleaseDigestSidecar(
	ctx context.Context,
	repo repoSlug,
	release *release,
	selection releaseSelection,
) (_ string, err error) {
	if release == nil || selection.asset == nil {
		return "", nil
	}
	sidecar := findReleaseAsset(release.Assets, selection.asset.Name+".sha256")
	if sidecar == nil {
		return "", nil
	}
	endpoint := firstNonEmpty(sidecar.URL, sidecar.BrowserDownloadURL)
	if endpoint == "" {
		return "", fmt.Errorf("github: digest sidecar URL is missing for %q", repo.full)
	}
	response, err := c.doRequest(ctx, endpoint, acceptBinary)
	if err != nil {
		return "", fmt.Errorf("github: download digest sidecar for %q: %w", repo.full, err)
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, fmt.Sprintf("digest sidecar for %q", repo.full)))
	}()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", responseError(response, "digest sidecar download", repo.full)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, maxDigestSidecarBytes+1))
	if err != nil {
		return "", fmt.Errorf("github: read digest sidecar for %q: %w", repo.full, err)
	}
	if len(payload) > maxDigestSidecarBytes {
		return "", fmt.Errorf("github: digest sidecar for %q exceeds %d bytes", repo.full, maxDigestSidecarBytes)
	}
	return parseDigestSidecar(payload, sidecar.Name)
}

func findReleaseAsset(assets []releaseAsset, name string) *releaseAsset {
	wanted := strings.TrimSpace(name)
	for index := range assets {
		if strings.TrimSpace(assets[index].Name) == wanted {
			return &assets[index]
		}
	}
	return nil
}

func parseDigestSidecar(payload []byte, name string) (string, error) {
	fields := strings.Fields(string(payload))
	if len(fields) == 0 {
		return "", fmt.Errorf("github: digest sidecar %q is empty", strings.TrimSpace(name))
	}
	digest := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(fields[0]), "sha256:"))
	decoded, err := hex.DecodeString(digest)
	if err != nil || len(decoded) != 32 {
		return "", fmt.Errorf("github: digest sidecar %q must contain a 64-character SHA-256 digest", name)
	}
	return digest, nil
}
