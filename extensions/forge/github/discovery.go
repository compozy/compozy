package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	c "github.com/compozy/compozy/internal/extension/contract"
)

const prSearchQueryKey = "query"

// One request returns the complete bounded page, so a search never reads PRs one by one.
const prSearchQuery = `query($query:String!,$first:Int!,$after:String){
  search(query:$query,type:ISSUE,first:$first,after:$after){
    nodes{... on PullRequest{number title url state headRefName headRepository{nameWithOwner}}}
    pageInfo{hasNextPage}
  }
}`

func (p *Provider) SearchPRs(ctx context.Context, request c.ForgePRSearchRequest) (c.ForgePRSearchResponse, error) {
	result := c.ForgePRSearchResponse{Items: []c.ForgePRCandidate{}}
	repo, ok := selectRepository(request.RemoteURLs)
	if !ok {
		return result, errors.New("unsupported repository")
	}
	credential, err := p.credentials.resolve(ctx)
	if err != nil {
		return result, err
	}
	if credential.token == "" {
		return result, errors.New("forge credentials unavailable")
	}
	limit := request.Limit
	if limit < 1 || limit > 100 {
		limit = 30
	}
	page := max(1, request.Page)
	// GraphQL returns the complete bounded page in one request. Preserve usable
	// nodes when GitHub reports an error on another node.
	var response struct {
		Data struct {
			Search struct {
				Nodes []struct {
					Number                         int
					Title, URL, State, HeadRefName string
					HeadRepository                 *struct{ NameWithOwner string }
				}
				PageInfo struct{ HasNextPage bool }
			}
		}
		Errors []graphError
	}
	var after *string
	if page > 1 {
		cursor := base64.StdEncoding.EncodeToString([]byte("cursor:" + strconv.Itoa((page-1)*limit)))
		after = &cursor
	}
	variables := map[string]any{
		prSearchQueryKey: fmt.Sprintf(
			"repo:%s/%s is:pr %q sort:updated-desc",
			repo.owner,
			repo.name,
			strings.TrimSpace(request.Query),
		),
		"first": limit,
		"after": after,
	}
	if err := p.api.request(
		ctx,
		http.MethodPost,
		"/graphql",
		credential.token,
		map[string]any{prSearchQueryKey: prSearchQuery, "variables": variables},
		&response,
	); err != nil {
		return result, err
	}
	for _, node := range response.Data.Search.Nodes {
		if node.Number < 1 || node.HeadRefName == "" {
			continue
		}
		head := ""
		if node.HeadRepository != nil {
			head = "github.com/" + strings.ToLower(node.HeadRepository.NameWithOwner)
		}
		result.Items = append(
			result.Items,
			c.ForgePRCandidate{
				Number:         node.Number,
				Title:          node.Title,
				URL:            node.URL,
				State:          strings.ToLower(node.State),
				Branch:         node.HeadRefName,
				HeadRepository: head,
			},
		)
	}
	result.HasMore = response.Data.Search.PageInfo.HasNextPage
	if len(response.Errors) > 0 && len(result.Items) == 0 {
		return result, errors.New("forge pull request search failed")
	}
	return result, nil
}

func (p *Provider) ReadPR(ctx context.Context, request c.ForgePRReadRequest) (c.ForgePRCandidate, error) {
	repo, ok := selectRepository(request.RemoteURLs)
	if !ok || request.Number < 1 {
		return c.ForgePRCandidate{}, errors.New("invalid pull request")
	}
	credential, err := p.credentials.resolve(ctx)
	if err != nil {
		return c.ForgePRCandidate{}, err
	}
	if credential.token == "" {
		return c.ForgePRCandidate{}, errors.New("forge credentials unavailable")
	}
	var pull pullPayload
	if err := p.api.request(
		ctx,
		http.MethodGet,
		repositoryAPIPath(repo, fmt.Sprintf("/pulls/%d", request.Number)),
		credential.token,
		nil,
		&pull,
	); err != nil {
		return c.ForgePRCandidate{}, err
	}
	state := pull.State
	if pull.MergedAt != nil {
		state = prStateMerged
	}
	head := ""
	if pull.Head.Repo != nil {
		head = "github.com/" + strings.ToLower(pull.Head.Repo.FullName)
	}
	return c.ForgePRCandidate{
		HeadRepository: head,
		Number:         pull.Number,
		Title:          pull.Title,
		URL:            pull.HTMLURL,
		Branch:         pull.Head.Ref,
		State:          state,
	}, nil
}
