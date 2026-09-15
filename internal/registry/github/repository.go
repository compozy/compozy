package github

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/registry"
)

var (
	ErrContentTooLarge = errors.New("github: repository content exceeds size limit")
	ErrRateLimited     = errors.New("github: rate limit exceeded")
	commitPattern      = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

// ResolveCommit pins a branch, tag, or HEAD to the repository's full commit identity.
func (c *Client) ResolveCommit(ctx context.Context, slug, ref string) (string, error) {
	repo, err := parseRepoSlug(slug)
	if err != nil {
		return "", err
	}
	if ref == "" {
		ref = "HEAD"
	}
	endpoint := c.repositoryEndpoint(repo) + "/commits/" + url.PathEscape(ref)
	raw, err := c.readRepositoryContent(ctx, endpoint, "application/vnd.github.sha", 128)
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(raw))
	if !commitPattern.MatchString(commit) {
		return "", errors.New("github: resolved commit must be a full lowercase SHA-1")
	}
	return commit, nil
}

// ReadFile reads bounded raw contents from a pinned repository revision.
func (c *Client) ReadFile(ctx context.Context, slug, commit, path string, maxBytes int64) ([]byte, error) {
	repo, err := parseRepoSlug(slug)
	if err != nil {
		return nil, err
	}
	if !commitPattern.MatchString(commit) || !fs.ValidPath(path) || path == "." || strings.Contains(path, "\\") {
		return nil, errors.New("github: file read requires a pinned commit and confined relative path")
	}
	if maxBytes <= 0 || maxBytes > maxReleaseMetadataBytes {
		return nil, errors.New("github: file read byte limit must be positive and at most 4 MiB")
	}
	var endpoint strings.Builder
	endpoint.WriteString(c.repositoryEndpoint(repo))
	endpoint.WriteString("/contents")
	for part := range strings.SplitSeq(path, "/") {
		endpoint.WriteByte('/')
		endpoint.WriteString(url.PathEscape(part))
	}
	return c.readRepositoryContent(ctx, endpoint.String()+"?ref="+commit, "application/vnd.github.raw+json", maxBytes)
}

func (c *Client) readRepositoryContent(
	ctx context.Context,
	endpoint, accept string,
	maxBytes int64,
) (raw []byte, err error) {
	response, err := c.doRequest(ctx, endpoint, accept)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, drainAndCloseResponseBody(response.Body, "repository content")) }()
	if response.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}
	if response.StatusCode == http.StatusNotFound {
		return nil, registry.NewPackageNotFoundError(endpoint)
	}
	if response.StatusCode != http.StatusOK {
		return nil, responseError(response, "read repository content", "")
	}
	if response.ContentLength > maxBytes {
		return nil, ErrContentTooLarge
	}
	raw, err = io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("github: read repository content: %w", err)
	}
	if int64(len(raw)) > maxBytes {
		return nil, ErrContentTooLarge
	}
	return raw, nil
}

// DownloadRevision returns an owned archive of a pinned commit, independent of release tags.
func (c *Client) DownloadRevision(
	ctx context.Context, slug, commit string, maxBytes int64,
) (*registry.DownloadResult, error) {
	repo, err := parseRepoSlug(slug)
	if err != nil {
		return nil, err
	}
	if !commitPattern.MatchString(commit) {
		return nil, errors.New("github: archive download requires a pinned commit")
	}
	response, err := c.doRequest(ctx, c.repositoryEndpoint(repo)+"/tarball/"+commit, acceptJSON)
	if err != nil {
		return nil, err
	}
	reader, contentType, size, err := finalizeDownloadResponse(
		response,
		repo.full,
		-1,
		normalizeArchiveSizeLimit(maxBytes),
	)
	if err != nil {
		return nil, err
	}
	return &registry.DownloadResult{
		Reader: reader, Slug: repo.full, Version: commit, ContentType: contentType, ContentSize: size,
	}, nil
}
