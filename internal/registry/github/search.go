package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/registry"
)

const (
	githubExtensionTopic = "compozy-extension"
	defaultSearchLimit   = 20
	maxSearchLimit       = 100
)

type repositorySearchResponse struct {
	Items []repositorySearchItem `json:"items"`
}

type repositorySearchItem struct {
	FullName      string   `json:"full_name"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Stargazers    int      `json:"stargazers_count"`
	HTMLURL       string   `json:"html_url"`
	Topics        []string `json:"topics"`
	DefaultBranch string   `json:"default_branch"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

func (c *Client) searchRepositories(
	ctx context.Context,
	query string,
	opts registry.SearchOpts,
) (_ []registry.Listing, err error) {
	if opts.Type != registry.PackageTypeAll && opts.Type != registry.PackageTypeExtension {
		return []registry.Listing{}, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	offset := max(opts.Offset, 0)
	page := offset/limit + 1
	qualifiedQuery := strings.TrimSpace(strings.TrimSpace(query) + " topic:" + githubExtensionTopic)
	parameters := url.Values{
		"q":        {qualifiedQuery},
		"per_page": {strconv.Itoa(limit)},
		"page":     {strconv.Itoa(page)},
		"sort":     {"stars"},
		"order":    {"desc"},
	}
	response, err := c.doRequest(ctx, c.baseURL+"/search/repositories?"+parameters.Encode(), acceptJSON)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, closeResponseBody(response.Body, "repository search response"))
	}()
	if response.StatusCode != http.StatusOK {
		return nil, responseError(response, "repository search", qualifiedQuery)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, publishResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("github: read repository search response: %w", err)
	}
	if len(payload) > publishResponseLimit {
		return nil, fmt.Errorf("github: repository search response exceeds %d bytes", publishResponseLimit)
	}
	var result repositorySearchResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("github: decode repository search response: %w", err)
	}
	listings := make([]registry.Listing, 0, len(result.Items))
	for _, item := range result.Items {
		if !containsTopic(item.Topics, githubExtensionTopic) {
			continue
		}
		listings = append(listings, registry.Listing{
			Slug:        strings.TrimSpace(item.FullName),
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			Author:      strings.TrimSpace(item.Owner.Login),
			Version:     strings.TrimSpace(item.DefaultBranch),
			Downloads:   item.Stargazers,
			Source:      c.Name(),
			Type:        registry.PackageTypeExtension,
		})
	}
	return listings, nil
}

func containsTopic(topics []string, wanted string) bool {
	for _, topic := range topics {
		if strings.EqualFold(strings.TrimSpace(topic), wanted) {
			return true
		}
	}
	return false
}
