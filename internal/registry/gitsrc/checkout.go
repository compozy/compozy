package gitsrc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

var gitCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

// Checkout owns a temporary repository tree until Close releases it.
type Checkout struct {
	Path       string
	Repository string
	// Commit is the verified full revision of the materialized tree.
	Commit     string
	tempRoot   string
	executable string
	closeOnce  sync.Once
	closeErr   error
}

var _ io.Closer = (*Checkout)(nil)

// Close releases the owned tree once and retains the first cleanup result.
func (c *Checkout) Close() error {
	c.closeOnce.Do(func() { c.closeErr = removeCloneDirectory(c.tempRoot) })
	return c.closeErr
}

// WithCheckoutTempDir places owned checkouts under an operator-selected, quota-controlled directory.
func WithCheckoutTempDir(directory string) Option {
	return func(client *Client) { client.checkoutTempDir = directory }
}

// Checkout transfers an exact repository tree to the caller, who must close it after capturing the package.
func (c *Client) Checkout(ctx context.Context, slug, ref string) (_ *Checkout, err error) {
	if ctx == nil {
		return nil, errors.New("gitsrc: context is required")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	ref = strings.TrimSpace(ref)
	checkout, err := c.cloneRepository(ctx, slug, ref)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, checkout.Close())
		}
	}()
	output, err := c.output(ctx, checkout.executable, "-C", checkout.Path, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, fmt.Errorf("gitsrc: resolve checkout commit: %w", err)
	}
	checkout.Commit = strings.TrimSpace(output)
	if !gitCommitPattern.MatchString(checkout.Commit) {
		return nil, errors.New("gitsrc: resolved checkout commit is invalid")
	}
	if gitCommitPattern.MatchString(ref) && checkout.Commit != ref {
		return nil, errors.New("gitsrc: checkout does not match the requested commit")
	}
	return checkout, nil
}

func (c *Client) cloneRepository(ctx context.Context, slug, ref string) (_ *Checkout, err error) {
	if ctx == nil {
		return nil, errors.New("gitsrc: context is required")
	}
	repository, err := parseRepositoryRef(slug)
	if err != nil {
		return nil, err
	}
	executable, err := c.lookPath("git")
	if err != nil {
		return nil, newGitUnavailableError(err)
	}
	if err := c.requireSupportedGit(ctx, executable); err != nil {
		return nil, err
	}
	addresses, err := resolveRepositoryAddresses(ctx, c.resolver, repository)
	if err != nil {
		return nil, err
	}
	// Git has no portable disk quota; the parent directory must provide one for arbitrary repositories.
	root, err := os.MkdirTemp(c.checkoutTempDir, "compozy-gitsrc-*")
	if err != nil {
		return nil, fmt.Errorf("gitsrc: create clone directory: %w", err)
	}
	checkout := &Checkout{
		Path: filepath.Join(root, "checkout"), Repository: repository.raw, tempRoot: root, executable: executable,
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, checkout.Close())
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if gitCommitPattern.MatchString(ref) {
		err = c.fetchCommit(ctx, repository, addresses, checkout, ref)
	} else {
		err = c.run(ctx, executable, cloneArgs(repository, addresses, ref, checkout.Path)...)
	}
	if err != nil {
		return nil, fmt.Errorf("gitsrc: clone repository: %w", errors.Join(err, ctx.Err()))
	}
	return checkout, nil
}

func (c *Client) fetchCommit(
	ctx context.Context, repository repositoryRef, addresses []netip.Addr, checkout *Checkout, commit string,
) error {
	repositoryDir := filepath.Join(checkout.tempRoot, "repository")
	objectFormat := "sha1"
	if len(commit) == 64 {
		objectFormat = "sha256"
	}
	commands := [][]string{
		{"init", "--bare", "--object-format=" + objectFormat, "--", repositoryDir},
		{"-C", repositoryDir, "fetch", "--depth", "1", "--no-tags", "--", repository.raw, commit},
		{"-C", repositoryDir, "worktree", "add", "--detach", "--", checkout.Path, "FETCH_HEAD"},
	}
	for _, command := range commands {
		args := append(repositoryArgs(repository, addresses), command...)
		if err := c.run(ctx, checkout.executable, args...); err != nil {
			return err
		}
	}
	return nil
}

func runGitOutput(ctx context.Context, executable string, args ...string) (string, error) {
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin = nil
	command.Stderr = io.Discard
	command.Env = isolatedGitEnvironment()
	output, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("run git: %w", err)
	}
	return string(output), nil
}
