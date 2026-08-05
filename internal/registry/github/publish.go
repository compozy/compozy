package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const publishResponseLimit = 1 << 20

var ErrPublishCredentialUnavailable = errors.New("github: publish credential unavailable")

// ReleaseUploader uploads a release archive and its SHA-256 sidecar.
type ReleaseUploader interface {
	UploadRelease(context.Context, ReleaseUploadRequest) (ReleaseUploadResult, error)
}

// ReleaseUploadRequest contains publishable release data. Credentials remain
// bound to the uploader and never enter this request.
type ReleaseUploadRequest struct {
	Repository    string
	TagName       string
	Draft         bool
	AssetName     string
	Archive       []byte
	DigestSidecar []byte
}

// ReleaseUploadResult reports the published release and archive locations.
type ReleaseUploadResult struct {
	ReleaseURL string
	AssetURL   string
}

var _ ReleaseUploader = (*Client)(nil)

// UploadRelease creates or updates a GitHub release and uploads its archive
// plus digest sidecar. Authentication is carried only by the client.
func (c *Client) UploadRelease(
	ctx context.Context,
	request ReleaseUploadRequest,
) (ReleaseUploadResult, error) {
	if strings.TrimSpace(c.token) == "" {
		return ReleaseUploadResult{}, fmt.Errorf(
			"%w: configure GITHUB_TOKEN or vault:github/publish",
			ErrPublishCredentialUnavailable,
		)
	}
	repo, err := parseRepoSlug(request.Repository)
	if err != nil {
		return ReleaseUploadResult{}, err
	}
	if err := validateReleaseUploadRequest(request); err != nil {
		return ReleaseUploadResult{}, err
	}

	release, err := c.ensureRelease(ctx, repo, request.TagName, request.Draft)
	if err != nil {
		return ReleaseUploadResult{}, err
	}
	if err := c.deleteConflictingAssets(ctx, repo, release, request.AssetName); err != nil {
		return ReleaseUploadResult{}, err
	}

	asset, err := c.uploadReleaseAsset(ctx, repo, release, request.AssetName, "application/gzip", request.Archive)
	if err != nil {
		return ReleaseUploadResult{}, err
	}
	_, sidecarErr := c.uploadReleaseAsset(
		ctx,
		repo,
		release,
		request.AssetName+".sha256",
		"text/plain; charset=utf-8",
		request.DigestSidecar,
	)
	if sidecarErr != nil {
		cleanupErr := c.deleteReleaseAsset(ctx, repo, asset.ID)
		return ReleaseUploadResult{}, errors.Join(sidecarErr, cleanupErr)
	}

	return ReleaseUploadResult{
		ReleaseURL: strings.TrimSpace(release.HTMLURL),
		AssetURL:   strings.TrimSpace(asset.BrowserDownloadURL),
	}, nil
}

func validateReleaseUploadRequest(request ReleaseUploadRequest) error {
	switch {
	case strings.TrimSpace(request.TagName) == "":
		return errors.New("github: release tag is required")
	case strings.TrimSpace(request.AssetName) == "":
		return errors.New("github: release asset name is required")
	case len(request.Archive) == 0:
		return errors.New("github: release archive is required")
	case len(request.DigestSidecar) == 0:
		return errors.New("github: release digest sidecar is required")
	default:
		return nil
	}
}

