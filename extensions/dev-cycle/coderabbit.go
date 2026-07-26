package devcycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	watchpkg "github.com/compozy/agh/internal/loop/watch"
)

const (
	codeRabbitBotLogin = "coderabbitai[bot]"
	ghAPIArg           = "api"
	ghGraphQLArg       = "graphql"
	gitRevParseArg     = "rev-parse"
)

type codeRabbitProvider struct {
	runner commandRunner
}

func newCodeRabbitProvider(runner commandRunner) *codeRabbitProvider {
	return &codeRabbitProvider{runner: runner}
}

func (p *codeRabbitProvider) FetchUnresolved(
	ctx context.Context,
	input codeRabbitFetchInput,
) (codeRabbitFetchOutput, error) {
	if err := p.requireGH(); err != nil {
		return codeRabbitFetchOutput{}, err
	}
	pr, err := normalizePR(input.PR)
	if err != nil {
		return codeRabbitFetchOutput{}, err
	}
	repo, err := p.currentRepository(ctx)
	if err != nil {
		return codeRabbitFetchOutput{}, err
	}
	payload, err := p.fetchPRGraph(ctx, repo, pr)
	if err != nil {
		return codeRabbitFetchOutput{}, err
	}
	issues := unresolvedCodeRabbitIssues(payload.Data.Repository.PullRequest.ReviewThreads.Nodes)
	if input.IncludeNitpicks {
		owner, name, splitErr := splitRepo(repo)
		if splitErr != nil {
			return codeRabbitFetchOutput{}, splitErr
		}
		reviews, fetchErr := p.fetchPullRequestReviews(ctx, owner, name, pr)
		if fetchErr != nil {
			return codeRabbitFetchOutput{}, fetchErr
		}
		issues = append(issues, parseReviewBodyCommentIssues(reviews, codeRabbitBotLogin)...)
	}
	sortCodeRabbitIssues(issues)
	return codeRabbitFetchOutput{
		PR:              pr,
		UnresolvedCount: len(issues),
		Issues:          issues,
	}, nil
}

func (p *codeRabbitProvider) ResolveThreads(
	ctx context.Context,
	input codeRabbitResolveInput,
) (codeRabbitResolveOutput, error) {
	if err := p.requireGH(); err != nil {
		return codeRabbitResolveOutput{}, err
	}
	pr, err := normalizePR(input.PR)
	if err != nil {
		return codeRabbitResolveOutput{}, err
	}
	threadIDs, err := resolveThreadIDs(input.Issues, input.Results)
	if err != nil {
		return codeRabbitResolveOutput{}, err
	}
	for _, threadID := range threadIDs {
		if _, err := p.runner.Run(ctx, "gh", []string{
			ghAPIArg, ghGraphQLArg, "-f",
			"query=mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{id isResolved}}}",
			"-F", "id=" + threadID,
		}, ""); err != nil {
			return codeRabbitResolveOutput{}, err
		}
	}
	return codeRabbitResolveOutput{PR: pr, ResolvedCount: len(threadIDs), ResolvedThreads: threadIDs}, nil
}

func (p *codeRabbitProvider) Poll(
	ctx context.Context,
	req watchpkg.PollRequest,
	spec codeRabbitWatchSpec,
) (watchpkg.PollResponse, error) {
	if err := p.requireGH(); err != nil {
		return watchpkg.PollResponse{}, err
	}
	pr, err := normalizePR(spec.PR)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	repo, err := p.currentRepository(ctx)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	payload, err := p.fetchPRGraph(ctx, repo, pr)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	owner, name, err := splitRepo(repo)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	localHead, err := p.currentGitHead(ctx)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	statuses, err := p.fetchCommitStatuses(ctx, owner, name, payload.Data.Repository.PullRequest.HeadRefOid)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	latestStatus, hasStatus := latestCodeRabbitCommitStatus(statuses)
	reviews, err := p.fetchPullRequestReviews(ctx, owner, name, pr)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	responsePayload, err := buildWatchPayload(
		pr,
		payload.Data.Repository.PullRequest.HeadRefOid,
		localHead,
		reviews,
		latestStatus,
		hasStatus,
	)
	if err != nil {
		return watchpkg.PollResponse{}, err
	}
	encoded, err := json.Marshal(responsePayload)
	if err != nil {
		return watchpkg.PollResponse{}, fmt.Errorf("dev-cycle: encode watch payload: %w", err)
	}
	digest := digestBytes(encoded)
	ready := responsePayload.ProviderState.State == codeRabbitWatchCurrentReviewed ||
		responsePayload.ProviderState.State == codeRabbitWatchCurrentSettled
	var settledAt *time.Time
	if ready {
		settledAt = quietPeriodDeadline(responsePayload, spec.QuietPeriod)
	}
	if req.ExpectedStateDigest == digest && !ready {
		return watchpkg.PollResponse{Ready: false, StateDigest: digest}, nil
	}
	return watchpkg.PollResponse{
		Ready:       ready,
		StateDigest: digest,
		Payload:     json.RawMessage(encoded),
		SettledAt:   settledAt,
	}, nil
}

