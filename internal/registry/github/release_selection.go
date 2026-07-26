package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"mime"
	"net/http"

	"slices"

	"strings"

	"github.com/compozy/agh/internal/registry"
)

func parseRepoSlug(slug string) (repoSlug, error) {
	trimmed := strings.TrimSpace(slug)
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 {
		return repoSlug{}, fmt.Errorf("github: slug %q must be in owner/repo format", trimmed)
	}

	owner := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if owner == "" || name == "" {
		return repoSlug{}, fmt.Errorf("github: slug %q must be in owner/repo format", trimmed)
	}

	return repoSlug{
		owner: owner,
		name:  name,
		full:  owner + "/" + name,
	}, nil
}

func filterPublishedReleases(releases []release) []release {
	filtered := make([]release, 0, len(releases))
	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		filtered = append(filtered, release)
	}
	return filtered
}

func releaseVersions(releases []release) []string {
	versions := make([]string, 0, len(releases))
	for _, release := range releases {
		if tag := strings.TrimSpace(release.TagName); tag != "" {
			versions = append(versions, tag)
		}
	}
	return versions
}

func releaseDownloadCount(release *release) int {
	if release == nil {
		return 0
	}
	total := 0
	for _, asset := range release.Assets {
		total += asset.DownloadCount
	}
	return total
}

func releaseDescription(release *release) string {
	if release == nil {
		return ""
	}
	if name := strings.TrimSpace(release.Name); name != "" {
		return name
	}
	body := strings.TrimSpace(release.Body)
	if body == "" {
		return ""
	}
	line, _, _ := strings.Cut(body, "\n")
	return strings.TrimSpace(line)
}

func selectReleaseDownload(release *release, requestedAsset string) (releaseSelection, error) {
	if release == nil {
		return releaseSelection{}, errors.New("release metadata is required")
	}

	candidates := make([]releaseAsset, 0, len(release.Assets))
	for _, asset := range release.Assets {
		if strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.Name)), ".tar.gz") {
			candidates = append(candidates, asset)
		}
	}

	if requestedAsset != "" {
		for _, asset := range release.Assets {
			if strings.TrimSpace(asset.Name) != requestedAsset {
				continue
			}
			if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(asset.Name)), ".tar.gz") {
				return releaseSelection{}, fmt.Errorf("asset %q is not a .tar.gz archive", requestedAsset)
			}
			selected := asset
			return releaseSelection{asset: &selected}, nil
		}
		return releaseSelection{}, fmt.Errorf(
			"asset %q not found; available assets: %s",
			requestedAsset,
			strings.Join(assetNames(release.Assets), ", "),
		)
	}

	switch len(candidates) {
	case 0:
		if strings.TrimSpace(release.TarballURL) == "" {
			return releaseSelection{}, errors.New("release has no .tar.gz assets and no source archive fallback")
		}
		return releaseSelection{useTarball: true}, nil
	case 1:
		selected := candidates[0]
		return releaseSelection{asset: &selected}, nil
	default:
		return releaseSelection{}, fmt.Errorf(
			"multiple .tar.gz assets found: %s; specify one with --asset",
			strings.Join(assetNames(candidates), ", "),
		)
	}
}

func assetNames[T interface{ GetName() string }](assets []T) []string {
	names := make([]string, 0, len(assets))
	for _, asset := range assets {
		names = append(names, asset.GetName())
	}
	slices.Sort(names)
	return names
}

func (a releaseAsset) GetName() string {
	return strings.TrimSpace(a.Name)
}

func validateDownloadContentType(contentType string) error {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return errors.New("github: download missing Content-Type header")
	}

	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err != nil {
		return fmt.Errorf("github: parse Content-Type %q: %w", trimmed, err)
	}

	switch mediaType {
	case clientApplicationGzipPath, clientApplicationXGzipPath, "application/octet-stream":
		return nil
	default:
		return fmt.Errorf("github: unexpected download content type %q", trimmed)
	}
}

func privateRepositoryError(slug string) error {
	return fmt.Errorf(
		"github: repository %q requires authentication; set GITHUB_TOKEN to access private repositories",
		slug,
	)
}

func repositoryNotFoundError(slug string) error {
	return fmt.Errorf("github: repository %q not found: %w", slug, registry.NewPackageNotFoundError(slug))
}

func responseError(response *http.Response, operation string, slug string) error {
	message := readErrorMessage(response.Body)
	if message == "" {
		return fmt.Errorf("github: %s request failed for %q: %s", operation, slug, response.Status)
	}
	return fmt.Errorf("github: %s request failed for %q: %s: %s", operation, slug, response.Status, message)
}

func readErrorMessage(body io.Reader) string {
	payload, err := io.ReadAll(io.LimitReader(body, maxErrorBodyBytes))
	if err != nil {
		return ""
	}

	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return ""
	}

	var envelope struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err == nil {
		for _, candidate := range []string{envelope.Message, envelope.Error} {
			if candidate = strings.TrimSpace(candidate); candidate != "" {
				return candidate
			}
		}
	}

	return trimmed
}

func closeResponseBody(body io.Closer, context string) error {
	if body == nil {
		return nil
	}
	if err := body.Close(); err != nil {
		return fmt.Errorf("github: close %s: %w", context, err)
	}
	return nil
}
