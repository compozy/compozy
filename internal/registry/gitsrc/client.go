package gitsrc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/outboundpolicy"
	"github.com/compozy/compozy/internal/registry"
)

const (
	sourceName          = "git"
	defaultCloneTimeout = 30 * time.Second
	archiveContentType  = "application/gzip"
)

type commandRunner func(context.Context, string, ...string) error
type gitVersionProbe func(context.Context, string) (string, error)

// Option configures a git registry client.
type Option func(*Client)

// Client downloads extension trees from public git repositories.
type Client struct {
	lookPath func(string) (string, error)
	run      commandRunner
	version  gitVersionProbe
	resolver outboundpolicy.Resolver
	timeout  time.Duration

	maxUncompressedSize int64
	maxFileCount        int
	archiveTempDir      string
}

var _ registry.Source = (*Client)(nil)

// NewClient constructs a git registry source.
func NewClient(opts ...Option) *Client {
	client := &Client{
		lookPath: exec.LookPath,
		run:      runGitCommand,
		version:  probeGitVersion,
		resolver: net.DefaultResolver,
		timeout:  defaultCloneTimeout,

		maxUncompressedSize: registry.DefaultMaxDecompressedSize,
		maxFileCount:        registry.DefaultMaxFileCount,
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
	if client.version == nil {
		client.version = probeGitVersion
	}
	if client.resolver == nil {
		client.resolver = net.DefaultResolver
	}
	if client.timeout <= 0 {
		client.timeout = defaultCloneTimeout
	}
	if client.maxUncompressedSize <= 0 {
		client.maxUncompressedSize = registry.DefaultMaxDecompressedSize
	}
	if client.maxFileCount <= 0 {
		client.maxFileCount = registry.DefaultMaxFileCount
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

func withGitVersionProbe(probe gitVersionProbe) Option {
	return func(client *Client) {
		client.version = probe
	}
}

func withRepositoryResolver(resolver outboundpolicy.Resolver) Option {
	return func(client *Client) {
		client.resolver = resolver
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
	repository, err := parseRepositoryRef(slug)
	if err != nil {
		return nil, err
	}
	name := repositoryName(repository.raw)
	return &registry.Detail{
		Listing: registry.Listing{
			Slug:   repository.raw,
			Name:   name,
			Source: sourceName,
			Type:   registry.PackageTypeExtension,
		},
		Repository: repository.raw,
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

	// Git does not expose a portable per-checkout disk quota. The clone is confined
	// to os.TempDir, so operators that accept arbitrary repositories must place that
	// filesystem behind an OS quota; the timeout bounds how long the subprocess may write.
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
	args := cloneArgs(repository, addresses, strings.TrimSpace(opts.Version), checkoutDir)
	if err := c.run(cloneCtx, executable, args...); err != nil {
		if cloneCtx.Err() != nil {
			return nil, fmt.Errorf("gitsrc: clone timed out: %w", cloneCtx.Err())
		}
		return nil, fmt.Errorf("gitsrc: clone repository: %w", err)
	}

	maxSize := opts.MaxArchiveSize
	if maxSize <= 0 {
		maxSize = registry.DefaultMaxArchiveSize
	}
	archive, archiveSize, err := createRepositoryArchive(ctx, checkoutDir, archiveLimits{
		maxCompressedSize:   maxSize,
		maxUncompressedSize: c.maxUncompressedSize,
		maxFileCount:        c.maxFileCount,
		tempDir:             c.archiveTempDir,
	})
	if err != nil {
		return nil, err
	}
	return &registry.DownloadResult{
		Reader:      archive,
		Slug:        repository.raw,
		Version:     strings.TrimSpace(opts.Version),
		ContentSize: archiveSize,
		ContentType: archiveContentType,
	}, nil
}

// Close has no retained resources.
func (c *Client) Close() error {
	return nil
}

func cloneArgs(repository repositoryRef, addresses []netip.Addr, ref string, checkoutDir string) []string {
	args := []string{
		"-c", "protocol.allow=never",
		"-c", "protocol.https.allow=always",
		"-c", "http.followRedirects=false",
		"-c", "http.proxy=",
		"-c", "http.curloptResolve=",
		"-c", "http.curloptResolve=" + curlResolveValue(repository, addresses),
		"-c", "credential.helper=",
		"-c", "credential.interactive=false",
		"-c", "core.hooksPath=" + os.DevNull,
		"clone", "--depth", "1", "--single-branch",
	}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	return append(args, "--", repository.raw, checkoutDir)
}

func runGitCommand(ctx context.Context, executable string, args ...string) error {
	command := exec.CommandContext(ctx, executable, args...)
	command.Stdin = nil
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.Env = isolatedGitEnvironment()
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
