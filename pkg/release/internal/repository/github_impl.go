package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/pkg/release/internal/config"
	"github.com/google/go-github/v74/github"
	"golang.org/x/oauth2"
)

// githubRepository is the implementation of the GithubRepository interface.
type githubRepository struct {
	client *github.Client
	owner  string
	repo   string
}

// Note: GitHub token and owner/repo validation functions have been consolidated
// in the config package to avoid duplication and ensure consistency.

// NewGithubRepository creates a new GithubRepository with validation.
func NewGithubRepository(token, owner, repo string) (GithubRepository, error) {
	// Validate token format using the consolidated validator from config package
	if err := config.ValidateGitHubToken(token); err != nil {
		return nil, fmt.Errorf("invalid GitHub token: %w", err)
	}

	// Validate owner and repo names using the consolidated validator
	if err := config.ValidateGitHubOwnerRepo(owner, repo); err != nil {
		return nil, fmt.Errorf("invalid repository configuration: %w", err)
	}

	// Create OAuth2 client with the validated token
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: strings.TrimSpace(token)},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	// Create and return the repository
	ghRepo := &githubRepository{
		client: client,
		owner:  owner,
		repo:   repo,
	}

	return ghRepo, nil
}

// NewGithubExtendedRepository creates a new GithubExtendedRepository with validation.
func NewGithubExtendedRepository(token, owner, repo string) (GithubExtendedRepository, error) {
	// Validate token format using the consolidated validator from config package
	if err := config.ValidateGitHubToken(token); err != nil {
		return nil, fmt.Errorf("invalid GitHub token: %w", err)
	}

	// Validate owner and repo names using the consolidated validator
	if err := config.ValidateGitHubOwnerRepo(owner, repo); err != nil {
		return nil, fmt.Errorf("invalid repository configuration: %w", err)
	}

	// Create OAuth2 client with the validated token
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: strings.TrimSpace(token)},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	// Create and return the repository
	ghRepo := &githubRepository{
		client: client,
		owner:  owner,
		repo:   repo,
	}

	return ghRepo, nil
}

// CreatePullRequest creates a new pull request.
func (r *githubRepository) CreatePullRequest(ctx context.Context, title, body, head, base string) (int, error) {
	pr, _, err := r.client.PullRequests.Create(ctx, r.owner, r.repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return 0, err
	}
	return pr.GetNumber(), nil
}

// CreateOrUpdatePR creates a new PR or updates an existing one.
func (r *githubRepository) CreateOrUpdatePR(
	ctx context.Context,
	head, base, title, body string,
	labels []string,
) error {
	// First, try to find an existing PR
	prs, _, err := r.client.PullRequests.List(ctx, r.owner, r.repo, &github.PullRequestListOptions{
		Head:  fmt.Sprintf("%s:%s", r.owner, head),
		Base:  base,
		State: "open",
	})
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}
	if len(prs) > 0 {
		// Update existing PR
		pr := prs[0]
		_, _, err = r.client.PullRequests.Edit(ctx, r.owner, r.repo, pr.GetNumber(), &github.PullRequest{
			Title: &title,
			Body:  &body,
		})
		if err != nil {
			return fmt.Errorf("failed to update pull request: %w", err)
		}
		// Update labels
		if len(labels) > 0 {
			_, _, err = r.client.Issues.AddLabelsToIssue(ctx, r.owner, r.repo, pr.GetNumber(), labels)
			if err != nil {
				return fmt.Errorf("failed to add labels: %w", err)
			}
		}
		return nil
	}
	// Create new PR
	pr, _, err := r.client.PullRequests.Create(ctx, r.owner, r.repo, &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	})
	if err != nil {
		return fmt.Errorf("failed to create pull request: %w", err)
	}
	// Add labels
	if len(labels) > 0 {
		_, _, err = r.client.Issues.AddLabelsToIssue(ctx, r.owner, r.repo, pr.GetNumber(), labels)
		if err != nil {
			return fmt.Errorf("failed to add labels: %w", err)
		}
	}
	return nil
}

// AddComment implementation
func (r *githubRepository) AddComment(ctx context.Context, prNumber int, body string) error {
	comment := &github.IssueComment{
		Body: github.Ptr(body),
	}
	_, _, err := r.client.Issues.CreateComment(ctx, r.owner, r.repo, prNumber, comment)
	if err != nil {
		return fmt.Errorf("failed to add comment to PR #%d: %w", prNumber, err)
	}
	return nil
}
