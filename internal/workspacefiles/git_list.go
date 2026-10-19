package workspacefiles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var errNotRepository = errors.New("workspacefiles: root is not inside a git repository")

// StreamingGitRunner is an optional GitRunner capability that delivers stdout
// chunks as git writes them, without an output cap. When the injected runner
// implements it, listings stream and stop at the index cap; otherwise the
// buffered Run path is used.
type StreamingGitRunner interface {
	GitRunner
	RunStreaming(
		ctx context.Context,
		dir string,
		onStdout func(chunk []byte),
		args ...string,
	) (stderr []byte, err error)
}

// gitFiles returns tracked and untracked-not-ignored files relative to root.
// Deleted tracked files are dropped by the per-path Lstat in classifyListed.
func (idx *index) gitFiles(ctx context.Context, root string) ([]string, bool, error) {
	gitCtx, cancel := context.WithTimeout(ctx, idx.gitTimeout)
	defer cancel()
	collector := fileCollector{
		expired: ctx.Err,
		root:    root,
		limit:   idx.maxFiles,
		seen:    make(map[string]struct{}),
		files:   make([]string, 0, 1024),
	}
	partial, err := idx.listGit(
		gitCtx, root, collector.add, "ls-files", "--cached", "--others", "--exclude-standard", "-z",
	)
	if err != nil {
		return nil, false, err
	}
	return collector.files, partial || collector.truncated, nil
}

// fileCollector accepts listed paths up to limit, dropping missing, duplicate,
// and non-indexable ones, and classifying symlinks by their target. It stops
// with truncation once the build deadline passes.
type fileCollector struct {
	expired   func() error // reports the build deadline
	root      string
	limit     int
	seen      map[string]struct{}
	files     []string
	truncated bool
}

// add returns false once a path beyond the cap arrives or the build deadline passes.
func (c *fileCollector) add(name string) bool {
	if c.expired() != nil {
		c.truncated = true
		return false
	}
	if !indexablePath(name) {
		return true
	}
	if _, dup := c.seen[name]; dup {
		return true
	}
	c.seen[name] = struct{}{}
	listed, keep := classifyListed(c.root, name)
	if !keep {
		return true
	}
	if len(c.files) == c.limit {
		c.truncated = true
		return false
	}
	c.files = append(c.files, listed)
	return true
}

// listGit runs one NUL-separated git listing and passes each record to visit
// until visit returns false. It reports partial when the buffered runner cut
// the last record.
func (idx *index) listGit(
	ctx context.Context,
	root string,
	visit func(string) bool,
	args ...string,
) (bool, error) {
	if streaming, ok := idx.git.(StreamingGitRunner); ok {
		return false, streamGitRecords(ctx, streaming, root, visit, args...)
	}
	stdout, stderr, err := idx.git.Run(ctx, root, args...)
	if err != nil {
		return false, gitListError(args, stderr, err)
	}
	names, partial := splitNUL(stdout)
	for _, name := range names {
		if !visit(name) {
			return partial, nil
		}
	}
	return partial, nil
}

// streamGitRecords parses records from stdout chunks and cancels git once
// visit stops accepting them.
func streamGitRecords(
	ctx context.Context,
	runner StreamingGitRunner,
	root string,
	visit func(string) bool,
	args ...string,
) error {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		mu      sync.Mutex
		pending []byte
		stopped bool
	)
	onStdout := func(chunk []byte) {
		mu.Lock()
		defer mu.Unlock()
		if stopped {
			return
		}
		pending = append(pending, chunk...)
		for {
			end := bytes.IndexByte(pending, 0)
			if end < 0 {
				break
			}
			record := string(pending[:end])
			pending = pending[end+1:]
			if record != "" && !visit(record) {
				stopped = true
				cancel()
				return
			}
		}
	}
	stderr, err := runner.RunStreaming(streamCtx, root, onStdout, args...)
	mu.Lock()
	defer mu.Unlock()
	if stopped {
		return nil
	}
	if err != nil {
		return gitListError(args, stderr, err)
	}
	return nil
}

func gitListError(args []string, stderr []byte, err error) error {
	if strings.Contains(string(stderr), "not a git repository") ||
		strings.Contains(err.Error(), "not a git repository") {
		return errNotRepository
	}
	return fmt.Errorf("workspacefiles: git %s: %w", args[0], err)
}

// splitNUL splits NUL-terminated records. A trailing record without its NUL was
// cut by the runner's output cap; it is dropped and reported as partial.
func splitNUL(output []byte) ([]string, bool) {
	if len(output) == 0 {
		return nil, false
	}
	partial := output[len(output)-1] != 0
	records := bytes.Split(output, []byte{0})
	records = records[:len(records)-1]
	names := make([]string, 0, len(records))
	for _, record := range records {
		if len(record) > 0 {
			names = append(names, string(record))
		}
	}
	return names, partial
}
