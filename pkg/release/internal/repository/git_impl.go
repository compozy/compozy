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
	"github.com/go-git/go-git/v5/plumbing/storer"
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

// NewGitExtendedRepository creates a new GitExtendedRepository with all extended operations.
func NewGitExtendedRepository() (GitExtendedRepository, error) {
	repo, err := git.PlainOpen(".")
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}
	return &gitRepository{repo: repo}, nil
}

// LatestTag returns the latest git tag.
func (r *gitRepository) LatestTag(_ context.Context) (string, error) {
	// First, try to fetch tags from remote to ensure we have the latest
	remote, err := r.repo.Remote("origin")
	if err == nil {
		// Fetch tags from remote (ignore error if already up to date)
		_ = remote.Fetch(&git.FetchOptions{
			RefSpecs: []config.RefSpec{
				config.RefSpec("+refs/tags/*:refs/tags/*"),
			},
			Auth: r.getAuth(),
		})
	}
	tagRefs, err := r.repo.Tags()
	if err != nil {
		return "", fmt.Errorf("failed to get tags: %w", err)
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
		return "", fmt.Errorf("failed to iterate tags: %w", err)
	}
	return latestTag, nil
}

// CommitsSinceTag returns the number of commits since the given tag.
func (r *gitRepository) CommitsSinceTag(_ context.Context, tag string) (int, error) {
	// First, try to fetch the tag from remote if it doesn't exist locally
	tagRef, err := r.repo.Tag(tag)
	if err != nil {
		// Tag doesn't exist locally, try to fetch it from remote
		remote, err := r.repo.Remote("origin")
		if err != nil {
			return 0, fmt.Errorf("failed to get remote: %w", err)
		}
		// Fetch tags from remote
		if err := remote.Fetch(&git.FetchOptions{
			RefSpecs: []config.RefSpec{
				config.RefSpec("+refs/tags/*:refs/tags/*"),
			},
			Auth: r.getAuth(),
		}); err != nil && err != git.NoErrAlreadyUpToDate {
			return 0, fmt.Errorf("failed to fetch tags from remote: %w", err)
		}
		// Try to get the tag again
		tagRef, err = r.repo.Tag(tag)
		if err != nil {
			return 0, fmt.Errorf("failed to get tag %s after fetching: %w", tag, err)
		}
	}
	// Resolve the tag to a commit hash (handles lightweight *and* annotated tags)
	var tagCommitHash plumbing.Hash
	if commit, err := r.repo.CommitObject(tagRef.Hash()); err == nil {
		// Lightweight tag – hash already points to commit
		tagCommitHash = commit.Hash
	} else if tagObj, err := r.repo.TagObject(tagRef.Hash()); err == nil {
		// Annotated tag – resolve to the tagged commit
		if commit, err := r.repo.CommitObject(tagObj.Target); err == nil {
			tagCommitHash = commit.Hash
		}
	}
	if tagCommitHash == (plumbing.Hash{}) {
		return 0, fmt.Errorf("failed to resolve commit for tag %s", tag)
	}
	// Get HEAD commit
	head, err := r.repo.Head()
	if err != nil {
		return 0, fmt.Errorf("failed to get HEAD: %w", err)
	}
	headCommit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return 0, fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	// Count commits from tag to HEAD (excluding the tag commit itself)
	var count int
	commits, err := r.repo.Log(&git.LogOptions{From: headCommit.Hash})
	if err != nil {
		return 0, fmt.Errorf("failed to get commits: %w", err)
	}
	err = commits.ForEach(func(c *object.Commit) error {
		// Stop when we reach the tag commit
		if c.Hash == tagCommitHash {
			return storer.ErrStop
		}
		count++
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return 0, fmt.Errorf("failed to iterate commits: %w", err)
	}
	return count, nil
}

// TagExists checks if a tag exists.
func (r *gitRepository) TagExists(_ context.Context, tag string) (bool, error) {
	_, err := r.repo.Tag(tag)
	if err == git.ErrTagNotFound {
		return false, nil
	}
	if err != nil && err != git.ErrTagNotFound {
		return false, fmt.Errorf("failed to check tag %s: %w", tag, err)
	}
	return err == nil, nil
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
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	_, err = r.repo.CreateTag(tag, head.Hash(), &git.CreateTagOptions{
		Message: msg,
		Tagger: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create tag %s: %w", tag, err)
	}
	return nil
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

// PushBranchForce pushes a branch to the remote with force.
func (r *gitRepository) PushBranchForce(ctx context.Context, name string) error {
	return r.repo.PushContext(ctx, &git.PushOptions{
		RefSpecs: []config.RefSpec{config.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", name, name))},
		Auth:     r.getAuth(),
		Force:    true,
	})
}

// CheckoutBranch switches to the specified branch.
func (r *gitRepository) CheckoutBranch(_ context.Context, name string) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	return w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Force:  false,
	})
}

// ConfigureUser sets the git user configuration.
func (r *gitRepository) ConfigureUser(_ context.Context, name, email string) error {
	cfg, err := r.repo.Config()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}
	cfg.User.Name = name
	cfg.User.Email = email
	return r.repo.Storer.SetConfig(cfg)
}

