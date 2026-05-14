package coderabbit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/provider"
)

const (
	name            = "coderabbit"
	defaultBotLogin = "coderabbitai[bot]"
)

type CommandRunner func(ctx context.Context, args ...string) ([]byte, error)

type Option func(*Provider)

type Provider struct {
	botLogin   string
	run        CommandRunner
	workingDir string
}

var _ provider.Provider = (*Provider)(nil)
var _ provider.WatchStatusProvider = (*Provider)(nil)

func New(opts ...Option) *Provider {
	p := &Provider{
		botLogin: defaultBotLogin,
	}
	p.run = p.runGH
	for _, opt := range opts {
		if opt != nil {
			opt(p)
		}
	}
	return p
}

func WithCommandRunner(run CommandRunner) Option {
	return func(p *Provider) {
		if run != nil {
			p.run = run
		}
	}
}

func WithWorkingDir(dir string) Option {
	return func(p *Provider) {
		p.workingDir = strings.TrimSpace(dir)
	}
}

func WithBotLogin(login string) Option {
	return func(p *Provider) {
		trimmed := strings.TrimSpace(login)
		if trimmed != "" {
			p.botLogin = trimmed
		}
	}
}

func (p *Provider) Name() string {
	return name
}

func (p *Provider) DisplayName() string {
	return "CodeRabbit"
}

func (p *Provider) FetchReviews(ctx context.Context, req provider.FetchRequest) ([]provider.ReviewItem, error) {
	if strings.TrimSpace(req.PR) == "" {
		return nil, errors.New("pull request number is required")
	}

	owner, repo, err := p.getRepo(ctx)
	if err != nil {
		return nil, err
	}

	comments, err := p.fetchReviewComments(ctx, owner, repo, req.PR)
	if err != nil {
		return nil, err
	}
	threads, err := p.fetchReviewThreads(ctx, owner, repo, req.PR)
	if err != nil {
		return nil, err
	}

	threadByCommentID := make(map[int]reviewThread)
	for _, thread := range threads {
		for _, comment := range thread.Comments.Nodes {
			if comment.DatabaseID == 0 {
				continue
			}
			threadByCommentID[comment.DatabaseID] = thread
		}
	}

	items := make([]provider.ReviewItem, 0, len(comments))
	for _, comment := range comments {
		if comment.User.Login != p.botLogin {
			continue
		}

		thread := threadByCommentID[comment.ID]
		if thread.IsResolved {
			continue
		}

		items = append(items, provider.ReviewItem{
			Title:       summarizeTitle(comment.Body),
			File:        comment.Path,
			Line:        comment.effectiveLine(),
			Author:      comment.User.Login,
			Body:        strings.TrimSpace(comment.Body),
			ProviderRef: buildProviderRef(thread.ID, comment.NodeID),
		})
	}

	if req.IncludeNitpicks {
		reviews, err := p.fetchPullRequestReviews(ctx, owner, repo, req.PR)
		if err != nil {
			return nil, err
		}
		items = append(items, parseReviewBodyCommentItems(reviews, p.botLogin)...)
	}

	sortReviewItems(items)
	return items, nil
}

func (p *Provider) WatchStatus(
	ctx context.Context,
	req provider.WatchStatusRequest,
) (provider.WatchStatus, error) {
	if strings.TrimSpace(req.PR) == "" {
		return provider.WatchStatus{}, errors.New("pull request number is required")
	}

	owner, repo, err := p.getRepo(ctx)
	if err != nil {
		return provider.WatchStatus{}, err
	}
	pr, err := p.fetchPullRequest(ctx, owner, repo, req.PR)
	if err != nil {
		return provider.WatchStatus{}, err
	}
	if strings.TrimSpace(pr.Head.SHA) == "" {
		return provider.WatchStatus{}, errors.New("pull request metadata response is incomplete")
	}
	statuses, err := p.fetchCommitStatuses(ctx, owner, repo, pr.Head.SHA)
	if err != nil {
		return provider.WatchStatus{}, err
	}
	latestStatus, hasStatus := latestCodeRabbitCommitStatus(statuses)

	reviews, err := p.fetchPullRequestReviews(ctx, owner, repo, req.PR)
	if err != nil {
		return provider.WatchStatus{}, err
	}
	latest, ok := latestProviderReview(reviews, p.botLogin)
	if !ok {
		return classifyWatchStatusWithoutReview(pr.Head.SHA, latestStatus, hasStatus)
	}

	return classifyWatchStatus(pr.Head.SHA, latest, latestStatus, hasStatus)
}

