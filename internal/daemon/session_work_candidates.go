package daemon

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	c "github.com/compozy/compozy/internal/extension/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/worktree"
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
	return a.linkRepository(ctx, workspaceID, ws.Root)
}

func (a sessionPullRequestAdapter) WorkPR(
	ctx context.Context,
	workspaceID string,
	number int,
) (session.WorkCandidate, error) {
	repo, err := a.candidateRepository(ctx, workspaceID)
	if err != nil {
		return session.WorkCandidate{}, err
	}
	if strings.HasPrefix(repo, "local:") {
		return session.WorkCandidate{}, session.ErrLinkedWorkInvalid
	}
	if a.runtime == nil {
		return session.WorkCandidate{}, errors.New("forge discovery unavailable")
	}
	runtime, ok := a.runtime().(forgeDiscoveryRuntime)
	if !ok {
		return session.WorkCandidate{}, errors.New("forge discovery unavailable")
	}
	pr, err := runtime.ForgeReadPR(
		ctx,
		c.ForgePRReadRequest{RemoteURLs: []string{"https://" + repo + ".git"}, Number: number},
	)
	if err != nil {
		return session.WorkCandidate{}, err
	}
	return session.WorkCandidate{
		Repository:     repo,
		HeadRepository: pr.HeadRepository,
		Branch:         pr.Branch,
		PRNumber:       pr.Number,
		Title:          pr.Title,
		URL:            pr.URL,
		State:          pr.State,
	}, nil
}

func (a sessionPullRequestAdapter) WorkCandidates(
	ctx context.Context,
	workspaceID, query, kind string,
	page, limit int,
) (session.WorkCandidates, error) {
	result := session.WorkCandidates{Items: []session.WorkCandidate{}}
	if kind != "" && kind != "branches" && kind != "pull_requests" {
		return result, fmt.Errorf("%w: candidate kind %q", session.ErrLinkedWorkInvalid, kind)
	}
	repo, err := a.candidateRepository(ctx, workspaceID)
	if err != nil {
		return result, err
	}
	number, err := workCandidateNumber(query, repo)
	if err != nil {
		return result, err
	}
	if a.runtime != nil && !strings.HasPrefix(repo, "local:") {
		if runtime, ok := a.runtime().(forgeDiscoveryRuntime); ok {
			capabilities, err := runtime.ForgeCapabilities(ctx, []string{"https://" + repo + ".git"})
			if err == nil && capabilities.Available && capabilities.SupportsPRDiscovery && kind != "branches" {
				return a.searchWorkPRs(ctx, runtime, workspaceID, repo, number, c.ForgePRSearchRequest{
					RemoteURLs: []string{"https://" + repo + ".git"}, Query: query, Page: page, Limit: limit,
				})
			}
		}
	}
	// Enumerate local refs once; substring filtering must cross branch path separators.
	ws, err := a.resolver.ResolveWorktreeWorkspace(ctx, workspaceID, "")
	if err != nil {
		return result, err
	}
	if strings.ContainsAny(query, "\x00\r\n") {
		return result, errors.New("invalid branch search")
	}
	runner := a.runner
	output, _, err := runner.Run(
		ctx,
		ws.Root,
		"for-each-ref",
		"--format=%(refname:short)",
		"refs/heads/",
	)
	if err != nil {
		return result, err
	}
	branches := make([]string, 0, page*limit+1)
	for branch := range strings.FieldsSeq(string(output)) {
		if strings.Contains(strings.ToLower(branch), strings.ToLower(query)) {
			branches = append(branches, branch)
			if len(branches) >= page*limit+1 {
				break
			}
		}
	}
	start := min((page-1)*limit, len(branches))
	end := min(start+limit, len(branches))
	for _, branch := range branches[start:end] {
		result.Items = append(result.Items, session.WorkCandidate{Repository: repo, Branch: branch})
	}
	result.HasMore = len(branches) > end
	return result, nil
}

func (a sessionPullRequestAdapter) searchWorkPRs(
	ctx context.Context, runtime forgeDiscoveryRuntime, workspaceID, repo string,
	number int, request c.ForgePRSearchRequest,
) (session.WorkCandidates, error) {
	result := session.WorkCandidates{Items: []session.WorkCandidate{}, SupportsPRDiscovery: true}
	if number > 0 && request.Page == 1 {
		candidate, readErr := a.WorkPR(ctx, workspaceID, number)
		// Providers do not share a not-found error; normal search remains the fallback.
		if readErr == nil && candidate.PRNumber == number && candidate.Branch != "" {
			result.Items = append(result.Items, candidate)
		}
	}
	response, err := runtime.ForgeSearchPRs(ctx, request)
	if err != nil {
		if len(result.Items) > 0 {
			return result, nil
		}
		return result, err
	}
	for _, pr := range response.Items {
		if number > 0 && pr.Number == number &&
			(request.Page > 1 || len(result.Items) > 0 && result.Items[0].PRNumber == number) {
			continue
		}
		result.Items = append(
			result.Items,
			session.WorkCandidate{
				Repository:     repo,
				HeadRepository: pr.HeadRepository,
				Branch:         pr.Branch,
				PRNumber:       pr.Number,
				Title:          pr.Title,
				URL:            pr.URL,
				State:          pr.State,
			},
		)
	}
	result.HasMore = response.HasMore
	return result, nil
}

var workCandidatePRPath = regexp.MustCompile(
	`^(.+?)/(?:-/)?(?:pull|pulls|merge_requests|pull-requests)/([0-9]+)(?:/.*)?$`,
)

func workCandidateNumber(query, repository string) (int, error) {
	query = strings.TrimSpace(query)
	if number, err := strconv.Atoi(strings.TrimPrefix(query, "#")); err == nil && number > 0 {
		return number, nil
	}
	parsed, err := url.Parse(query)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return 0, nil
	}
	match := workCandidatePRPath.FindStringSubmatch(parsed.Path)
	if match == nil {
		return 0, nil
	}
	parsed.Path, parsed.RawPath, parsed.RawQuery, parsed.Fragment = match[1], "", "", ""
	repo, err := worktree.NormalizeRepoKey(parsed.String())
	if err != nil || repo != repository {
		return 0, session.ErrLinkedWorkRepository
	}
	number, err := strconv.Atoi(match[2])
	if err != nil || number < 1 {
		return 0, session.ErrLinkedWorkInvalid
	}
	return number, nil
}
