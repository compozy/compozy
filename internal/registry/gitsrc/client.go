package gitsrc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/registry"
)

const (
	sourceName          = "git"
	defaultCloneTimeout = 30 * time.Second
	archiveContentType  = "application/gzip"
)

type commandRunner func(context.Context, string, ...string) error

// Option configures a git registry client.
type Option func(*Client)

// Client downloads extension trees from public git repositories.
type Client struct {
	lookPath func(string) (string, error)
	run      commandRunner
	timeout  time.Duration
}

var _ registry.Source = (*Client)(nil)

// NewClient constructs a git registry source.
func NewClient(opts ...Option) *Client {
	client := &Client{
		lookPath: exec.LookPath,
		run:      runGitCommand,
		timeout:  defaultCloneTimeout,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(client)
		}
	}
	if client.lookPath == nil {
		client.lookPath = exec.LookPath
	}
	if client.run == nil {
		client.run = runGitCommand
	}
	if client.timeout <= 0 {
		client.timeout = defaultCloneTimeout
	}
	return client
}

// WithLookPath overrides executable discovery.
func WithLookPath(lookPath func(string) (string, error)) Option {
	return func(client *Client) {
		client.lookPath = lookPath
	}
}

// WithRunner overrides command execution.
func WithRunner(run commandRunner) Option {
	return func(client *Client) {
		client.run = run
	}
}

// WithTimeout overrides the clone timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(client *Client) {
		client.timeout = timeout
	}
}

// Name returns the registry source identifier.
func (c *Client) Name() string {
	return sourceName
}

// Capabilities reports that arbitrary git repositories are not searchable.
func (c *Client) Capabilities() registry.SourceCaps {
	return registry.SourceCaps{Search: false}
}

// Search is not supported for arbitrary git repositories.
func (c *Client) Search(context.Context, string, registry.SearchOpts) ([]registry.Listing, error) {
	return nil, registry.ErrNotSupported
}

// Info projects repository metadata without making a network request.
func (c *Client) Info(_ context.Context, slug string) (*registry.Detail, error) {
	repository, err := normalizeRepositoryRef(slug)
	if err != nil {
		return nil, err
	}
	name := repositoryName(repository)
	return &registry.Detail{
		Listing: registry.Listing{
			Slug:   repository,
			Name:   name,
			Source: sourceName,
			Type:   registry.PackageTypeExtension,
		},
		Repository: repository,
	}, nil
}

// Download shallow-clones a repository and returns its working tree as a gzip archive.
func (c *Client) Download(
	ctx context.Context,
	slug string,
	opts registry.DownloadOpts,
) (_ *registry.DownloadResult, err error) {
	if ctx == nil {
		return nil, errors.New("gitsrc: context is required")
	}
	repository, err := normalizeRepositoryRef(slug)
	if err != nil {
		return nil, err
	}
	executable, err := c.lookPath("git")
	if err != nil {
		return nil, newGitUnavailableError(err)
	}

	tempRoot, err := os.MkdirTemp("", "compozy-gitsrc-*")
	if err != nil {
		return nil, fmt.Errorf("gitsrc: create clone directory: %w", err)
	}
	defer func() {
		err = errors.Join(err, removeCloneDirectory(tempRoot))
	}()
	checkoutDir := filepath.Join(tempRoot, "checkout")
	cloneCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	args := cloneArgs(repository, strings.TrimSpace(opts.Version), checkoutDir)
	if err := c.run(cloneCtx, executable, args...); err != nil {
		if cloneCtx.Err() != nil {
			return nil, fmt.Errorf("gitsrc: clone timed out: %w", cloneCtx.Err())
		}
		return nil, fmt.Errorf("gitsrc: clone repository: %w", err)
	}

	archive, err := fileutil.TarGzipDirectory(checkoutDir, map[string]struct{}{`.git`: {}})
	if err != nil {
		return nil, err
	}
	maxSize := opts.MaxArchiveSize
	if maxSize <= 0 {
		maxSize = registry.DefaultMaxArchiveSize
	}
	if int64(len(archive)) > maxSize {
		return nil, fmt.Errorf(
			"gitsrc: repository archive: %w: size=%d limit=%d",
			registry.ErrArchiveTooLargeCompressed,
			len(archive),
			maxSize,
		)
	}
	return &registry.DownloadResult{
		Reader:      io.NopCloser(bytes.NewReader(archive)),
		Slug:        repository,
		Version:     strings.TrimSpace(opts.Version),
		ContentSize: int64(len(archive)),
		ContentType: archiveContentType,
	}, nil
}

// Close has no retained resources.
func (c *Client) Close() error {
	return nil
}

func cloneArgs(repository string, ref string, checkoutDir string) []string {
	args := []string{"clone", "--depth", "1", "--single-branch"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	return append(args, "--", repository, checkoutDir)
}

func runGitCommand(ctx context.Context, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin = nil
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return fmt.Errorf("run git: %w", err)
	}
	return nil
}

func removeCloneDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("gitsrc: remove clone directory %q: %w", path, err)
	}
	return nil
}