func (p *Provider) fetchPullRequest(
	ctx context.Context,
	owner string,
	repo string,
	pr string,
) (pullRequest, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%s", owner, repo, pr)
	output, err := p.run(ctx, "api", endpoint)
	if err != nil {
		return pullRequest{}, fmt.Errorf("fetch pull request metadata: %w", err)
	}

	var payload pullRequest
	if err := json.Unmarshal(output, &payload); err != nil {
		return pullRequest{}, fmt.Errorf("decode pull request metadata: %w", err)
	}
	return payload, nil
}

type commitStatus struct {
	State       string `json:"state"`
	Description string `json:"description"`
	Context     string `json:"context"`
	UpdatedAt   string `json:"updated_at"`
	CreatedAt   string `json:"created_at"`
}

func (p *Provider) fetchCommitStatuses(
	ctx context.Context,
	owner string,
	repo string,
	sha string,
) ([]commitStatus, error) {
	statuses := make([]commitStatus, 0, 8)
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/commits/%s/statuses?per_page=100&page=%d", owner, repo, sha, page)
		output, err := p.run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetch commit statuses page %d: %w", page, err)
		}

		var pageStatuses []commitStatus
		if err := json.Unmarshal(output, &pageStatuses); err != nil {
			return nil, fmt.Errorf("decode commit statuses page %d: %w", page, err)
		}

		statuses = append(statuses, pageStatuses...)
		if len(pageStatuses) < 100 {
			break
		}
	}
	return statuses, nil
}

func latestCodeRabbitCommitStatus(statuses []commitStatus) (commitStatus, bool) {
	var latest commitStatus
	found := false
	for _, status := range statuses {
		if !strings.EqualFold(strings.TrimSpace(status.Context), "CodeRabbit") {
			continue
		}
		if !found || commitStatusIsNewer(status, latest) {
			latest = status
			found = true
		}
	}
	return latest, found
}

func commitStatusIsNewer(candidate commitStatus, current commitStatus) bool {
	candidateTime := commitStatusTimestamp(candidate)
	currentTime := commitStatusTimestamp(current)
	if candidateTime.IsZero() {
		return false
	}
	if currentTime.IsZero() {
		return true
	}
	return candidateTime.After(currentTime)
}

func commitStatusTimestamp(status commitStatus) time.Time {
	if updatedAt := parseReviewSubmittedAt(status.UpdatedAt); !updatedAt.IsZero() {
		return updatedAt
	}
	return parseReviewSubmittedAt(status.CreatedAt)
}

func latestProviderReview(reviews []pullRequestReview, botLogin string) (pullRequestReview, bool) {
	var latest pullRequestReview
	found := false
	for _, review := range reviews {
		if review.User.Login != botLogin {
			continue
		}
		if !found || reviewIsNewer(review, latest) {
			latest = review
			found = true
		}
	}
	return latest, found
}

func reviewIsNewer(candidate pullRequestReview, current pullRequestReview) bool {
	candidateTime := parseReviewSubmittedAt(candidate.SubmittedAt)
	currentTime := parseReviewSubmittedAt(current.SubmittedAt)
	if candidateTime.IsZero() || currentTime.IsZero() {
		return compareReviewIDs(strconv.Itoa(candidate.ID), strconv.Itoa(current.ID)) > 0
	}
	if candidateTime.After(currentTime) {
		return true
	}
	if currentTime.After(candidateTime) {
		return false
	}
	return compareReviewIDs(strconv.Itoa(candidate.ID), strconv.Itoa(current.ID)) > 0
}

