package repository

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// gitRepository is the implementation of the GitRepository interface.

type gitRepository struct {
	repo *git.Repository
}

// NewGitRepository creates a new GitRepository.
func NewGitRepository() (GitRepository, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}
	return &gitRepository{repo: repo}, nil
}

// LatestTag returns the latest git tag.
func (r *gitRepository) LatestTag(_ context.Context) (string, error) {
	tagRefs, err := r.repo.Tags()
	if err != nil {
		return "", err
	}

	var latestTag string
	var latestCommitTime time.Time

	if err := tagRefs.ForEach(func(ref *plumbing.Reference) error {
		// Try to get the commit directly first (lightweight tag)
		commit, err := r.repo.CommitObject(ref.Hash())
		if err != nil {
			// If that fails, try to get it as an annotated tag
			tag, err := r.repo.TagObject(ref.Hash())
			if err != nil {
				return nil // Skip this tag if we can't resolve it
			}
			commit, err = r.repo.CommitObject(tag.Target)
			if err != nil {
				return nil // Skip if we can't get the commit
			}
		}

		if commit.Committer.When.After(latestCommitTime) {
			latestCommitTime = commit.Committer.When
			latestTag = ref.Name().Short()
		}
		return nil
	}); err != nil {
		return "", err
	}

	return latestTag, nil
}

// CommitsSinceTag returns the number of commits since the given tag.
func (r *gitRepository) CommitsSinceTag(_ context.Context, tag string) (int, error) {
	tagRef, err := r.repo.Tag(tag)
	if err != nil {
		return 0, err
	}

	commits, err := r.repo.Log(&git.LogOptions{From: tagRef.Hash()})
	if err != nil {
		return 0, err
	}

	var count int
	if err := commits.ForEach(func(_ *object.Commit) error {
		count++
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}

// TagExists checks if a tag exists.
func (r *gitRepository) TagExists(_ context.Context, tag string) (bool, error) {
	_, err := r.repo.Tag(tag)
	if err == git.ErrTagNotFound {
		return false, nil
	}
	return err == nil, err
}

// CreateBranch creates a new branch.
func (r *gitRepository) CreateBranch(_ context.Context, name string) error {
	// Check if branch already exists
	branchRef := plumbing.NewBranchReferenceName(name)
	_, err := r.repo.Reference(branchRef, false)
	if err == nil {
		return fmt.Errorf("branch %s already exists", name)
	}

	head, err := r.repo.Head()
	if err != nil {
		return err
	}

	ref := plumbing.NewHashReference(branchRef, head.Hash())
	return r.repo.Storer.SetReference(ref)
}

// CreateTag creates a new tag.
func (r *gitRepository) CreateTag(_ context.Context, tag, msg string) error {
	head, err := r.repo.Head()
	if err != nil {
		return err
	}

	_, err = r.repo.CreateTag(tag, head.Hash(), &git.CreateTagOptions{
		Message: msg,
	})
	return err
}

// getAuth returns authentication configuration for GitHub Actions
func (r *gitRepository) getAuth() *http.BasicAuth {
	// Check for GITHUB_TOKEN environment variable (used in GitHub Actions)
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		// Also check for COMPOZY_RELEASE_GITHUB_TOKEN
		token = os.Getenv("COMPOZY_RELEASE_GITHUB_TOKEN")
	}
	if token == "" {
		return nil
	}
	// Use x-access-token as username for GitHub token authentication
	return &http.BasicAuth{
		Username: "x-access-token",
		Password: token,
	}
}

// PushTag pushes a tag to the remote.
func (r *gitRepository) PushTag(ctx context.Context, tag string) error {
	return r.repo.PushContext(ctx, &git.PushOptions{
		RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/tags/%s:refs/tags/%s", tag, tag))},
		Auth:     r.getAuth(),
	})
}

// PushBranch pushes a branch to the remote.
func (r *gitRepository) PushBranch(ctx context.Context, name string) error {
	return r.repo.PushContext(ctx, &git.PushOptions{
		RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", name, name))},
		Auth:     r.getAuth(),
	})
}