func (p *codeRabbitProvider) requireGH() error {
	if _, err := p.runner.LookPath("gh"); err != nil {
		return fmt.Errorf("dev-cycle: gh executable is required for CodeRabbit review tools: %w", err)
	}
	return nil
}

func (p *codeRabbitProvider) currentRepository(ctx context.Context) (string, error) {
	output, err := p.runner.Run(ctx, "gh", []string{
		"repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner",
	}, "")
	if err == nil {
		repo := strings.TrimSpace(string(output))
		if repo != "" {
			return repo, nil
		}
	}
	remote, remoteErr := p.runner.Run(ctx, "git", []string{"remote", "get-url", defaultGitRemote}, "")
	if remoteErr != nil {
		return "", errors.Join(err, remoteErr)
	}
	return parseGitHubRemote(strings.TrimSpace(string(remote)))
}

func (p *codeRabbitProvider) currentGitHead(ctx context.Context) (string, error) {
	output, err := p.runner.Run(ctx, "git", []string{gitRevParseArg, "HEAD"}, "")
	if err != nil {
		return "", fmt.Errorf("dev-cycle: resolve local HEAD: %w", err)
	}
	head := strings.TrimSpace(string(output))
	if head == "" {
		return "", fmt.Errorf("dev-cycle: local HEAD is empty")
	}
	return head, nil
}

func (p *codeRabbitProvider) fetchPRGraph(ctx context.Context, repo string, pr string) (graphQLResponse, error) {
	owner, name, err := splitRepo(repo)
	if err != nil {
		return graphQLResponse{}, err
	}
	query := `
query($owner:String!,$repo:String!,$pr:Int!){
  repository(owner:$owner,name:$repo){
    pullRequest(number:$pr){
      headRefOid
      reviewThreads(first:100){nodes{id isResolved comments(first:20){nodes{body path line author{login} createdAt}}}}
    }
  }
}`
	output, err := p.runner.Run(ctx, "gh", []string{
		ghAPIArg, ghGraphQLArg,
		"-f", "query=" + query,
		"-F", "owner=" + owner,
		"-F", "repo=" + name,
		"-F", "pr=" + pr,
	}, "")
	if err != nil {
		return graphQLResponse{}, err
	}
	var response graphQLResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return graphQLResponse{}, fmt.Errorf("dev-cycle: decode GitHub GraphQL response: %w", err)
	}
	if len(response.Errors) > 0 {
		return graphQLResponse{}, fmt.Errorf("dev-cycle: GitHub GraphQL error: %s", response.Errors[0].Message)
	}
	return response, nil
}

func unresolvedCodeRabbitIssues(threads []graphQLThread) []codeRabbitIssue {
	issues := make([]codeRabbitIssue, 0)
	for _, thread := range threads {
		if thread.IsResolved {
			continue
		}
		comment, ok := firstCodeRabbitComment(thread.Comments.Nodes)
		if !ok {
			continue
		}
		issue := codeRabbitIssue{
			ID:          stableIssueID(thread.ID),
			Title:       summarizeTitle(comment.Body),
			Body:        strings.TrimSpace(comment.Body),
			BodyRef:     digestString(comment.Body),
			File:        strings.TrimSpace(comment.Path),
			Severity:    "review",
			Author:      strings.TrimSpace(comment.Author.Login),
			ProviderRef: thread.ID,
		}
		if comment.Line != nil {
			issue.Line = *comment.Line
		}
		issues = append(issues, issue)
	}
	return issues
}