func classifyWatchStatus(
	headSHA string,
	review pullRequestReview,
	latestStatus commitStatus,
	hasStatus bool,
) (provider.WatchStatus, error) {
	status := provider.WatchStatus{
		PRHeadSHA:       strings.TrimSpace(headSHA),
		ReviewCommitSHA: strings.TrimSpace(review.CommitID),
		ReviewID:        strconv.Itoa(review.ID),
		ReviewState:     strings.TrimSpace(review.State),
		State:           provider.WatchStatusPending,
	}
	if hasStatus {
		applyCommitStatus(&status, latestStatus)
		ready, err := applyCodeRabbitStatusGate(&status, latestStatus)
		if err != nil || !ready {
			return status, err
		}
		return classifySuccessfulWatchStatus(status, review)
	}
	return classifyWatchStatusFromReviewOnly(status, review)
}

func classifyWatchStatusWithoutReview(
	headSHA string,
	latestStatus commitStatus,
	hasStatus bool,
) (provider.WatchStatus, error) {
	status := provider.WatchStatus{
		PRHeadSHA: strings.TrimSpace(headSHA),
		State:     provider.WatchStatusPending,
	}
	if !hasStatus {
		return status, nil
	}
	applyCommitStatus(&status, latestStatus)
	ready, err := applyCodeRabbitStatusGate(&status, latestStatus)
	if err != nil || !ready {
		return status, err
	}
	status.State = provider.WatchStatusCurrentSettled
	return status, nil
}

func classifySuccessfulWatchStatus(
	status provider.WatchStatus,
	review pullRequestReview,
) (provider.WatchStatus, error) {
	if status.ReviewCommitSHA == "" || reviewStateIsPending(status.ReviewState) {
		status.State = provider.WatchStatusCurrentSettled
		return status, nil
	}

	submittedAt := parseReviewSubmittedAt(review.SubmittedAt)
	if submittedAt.IsZero() {
		return provider.WatchStatus{}, fmt.Errorf(
			"decode pull request review %d submitted_at: %q",
			review.ID,
			review.SubmittedAt,
		)
	}
	status.SubmittedAt = submittedAt
	if status.ReviewCommitSHA != status.PRHeadSHA {
		status.State = provider.WatchStatusCurrentSettled
		return status, nil
	}
	status.State = provider.WatchStatusCurrentReviewed
	return status, nil
}

func classifyWatchStatusFromReviewOnly(
	status provider.WatchStatus,
	review pullRequestReview,
) (provider.WatchStatus, error) {
	if status.ReviewCommitSHA == "" || reviewStateIsPending(status.ReviewState) {
		return status, nil
	}

	submittedAt := parseReviewSubmittedAt(review.SubmittedAt)
	if submittedAt.IsZero() {
		return provider.WatchStatus{}, fmt.Errorf(
			"decode pull request review %d submitted_at: %q",
			review.ID,
			review.SubmittedAt,
		)
	}
	status.SubmittedAt = submittedAt
	if status.ReviewCommitSHA != status.PRHeadSHA {
		status.State = provider.WatchStatusStale
	}
	return status, nil
}

func reviewStateIsPending(state string) bool {
	return strings.EqualFold(strings.TrimSpace(state), "PENDING")
}

func applyCommitStatus(status *provider.WatchStatus, commitStatus commitStatus) {
	if status == nil {
		return
	}
	status.ProviderStatusState = strings.TrimSpace(commitStatus.State)
	status.ProviderStatusDescription = strings.TrimSpace(commitStatus.Description)
	status.ProviderStatusUpdatedAt = commitStatusTimestamp(commitStatus)
}

