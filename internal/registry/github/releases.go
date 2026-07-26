package github

import (
	"context"
	"encoding/json"

	"fmt"
	"io"

	"net/http"
	"net/url"
)

func (c *Client) fetchLatestRelease(ctx context.Context, repo repoSlug) (_ *release, err error) {
	endpoint := c.baseURL + "/repos/" + url.PathEscape(
		repo.owner,
	) + "/" + url.PathEscape(
		repo.name,
	) + "/releases/latest"
	response, err := c.doRequest(ctx, endpoint, acceptJSON)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = joinErrors(
				err,
				fmt.Errorf("github: close latest release response for %q: %w", repo.full, closeErr),
			)
		}
	}()

	switch response.StatusCode {
	case http.StatusOK:
		payload, err := io.ReadAll(response.Body)
		closeErr := closeResponseBody(response.Body, fmt.Sprintf("latest release response for %q", repo.full))
		if err != nil {
			return nil, joinErrors(
				fmt.Errorf("github: read latest release for %q: %w", repo.full, err),
				closeErr,
			)
		}
		if closeErr != nil {
			return nil, closeErr
		}

		var latest release
		if err := json.Unmarshal(payload, &latest); err != nil {
			return nil, fmt.Errorf("github: decode latest release for %q: %w", repo.full, err)
		}
		return &latest, nil
	case http.StatusUnauthorized:
		closeErr := closeResponseBody(
			response.Body,
			fmt.Sprintf("latest release response for %q", repo.full),
		)
		if closeErr != nil {
			return nil, joinErrors(privateRepositoryError(repo.full), closeErr)
		}
		return nil, privateRepositoryError(repo.full)
	case http.StatusNotFound:
		if err := closeResponseBody(
			response.Body,
			fmt.Sprintf("latest release response for %q", repo.full),
		); err != nil {
			return nil, err
		}
		releases, listErr := c.fetchReleasePage(ctx, repo)
		if listErr != nil {
			return nil, listErr
		}
		if len(releases) == 0 {
			return nil, fmt.Errorf("github: repository %q has no published releases", repo.full)
		}
		latest := releases[0]
		return &latest, nil
	default:
		err := responseError(response, "latest release", repo.full)
		return nil, joinErrors(
			err,
			closeResponseBody(response.Body, fmt.Sprintf("latest release response for %q", repo.full)),
		)
	}
}

func (c *Client) fetchRequestedRelease(
	ctx context.Context,
	repo repoSlug,
	version string,
) (_ *release, err error) {
	if version == "" {
		return c.fetchLatestRelease(ctx, repo)
	}

	endpoint := c.baseURL + "/repos/" + url.PathEscape(
		repo.owner,
	) + "/" + url.PathEscape(
		repo.name,
	) + "/releases/tags/" + url.PathEscape(
		version,
	)
	response, err := c.doRequest(ctx, endpoint, acceptJSON)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = joinErrors(
				err,
				fmt.Errorf("github: close release response for %q at %q: %w", repo.full, version, closeErr),
			)
		}
	}()

	switch response.StatusCode {
	case http.StatusOK:
		payload, err := io.ReadAll(response.Body)
		closeErr := closeResponseBody(response.Body, fmt.Sprintf("release response for %q at %q", repo.full, version))
		if err != nil {
			return nil, joinErrors(
				fmt.Errorf("github: read release %q for %q: %w", version, repo.full, err),
				closeErr,
			)
		}
		if closeErr != nil {
			return nil, closeErr
		}

		var result release
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil, fmt.Errorf("github: decode release %q for %q: %w", version, repo.full, err)
		}
		if result.Draft || result.Prerelease {
			return nil, fmt.Errorf("github: release %q for %q is not a published full release", version, repo.full)
		}
		return &result, nil
	case http.StatusUnauthorized:
		closeErr := closeResponseBody(response.Body, fmt.Sprintf("release response for %q at %q", repo.full, version))
		if closeErr != nil {
			return nil, joinErrors(privateRepositoryError(repo.full), closeErr)
		}
		return nil, privateRepositoryError(repo.full)
	case http.StatusNotFound:
		if err := closeResponseBody(
			response.Body,
			fmt.Sprintf("release response for %q at %q", repo.full, version),
		); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("github: release %q not found for repository %q", version, repo.full)
	default:
		err := responseError(response, "release lookup", repo.full)
		return nil, joinErrors(
			err,
			closeResponseBody(response.Body, fmt.Sprintf("release response for %q at %q", repo.full, version)),
		)
	}
}

func (c *Client) fetchReleasePage(ctx context.Context, repo repoSlug) (_ []release, err error) {
	endpoint := c.baseURL + "/repos/" + url.PathEscape(
		repo.owner,
	) + "/" + url.PathEscape(
		repo.name,
	) + "/releases?per_page=30&page=1"
	response, err := c.doRequest(ctx, endpoint, acceptJSON)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			err = joinErrors(
				err,
				fmt.Errorf("github: close releases page response for %q: %w", repo.full, closeErr),
			)
		}
	}()

	switch response.StatusCode {
	case http.StatusOK:
		payload, err := io.ReadAll(response.Body)
		closeErr := closeResponseBody(response.Body, fmt.Sprintf("releases page response for %q", repo.full))
		if err != nil {
			return nil, joinErrors(
				fmt.Errorf("github: read releases page for %q: %w", repo.full, err),
				closeErr,
			)
		}
		if closeErr != nil {
			return nil, closeErr
		}

		var releases []release
		if err := json.Unmarshal(payload, &releases); err != nil {
			return nil, fmt.Errorf("github: decode releases page for %q: %w", repo.full, err)
		}
		return filterPublishedReleases(releases), nil
	case http.StatusUnauthorized:
		closeErr := closeResponseBody(response.Body, fmt.Sprintf("releases page response for %q", repo.full))
		if closeErr != nil {
			return nil, joinErrors(privateRepositoryError(repo.full), closeErr)
		}
		return nil, privateRepositoryError(repo.full)
	case http.StatusNotFound:
		if err := closeResponseBody(
			response.Body,
			fmt.Sprintf("releases page response for %q", repo.full),
		); err != nil {
			return nil, err
		}
		return nil, repositoryNotFoundError(repo.full)
	default:
		err := responseError(response, "releases page", repo.full)
		return nil, joinErrors(
			err,
			closeResponseBody(response.Body, fmt.Sprintf("releases page response for %q", repo.full)),
		)
	}
}