func (c *Client) ensureRelease(ctx context.Context, repo repoSlug, tag string, draft bool) (*release, error) {
	existing, found, err := c.findPublishRelease(ctx, repo, tag)
	if err != nil {
		return nil, err
	}
	payload := struct {
		TagName    string `json:"tag_name"`
		Name       string `json:"name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}{TagName: tag, Name: tag, Draft: draft}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("github: encode release %q for %q: %w", tag, repo.full, err)
	}

	method := http.MethodPost
	endpoint := c.repositoryEndpoint(repo) + "/releases"
	if found {
		method = http.MethodPatch
		endpoint += "/" + strconv.FormatInt(existing.ID, 10)
	}
	//nolint:bodyclose // decodePublishResponse owns and closes every returned response body.
	response, err := c.doMutationRequest(ctx, method, endpoint, "application/json", encoded)
	if err != nil {
		return nil, fmt.Errorf("github: publish release %q for %q: %w", tag, repo.full, err)
	}
	return decodePublishResponse[release](response, "publish release", repo.full)
}

func (c *Client) findPublishRelease(
	ctx context.Context,
	repo repoSlug,
	tag string,
) (_ *release, found bool, err error) {
	endpoint := c.repositoryEndpoint(repo) + "/releases/tags/" + url.PathEscape(strings.TrimSpace(tag))
	//nolint:bodyclose // every status branch below closes or transfers ownership of the response body.
	response, err := c.doRequest(ctx, endpoint, acceptJSON)
	if err != nil {
		return nil, false, err
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, false, closeResponseBody(response.Body, fmt.Sprintf("publish lookup for %q", repo.full))
	}
	if response.StatusCode != http.StatusOK {
		return nil, false, publishStatusError(response, "release lookup", repo.full)
	}
	result, err := decodePublishResponse[release](response, "release lookup", repo.full)
	return result, err == nil, err
}

func (c *Client) deleteConflictingAssets(
	ctx context.Context,
	repo repoSlug,
	release *release,
	assetName string,
) error {
	wanted := map[string]struct{}{assetName: {}, assetName + ".sha256": {}}
	for _, asset := range release.Assets {
		if _, ok := wanted[strings.TrimSpace(asset.Name)]; !ok {
			continue
		}
		if err := c.deleteReleaseAsset(ctx, repo, asset.ID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) deleteReleaseAsset(ctx context.Context, repo repoSlug, assetID int64) error {
	if assetID <= 0 {
		return nil
	}
	endpoint := c.repositoryEndpoint(repo) + "/releases/assets/" + strconv.FormatInt(assetID, 10)
	//nolint:bodyclose // every status branch below closes the response body.
	response, err := c.doMutationRequest(ctx, http.MethodDelete, endpoint, "", nil)
	if err != nil {
		return fmt.Errorf("github: delete release asset for %q: %w", repo.full, err)
	}
	if response.StatusCode != http.StatusNoContent {
		return publishStatusError(response, "delete release asset", repo.full)
	}
	return closeResponseBody(response.Body, fmt.Sprintf("release asset deletion for %q", repo.full))
}

func (c *Client) uploadReleaseAsset(
	ctx context.Context,
	repo repoSlug,
	release *release,
	name string,
	contentType string,
	payload []byte,
) (*releaseAsset, error) {
	endpoint, err := releaseAssetUploadURL(release.UploadURL, name)
	if err != nil {
		return nil, fmt.Errorf("github: resolve release upload URL for %q: %w", repo.full, err)
	}
	//nolint:bodyclose // decodePublishResponse owns and closes every returned response body.
	response, err := c.doMutationRequest(ctx, http.MethodPost, endpoint, contentType, payload)
	if err != nil {
		return nil, fmt.Errorf("github: upload release asset %q for %q: %w", name, repo.full, err)
	}
	return decodePublishResponse[releaseAsset](response, "upload release asset", repo.full)
}

func (c *Client) doMutationRequest(
	ctx context.Context,
	method string,
	rawURL string,
	contentType string,
	payload []byte,
) (*http.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("github: request aborted: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("github: create %s request: %w", method, err)
	}
	request.Header.Set("Accept", acceptJSON)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if token := strings.TrimSpace(c.token); token != "" && c.networkPolicy.IsTrustedOrigin(request.URL) {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("github: %s request failed: %w", method, err)
	}
	if err := c.checkRateLimit(response); err != nil {
		return nil, err
	}
	return response, nil
}

func decodePublishResponse[T any](response *http.Response, operation string, repo string) (_ *T, err error) {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, publishStatusError(response, operation, repo)
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, operation+" response for "+repo))
	}()
	payload, err := io.ReadAll(io.LimitReader(response.Body, publishResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("github: read %s response for %q: %w", operation, repo, err)
	}
	if len(payload) > publishResponseLimit {
		return nil, fmt.Errorf("github: %s response for %q exceeds %d bytes", operation, repo, publishResponseLimit)
	}
	var result T
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("github: decode %s response for %q: %w", operation, repo, err)
	}
	return &result, nil
}

func publishStatusError(response *http.Response, operation string, repo string) error {
	status := response.Status
	closeErr := closeResponseBody(response.Body, operation+" response for "+repo)
	return errors.Join(fmt.Errorf("github: %s failed for %q: %s", operation, repo, status), closeErr)
}

func releaseAssetUploadURL(template string, name string) (string, error) {
	rawURL := strings.Split(strings.TrimSpace(template), "{")[0]
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("unsupported scheme %q", parsed.Scheme)
	}
	query := parsed.Query()
	query.Set("name", name)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (c *Client) repositoryEndpoint(repo repoSlug) string {
	return c.baseURL + "/repos/" + url.PathEscape(repo.owner) + "/" + url.PathEscape(repo.name)
}