func applyCodeRabbitStatusGate(status *provider.WatchStatus, latestStatus commitStatus) (bool, error) {
	if commitStatusIsPending(latestStatus) {
		status.State = provider.WatchStatusPending
		return false, nil
	}
	if commitStatusFailed(latestStatus) {
		return false, fmt.Errorf(
			"coderabbit status %q for head %s: %s",
			strings.TrimSpace(latestStatus.State),
			status.PRHeadSHA,
			strings.TrimSpace(latestStatus.Description),
		)
	}
	if !commitStatusSucceeded(latestStatus) {
		return false, fmt.Errorf(
			"coderabbit status %q for head %s is unsupported",
			strings.TrimSpace(latestStatus.State),
			status.PRHeadSHA,
		)
	}
	return true, nil
}

func commitStatusIsPending(status commitStatus) bool {
	return strings.EqualFold(strings.TrimSpace(status.State), "pending")
}

func commitStatusSucceeded(status commitStatus) bool {
	return strings.EqualFold(strings.TrimSpace(status.State), "success")
}

func commitStatusFailed(status commitStatus) bool {
	state := strings.ToLower(strings.TrimSpace(status.State))
	return state == "failure" || state == "error"
}

func sortReviewItems(items []provider.ReviewItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].File != items[j].File {
			return items[i].File < items[j].File
		}
		if items[i].Line != items[j].Line {
			return items[i].Line < items[j].Line
		}
		if items[i].Title != items[j].Title {
			return items[i].Title < items[j].Title
		}
		if items[i].ReviewHash != items[j].ReviewHash {
			return items[i].ReviewHash < items[j].ReviewHash
		}
		return items[i].ProviderRef < items[j].ProviderRef
	})
}

func (p *Provider) ResolveIssues(ctx context.Context, _ string, issues []provider.ResolvedIssue) error {
	seen := make(map[string]struct{}, len(issues))
	var errs []error

	for _, issue := range issues {
		threadID := providerRefValue(issue.ProviderRef, "thread")
		if threadID == "" {
			continue
		}
		if _, ok := seen[threadID]; ok {
			continue
		}
		seen[threadID] = struct{}{}

		if err := p.resolveThread(ctx, threadID); err != nil {
			errs = append(errs, fmt.Errorf("resolve thread %s: %w", threadID, err))
		}
	}

	return errors.Join(errs...)
}

func (p *Provider) getRepo(ctx context.Context) (string, string, error) {
	output, err := p.run(ctx, "repo", "view", "--json", "owner,name")
	if err != nil {
		return "", "", fmt.Errorf("resolve repository metadata: %w", err)
	}

	var payload struct {
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		return "", "", fmt.Errorf("decode repository metadata: %w", err)
	}
	if payload.Owner.Login == "" || payload.Name == "" {
		return "", "", errors.New("repository metadata response is incomplete")
	}
	return payload.Owner.Login, payload.Name, nil
}

func (p *Provider) fetchReviewComments(
	ctx context.Context,
	owner string,
	repo string,
	pr string,
) ([]pullRequestComment, error) {
	comments := make([]pullRequestComment, 0, 32)
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("repos/%s/%s/pulls/%s/comments?per_page=100&page=%d", owner, repo, pr, page)
		output, err := p.run(ctx, "api", endpoint)
		if err != nil {
			return nil, fmt.Errorf("fetch pull request comments page %d: %w", page, err)
		}

		var pageComments []pullRequestComment
		if err := json.Unmarshal(output, &pageComments); err != nil {
			return nil, fmt.Errorf("decode pull request comments page %d: %w", page, err)
		}

		comments = append(comments, pageComments...)
		if len(pageComments) < 100 {
			break
		}
	}
	return comments, nil
}