func sortCodeRabbitIssues(issues []codeRabbitIssue) {
	slices.SortFunc(issues, func(a, b codeRabbitIssue) int {
		if a.File != b.File {
			return strings.Compare(a.File, b.File)
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		if a.Title != b.Title {
			return strings.Compare(a.Title, b.Title)
		}
		if a.ReviewHash != b.ReviewHash {
			return strings.Compare(a.ReviewHash, b.ReviewHash)
		}
		return strings.Compare(a.ProviderRef, b.ProviderRef)
	})
}

func firstCodeRabbitComment(comments []graphQLComment) (graphQLComment, bool) {
	for _, comment := range comments {
		if strings.EqualFold(strings.TrimSpace(comment.Author.Login), codeRabbitBotLogin) {
			return comment, true
		}
	}
	return graphQLComment{}, false
}

func resolveThreadIDs(issues []codeRabbitIssue, results []codeRabbitFixEntry) ([]string, error) {
	acceptedIssueIDs := acceptedCodeRabbitIssueIDs(results)
	acceptedRefs := acceptedCodeRabbitProviderRefs(results)
	if len(issues) > 0 && len(acceptedIssueIDs) == 0 && len(acceptedRefs) == 0 {
		return nil, fmt.Errorf(
			"%w: CodeRabbit resolve requires at least one result with resolution fixed or documented",
			watchpkg.ErrSpecInvalid,
		)
	}
	if len(issues) == 0 || (len(acceptedIssueIDs) == 0 && len(acceptedRefs) == 0) {
		return nil, nil
	}
	accepted := make(map[string]bool)
	for id := range acceptedIssueIDs {
		accepted[id] = true
	}
	for providerRef := range acceptedRefs {
		accepted[providerRef] = true
	}
	threadIDs := make([]string, 0)
	for _, issue := range issues {
		if !isResolvableCodeRabbitThreadRef(issue.ProviderRef) {
			continue
		}
		if accepted[issue.ID] || accepted[issue.ProviderRef] {
			threadIDs = append(threadIDs, issue.ProviderRef)
		}
	}
	slices.Sort(threadIDs)
	return slices.Compact(threadIDs), nil
}

func isResolvableCodeRabbitThreadRef(providerRef string) bool {
	trimmed := strings.TrimSpace(providerRef)
	return trimmed != "" && !strings.HasPrefix(trimmed, reviewBodyCommentProviderRefPrefix)
}

func acceptedCodeRabbitIssueIDs(results []codeRabbitFixEntry) map[string]struct{} {
	accepted := make(map[string]struct{})
	for _, result := range results {
		if !codeRabbitResolutionAccepted(result.Resolution) {
			continue
		}
		id := strings.TrimSpace(result.ID)
		if id == "" {
			continue
		}
		accepted[id] = struct{}{}
	}
	return accepted
}

func acceptedCodeRabbitProviderRefs(results []codeRabbitFixEntry) map[string]struct{} {
	accepted := make(map[string]struct{})
	for _, result := range results {
		if !codeRabbitResolutionAccepted(result.Resolution) {
			continue
		}
		providerRef := strings.TrimSpace(result.ProviderRef)
		if providerRef == "" {
			continue
		}
		accepted[providerRef] = struct{}{}
	}
	return accepted
}

func codeRabbitResolutionAccepted(resolution string) bool {
	resolution = strings.ToLower(strings.TrimSpace(resolution))
	return resolution == codeRabbitFixed ||
		resolution == codeRabbitDocumented
}

func normalizePR(value any) (string, error) {
	switch typed := value.(type) {
	case float64:
		if typed <= 0 || typed != float64(int64(typed)) {
			return "", fmt.Errorf("dev-cycle: pr must be a positive integer")
		}
		return strconv.FormatInt(int64(typed), 10), nil
	case int:
		if typed <= 0 {
			return "", fmt.Errorf("dev-cycle: pr must be a positive integer")
		}
		return strconv.Itoa(typed), nil
	case string:
		pr := strings.TrimSpace(strings.TrimPrefix(typed, "#"))
		if pr == "" {
			return "", fmt.Errorf("dev-cycle: pr is required")
		}
		if _, err := strconv.Atoi(pr); err != nil {
			return "", fmt.Errorf("dev-cycle: pr must be numeric: %w", err)
		}
		return pr, nil
	default:
		return "", fmt.Errorf("dev-cycle: unsupported pr value %T", value)
	}
}

func splitRepo(repo string) (string, string, error) {
	parts := strings.Split(strings.TrimSpace(repo), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("dev-cycle: GitHub repository must be owner/name, got %q", repo)
	}
	return parts[0], parts[1], nil
}

func parseGitHubRemote(remote string) (string, error) {
	if repo, ok := strings.CutPrefix(remote, "git@github.com:"); ok {
		return strings.TrimSuffix(repo, ".git"), nil
	}
	parsed, err := url.Parse(remote)
	if err != nil {
		return "", fmt.Errorf("dev-cycle: parse git remote %q: %w", remote, err)
	}
	if parsed.Host != "github.com" {
		return "", fmt.Errorf("dev-cycle: origin remote is not github.com: %q", remote)
	}
	return strings.TrimSuffix(strings.TrimPrefix(parsed.Path, "/"), ".git"), nil
}

func summarizeTitle(body string) string {
	text := strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(body, " "))
	if text == "" {
		return "CodeRabbit review comment"
	}
	if len(text) <= 96 {
		return text
	}
	return strings.TrimSpace(text[:96]) + "..."
}

func stableIssueID(providerRef string) string {
	sum := sha256.Sum256([]byte(providerRef))
	return "cr-" + hex.EncodeToString(sum[:])[:12]
}

func digestString(value string) string {
	return digestBytes([]byte(value))
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func quietPeriodDeadline(payload codeRabbitWatchPayload, raw string) *time.Time {
	quietPeriod, err := time.ParseDuration(strings.TrimSpace(raw))
	if err != nil || quietPeriod <= 0 {
		return nil
	}
	base := time.Now().UTC()
	if payload.SubmittedAt != nil {
		base = payload.SubmittedAt.UTC()
	} else if payload.ProviderState.UpdatedAt != nil {
		base = payload.ProviderState.UpdatedAt.UTC()
	}
	settled := base.Add(quietPeriod)
	return &settled
}
