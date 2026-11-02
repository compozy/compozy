package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	c "github.com/compozy/compozy/internal/extension/contract"
)

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
	q := url.Values{"q": {fmt.Sprintf("repo:%s/%s is:pr %s", repo.owner, repo.name, strings.TrimSpace(request.Query))}, "per_page": {fmt.Sprint(limit)}, "page": {fmt.Sprint(page)}, "sort": {"updated"}, "order": {"desc"}}
	var response struct {
		Total int `json:"total_count"`
		Items []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"html_url"`
			State  string `json:"state"`
		} `json:"items"`
	}
	if err := p.api.request(ctx, http.MethodGet, "/search/issues?"+q.Encode(), credential.token, nil, &response); err != nil {
		return result, err
	}
	// Search returns issue identities; resolve branches only for this bounded page.
	for _, item := range response.Items {
		candidate, err := p.ReadPR(ctx, c.ForgePRReadRequest{RemoteURLs: request.RemoteURLs, Number: item.Number})
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, candidate)
	}
	result.HasMore = page*limit < response.Total
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
	if err := p.api.request(ctx, http.MethodGet, repositoryAPIPath(repo, fmt.Sprintf("/pulls/%d", request.Number)), credential.token, nil, &pull); err != nil {
		return c.ForgePRCandidate{}, err
	}
	state := pull.State
	if pull.MergedAt != nil {
		state = "merged"
	}
	return c.ForgePRCandidate{Number: pull.Number, Title: pull.Title, URL: pull.HTMLURL, Branch: pull.Head.Ref, State: state}, nil
}