func (p *Provider) fetchReviewThreads(
	ctx context.Context,
	owner string,
	repo string,
	pr string,
) ([]reviewThread, error) {
	const query = `
query($owner: String!, $repo: String!, $pr: Int!, $after: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $pr) {
      reviewThreads(first: 100, after: $after) {
        pageInfo {
          hasNextPage
          endCursor
        }
        nodes {
          id
          isResolved
          comments(first: 100) {
            nodes {
              id
              databaseId
            }
          }
        }
      }
    }
  }
}`

	threads := make([]reviewThread, 0, 32)
	after := ""
	for {
		args := []string{
			"api",
			"graphql",
			"-F", "query=" + strings.TrimSpace(query),
			"-F", "owner=" + owner,
			"-F", "repo=" + repo,
			"-F", "pr=" + pr,
		}
		if after != "" {
			args = append(args, "-F", "after="+after)
		}

		output, err := p.run(ctx, args...)
		if err != nil {
			return nil, fmt.Errorf("fetch review threads: %w", err)
		}

		var response reviewThreadsResponse
		if err := json.Unmarshal(output, &response); err != nil {
			return nil, fmt.Errorf("decode review threads: %w", err)
		}

		reviewThreads := response.Data.Repository.PullRequest.ReviewThreads
		threads = append(threads, reviewThreads.Nodes...)
		if !reviewThreads.PageInfo.HasNextPage {
			break
		}
		after = reviewThreads.PageInfo.EndCursor
	}

	return threads, nil
}

func (p *Provider) resolveThread(ctx context.Context, threadID string) error {
	const mutation = `mutation($threadId: ID!) {
  resolveReviewThread(input: { threadId: $threadId }) {
    thread {
      isResolved
    }
  }
}`

	if _, err := p.run(
		ctx,
		"api",
		"graphql",
		"-f", "query="+strings.TrimSpace(mutation),
		"-F", "threadId="+threadID,
	); err != nil {
		return err
	}
	return nil
}

func summarizeTitle(body string) string {
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(strings.TrimLeft(rawLine, "-*#> "))
		if line == "" {
			continue
		}
		line = strings.ReplaceAll(line, "`", "")
		line = strings.Join(strings.Fields(line), " ")
		runes := []rune(line)
		if len(runes) > 72 {
			return string(runes[:69]) + "..."
		}
		return line
	}
	return "Review comment"
}

func buildProviderRef(threadID string, commentID string) string {
	parts := make([]string, 0, 2)
	if strings.TrimSpace(threadID) != "" {
		parts = append(parts, "thread:"+threadID)
	}
	if strings.TrimSpace(commentID) != "" {
		parts = append(parts, "comment:"+commentID)
	}
	return strings.Join(parts, ",")
}

func providerRefValue(ref string, key string) string {
	for _, part := range strings.Split(ref, ",") {
		rawKey, rawValue, ok := strings.Cut(strings.TrimSpace(part), ":")
		if !ok {
			continue
		}
		if rawKey == key {
			return strings.TrimSpace(rawValue)
		}
	}
	return ""
}

func (p *Provider) runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	if p != nil {
		if workingDir := strings.TrimSpace(p.workingDir); workingDir != "" {
			cmd.Dir = workingDir
		}
	}
	output, err := cmd.CombinedOutput()
	if err == nil {
		return output, nil
	}

	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), trimmed)
}

type pullRequestComment struct {
	ID           int    `json:"id"`
	NodeID       string `json:"node_id"`
	Body         string `json:"body"`
	Path         string `json:"path"`
	Line         int    `json:"line"`
	OriginalLine int    `json:"original_line"`
	User         struct {
		Login string `json:"login"`
	} `json:"user"`
}

type pullRequest struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func (c pullRequestComment) effectiveLine() int {
	if c.Line > 0 {
		return c.Line
	}
	return c.OriginalLine
}

type reviewThreadsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads reviewThreadConnection `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

type reviewThreadConnection struct {
	PageInfo struct {
		HasNextPage bool   `json:"hasNextPage"`
		EndCursor   string `json:"endCursor"`
	} `json:"pageInfo"`
	Nodes []reviewThread `json:"nodes"`
}

type reviewThread struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"isResolved"`
	Comments   struct {
		Nodes []reviewThreadComment `json:"nodes"`
	} `json:"comments"`
}

type reviewThreadComment struct {
	ID         string `json:"id"`
	DatabaseID int    `json:"databaseId"`
}
