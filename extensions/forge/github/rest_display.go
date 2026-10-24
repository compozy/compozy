package github

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

const pullSelectionIncomplete = "incomplete"

func (p *Provider) restDisplayStatus(
	ctx context.Context,
	repo repository,
	branch, token string,
) (extensioncontract.ForgeStatusResponse, error) {
	response := extensioncontract.ForgeStatusResponse{
		Provider:  githubProviderName,
		FetchedAt: p.now().UTC(),
		Selection: "complete",
	}
	for _, state := range []string{"open", "closed"} {
		path, err := url.Parse(pullListPath(repo, branch, state))
		if err != nil {
			return response, err
		}
		query := path.Query()
		query.Set("sort", "updated")
		query.Set("direction", "desc")
		path.RawQuery = query.Encode()
		var pulls []pullPayload
		if err := p.api.request(ctx, http.MethodGet, path.String(), token, nil, &pulls); err != nil {
			if cause := githubErrorCause(err); cause != "" {
				response.Cause = cause
				if failure, ok := errors.AsType[*githubAPIError](err); ok {
					response.RetryAfter = failure.retryAfter
				}
				return response, nil
			}
			return response, err
		}
		var selected *pullPayload
		for i := range pulls {
			pull := &pulls[i]
			if pull.Head.Ref == "" {
				response.Selection = pullSelectionIncomplete
				return response, nil
			}
			if pull.Head.Ref != branch {
				continue
			}
			if pull.Head.Repo == nil || pull.Head.Repo.FullName == "" || pull.UpdatedAt.IsZero() {
				response.Selection = pullSelectionIncomplete
				return response, nil
			}
			if pull.State != state || !strings.EqualFold(pull.Head.Repo.FullName, repo.owner+"/"+repo.name) {
				continue
			}
			if selected == nil || pull.UpdatedAt.After(selected.UpdatedAt) {
				selected = pull
			}
		}
		if selected != nil {
			state, merged := selected.State, selected.MergedAt != nil
			if merged {
				state = "merged"
			}
			response.PRNumber, response.PRState = &selected.Number, &state
			response.PRURL, response.Merged = selected.HTMLURL, &merged
			response.Head, response.Base = selected.Head.Ref, selected.Base.Ref
			response.HeadSHA, response.Draft = selected.Head.SHA, selected.Draft
			response.UpdatedAt = &selected.UpdatedAt
			return response, nil
		}
		if len(pulls) >= githubPullPageSize {
			response.Selection = pullSelectionIncomplete
			return response, nil
		}
	}
	return response, nil
}
