package desktoprelease

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

const githubCommitPageSize = 100

type githubAuditCommit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

func (b *GitHubBackend) FindOperation(
	ctx context.Context,
	branch, operationID string,
) (*ChannelCommit, error) {
	needle := "operation_id=" + operationID
	var match *ChannelCommit
	err := b.walkAuditCommits(ctx, branch, func(candidate githubAuditCommit) (bool, error) {
		if !hasAuditLine(candidate.Commit.Message, needle) {
			return false, nil
		}
		resolved, err := b.ReadCommit(ctx, candidate.SHA)
		if err != nil {
			return false, err
		}
		match = &resolved
		return true, nil
	})
	if isGitHubStatus(err, http.StatusNotFound) {
		return nil, nil
	}
	return match, err
}

func (b *GitHubBackend) KnownGood(
	ctx context.Context,
	branch, version string,
) (ChannelCommit, error) {
	var match ChannelCommit
	found := false
	err := b.walkAuditCommits(ctx, branch, func(candidate githubAuditCommit) (bool, error) {
		if !hasAuditLine(candidate.Commit.Message, "known_good=true") {
			return false, nil
		}
		commit, err := b.ReadCommit(ctx, candidate.SHA)
		if err != nil {
			return false, fmt.Errorf("desktop release: inspect known-good commit %s: %w", candidate.SHA, err)
		}
		if commit.Generation.Version != version {
			return false, nil
		}
		match = commit
		found = true
		return true, nil
	})
	if err != nil {
		return ChannelCommit{}, err
	}
	if !found {
		return ChannelCommit{}, fmt.Errorf("desktop release: no known-good generation for %s", version)
	}
	return match, nil
}

func (b *GitHubBackend) walkAuditCommits(
	ctx context.Context,
	branch string,
	visit func(githubAuditCommit) (bool, error),
) error {
	for page := 1; ; page++ {
		var commits []githubAuditCommit
		endpoint := fmt.Sprintf(
			"/repos/%s/commits?sha=%s&per_page=%d&page=%d",
			b.repository,
			url.QueryEscape(branch),
			githubCommitPageSize,
			page,
		)
		if err := b.get(ctx, endpoint, &commits); err != nil {
			return err
		}
		for _, commit := range commits {
			stop, err := visit(commit)
			if err != nil {
				return err
			}
			if stop {
				return nil
			}
		}
		if len(commits) < githubCommitPageSize {
			return nil
		}
	}
}

func hasAuditLine(message, want string) bool {
	for line := range strings.SplitSeq(message, "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}
