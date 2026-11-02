package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	c "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/session"
)

type forgeDiscoveryRuntime interface {
	ForgeCapabilities(context.Context, []string) (c.ForgeCapabilitiesResponse, error)
	ForgeSearchPRs(context.Context, c.ForgePRSearchRequest) (c.ForgePRSearchResponse, error)
	ForgeReadPR(context.Context, c.ForgePRReadRequest) (c.ForgePRCandidate, error)
}

func (a sessionPullRequestAdapter) candidateRepository(ctx context.Context, workspaceID string) (string, error) {
	ws, err := a.resolver.ResolveWorktreeWorkspace(ctx, workspaceID, "")
	if err != nil {
		return "", err
	}
	return a.service.RepoKey(ctx, ws.Root)
}
func (a sessionPullRequestAdapter) WorkPR(ctx context.Context, workspaceID string, number int) (session.WorkCandidate, error) {
	repo, err := a.candidateRepository(ctx, workspaceID)
	if err != nil {
		return session.WorkCandidate{}, err
	}
	if a.runtime == nil {
		return session.WorkCandidate{}, errors.New("forge discovery unavailable")
	}
	runtime, ok := a.runtime().(forgeDiscoveryRuntime)
	if !ok {
		return session.WorkCandidate{}, errors.New("forge discovery unavailable")
	}
	pr, err := runtime.ForgeReadPR(ctx, c.ForgePRReadRequest{RemoteURLs: []string{"https://" + repo + ".git"}, Number: number})
	if err != nil {
		return session.WorkCandidate{}, err
	}
	return session.WorkCandidate{Repository: repo, Branch: pr.Branch, PRNumber: pr.Number, Title: pr.Title, URL: pr.URL, State: pr.State}, nil
}
func (a sessionPullRequestAdapter) WorkCandidates(ctx context.Context, workspaceID, query, kind string, page, limit int) (session.WorkCandidates, error) {
	result := session.WorkCandidates{Items: []session.WorkCandidate{}}
	if kind != "" && kind != "branches" && kind != "pull_requests" {
		return result, errors.New("invalid candidate kind")
	}
	repo, err := a.candidateRepository(ctx, workspaceID)
	if err != nil {
		return result, err
	}
	if a.runtime != nil {
		if runtime, ok := a.runtime().(forgeDiscoveryRuntime); ok {
			capabilities, err := runtime.ForgeCapabilities(ctx, []string{"https://" + repo + ".git"})
			if err == nil && capabilities.Available && capabilities.SupportsPRDiscovery && kind != "branches" {
				result.SupportsPRDiscovery = true
				response, err := runtime.ForgeSearchPRs(ctx, c.ForgePRSearchRequest{RemoteURLs: []string{"https://" + repo + ".git"}, Query: query, Page: page, Limit: limit})
				if err != nil {
					return result, err
				}
				for _, pr := range response.Items {
					result.Items = append(result.Items, session.WorkCandidate{Repository: repo, Branch: pr.Branch, PRNumber: pr.Number, Title: pr.Title, URL: pr.URL, State: pr.State})
				}
				result.HasMore = response.HasMore
				return result, nil
			}
		}
	}
	// Git performs filtering and paging before candidate materialization.
	ws, err := a.resolver.ResolveWorktreeWorkspace(ctx, workspaceID, "")
	if err != nil {
		return result, err
	}
	if strings.ContainsAny(query, "\x00\r\n") {
		return result, errors.New("invalid branch search")
	}
	runner := a.runner
	output, _, err := runner.Run(ctx, ws.Root, "for-each-ref", "--format=%(refname:short)", "--count="+fmt.Sprint(page*limit+1), "refs/heads/*"+query+"*")
	if err != nil {
		return result, err
	}
	branches := strings.Fields(string(output))
	start := min((page-1)*limit, len(branches))
	end := min(start+limit, len(branches))
	for _, branch := range branches[start:end] {
		result.Items = append(result.Items, session.WorkCandidate{Repository: repo, Branch: branch})
	}
	result.HasMore = len(branches) > end
	return result, nil
}