// AddFiles stages files matching the pattern.
func (r *gitRepository) AddFiles(_ context.Context, pattern string) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	// AddGlob can return an error if no files match the pattern
	// or if there are no changes to stage. We should not fail in these cases.
	err = w.AddGlob(pattern)
	if err != nil && err.Error() != "glob pattern did not match any files" {
		return fmt.Errorf("failed to add files with pattern %s: %w", pattern, err)
	}
	return nil
}

// Commit creates a commit with the given message.
func (r *gitRepository) Commit(_ context.Context, message string) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	_, err = w.Commit(message, &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("failed to create commit: %w", err)
	}
	return nil
}

// GetCurrentBranch returns the name of the current branch.
func (r *gitRepository) GetCurrentBranch(_ context.Context) (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}
	return head.Name().Short(), nil
}

// DeleteBranch deletes a local branch.
func (r *gitRepository) DeleteBranch(_ context.Context, name string) error {
	err := r.repo.Storer.RemoveReference(plumbing.NewBranchReferenceName(name))
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", name, err)
	}
	return nil
}

// DeleteRemoteBranch deletes a remote branch.
func (r *gitRepository) DeleteRemoteBranch(_ context.Context, name string) error {
	// Get auth from environment
	token := os.Getenv("GITHUB_TOKEN")
	auth := &http.BasicAuth{
		Username: "github-actions[bot]",
		Password: token,
	}
	refSpec := config.RefSpec(":refs/heads/" + name)
	err := r.repo.Push(&git.PushOptions{
		RemoteName: "origin",
		RefSpecs:   []config.RefSpec{refSpec},
		Auth:       auth,
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		return fmt.Errorf("failed to delete remote branch %s: %w", name, err)
	}
	return nil
}

// RestoreFile restores a file to its state in HEAD.
func (r *gitRepository) RestoreFile(_ context.Context, path string) error {
	// Get HEAD commit
	head, err := r.repo.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}
	commit, err := r.repo.CommitObject(head.Hash())
	if err != nil {
		return fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	// Get the file from HEAD commit
	file, err := commit.File(path)
	if err != nil {
		return fmt.Errorf("failed to get file %s from HEAD: %w", path, err)
	}
	// Get file contents
	contents, err := file.Contents()
	if err != nil {
		return fmt.Errorf("failed to get file contents: %w", err)
	}
	// Write the contents back to the working directory
	err = os.WriteFile(path, []byte(contents), 0600)
	if err != nil {
		return fmt.Errorf("failed to restore file %s: %w", path, err)
	}
	return nil
}

// ResetHard performs a hard reset to the specified reference.
func (r *gitRepository) ResetHard(_ context.Context, ref string) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}
	// Resolve the reference
	hash, err := r.repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return fmt.Errorf("failed to resolve revision %s: %w", ref, err)
	}
	// Perform hard reset
	err = w.Reset(&git.ResetOptions{
		Commit: *hash,
		Mode:   git.HardReset,
	})
	if err != nil {
		return fmt.Errorf("failed to reset to %s: %w", ref, err)
	}
	return nil
}

// GetHeadCommit returns the SHA of the current HEAD commit.
func (r *gitRepository) GetHeadCommit(_ context.Context) (string, error) {
	head, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}
	return head.Hash().String(), nil
}

// ListLocalBranches returns a list of all local branch names.
func (r *gitRepository) ListLocalBranches(_ context.Context) ([]string, error) {
	iter, err := r.repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}
	var branches []string
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		branches = append(branches, ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate branches: %w", err)
	}
	return branches, nil
}

// ListRemoteBranches returns a list of all remote branch names.
func (r *gitRepository) ListRemoteBranches(_ context.Context) ([]string, error) {
	remote, err := r.repo.Remote("origin")
	if err != nil {
		return nil, fmt.Errorf("failed to get remote: %w", err)
	}
	refs, err := remote.List(&git.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list remote refs: %w", err)
	}
	var branches []string
	for _, ref := range refs {
		if ref.Name().IsBranch() {
			// Returns in format "origin/branch-name"
			branches = append(branches, "origin/"+ref.Name().Short())
		}
	}
	return branches, nil
}

// GetFileStatus returns the git status of a specific file.
// Returns "clean" if the file has no changes, "modified" if it has uncommitted changes.
func (r *gitRepository) GetFileStatus(_ context.Context, path string) (string, error) {
	w, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	status, err := w.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}
	fileStatus := status.File(path)
	if fileStatus.Worktree == git.Unmodified && fileStatus.Staging == git.Unmodified {
		return "clean", nil
	}
	return "modified", nil
}
