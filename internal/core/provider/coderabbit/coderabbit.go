package coderabbit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/provider"
)

const (
	name            = "coderabbit"
	defaultBotLogin = "coderabbitai[bot]"
)

type CommandRunner func(ctx context.Context, args ...string) ([]byte, error)

type Option func(*Provider)

type Provider struct {
	botLogin string
	run      CommandRunner
}

var _ provider.Provider = (*Provider)(nil)

func New(opts ...Option) *Provider {
	p := &Provider{
		botLogin: defaultBotLogin,
		run:      runGH,
	}
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

func runGH(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
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
