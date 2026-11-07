package changes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/worktree"
)

type GitEngine struct {
	runner worktree.StreamingGitRunner
	reader projectfiles.Reader
}

var _ Engine = (*GitEngine)(nil)

type Option func(*GitEngine)

func WithGitRunner(runner worktree.StreamingGitRunner) Option {
	return func(e *GitEngine) { e.runner = runner }
}
func WithReader(reader projectfiles.Reader) Option { return func(e *GitEngine) { e.reader = reader } }
func NewEngine(opts ...Option) *GitEngine {
	e := &GitEngine{}
	for _, opt := range opts {
		opt(e)
	}
	if e.runner == nil {
		runner, err := worktree.NewRealGitRunner(10 * time.Second)
		if err == nil {
			e.runner = runner
		}
	}
	if e.reader == nil {
		e.reader = projectfiles.NewReader(projectfiles.WithGitRunner(e.runner))
	}
	return e
}

func (e *GitEngine) stream(
	ctx context.Context,
	root projectfiles.Root,
	fn worktree.GitOutputHandler,
	command string,
	args ...string,
) error {
	if e.runner == nil {
		return ErrUnavailable
	}
	argv, err := projectfiles.GitReadArgs(ctx, e.runner, root.AbsPath, command, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	argv = append([]string{"--literal-pathspecs"}, argv...)
	_, _, err = e.runner.RunStreaming(ctx, root.AbsPath, fn, argv...)
	if err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

func (e *GitEngine) git(
	ctx context.Context,
	root projectfiles.Root,
	limit int,
	command string,
	args ...string,
) ([]byte, error) {
	var out bytes.Buffer
	overflow := false
	err := e.stream(ctx, root, func(o worktree.GitOutput) {
		if o.Stream != worktree.GitOutputStdout {
			return
		}
		if len(o.Chunk) > limit-out.Len() {
			overflow = true
		}
		out.Write(o.Chunk[:min(len(o.Chunk), limit-out.Len())])
	}, command, args...)
	if overflow {
		return nil, ErrUnavailable
	}
	return out.Bytes(), err
}
func validRef(ref string) bool {
	return ref != "" && !strings.HasPrefix(ref, "-") && !strings.Contains(ref, "..") && !strings.Contains(ref, "@{") &&
		!strings.ContainsFunc(ref, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
}

func (e *GitEngine) resolveBase(
	ctx context.Context,
	root projectfiles.Root,
	request string,
) (string, BaseSource, error) {
	candidates := []string{request, root.BaseRef}
	remote, err := e.git(ctx, root, 4096, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	if err == nil {
		candidates = append(candidates, strings.TrimPrefix(strings.TrimSpace(string(remote)), "refs/remotes/"))
	}
	candidates = append(candidates, "main", "master")
	for i, ref := range candidates {
		if ref == "" {
			continue
		}
		source := BaseSource("default_branch")
		switch i {
		case 0:
			source = "request"
		case 1:
			source = "worktree"
		}
		if validRef(ref) {
			if _, err := e.git(ctx, root, 256, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
				return ref, source, nil
			}
		}
		if i < 2 {
			return "", "", ErrBaseNotFound
		}
	}
	return "", "", ErrBaseNotFound
}
func (e *GitEngine) resolve(ctx context.Context, root projectfiles.Root, req Request) (Summary, error) {
	s := Summary{Files: []FileChange{}}
	head, err := e.git(ctx, root, 256, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return s, ErrNotARepository
	}
	s.HeadSHA = strings.TrimSpace(string(head))
	branch, err := e.git(ctx, root, 4096, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		s.HeadBranch = strings.TrimSpace(string(branch))
	}
	s.Scope = req.Scope
	if s.Scope != "" && s.Scope != Branch && s.Scope != Uncommitted {
		return s, ErrInvalidScope
	}
	if s.Scope != Uncommitted {
		s.BaseRef, s.BaseSource, err = e.resolveBase(ctx, root, req.BaseRef)
		if err == nil {
			var base []byte
			base, err = e.git(ctx, root, 256, "merge-base", s.BaseRef, "HEAD")
			s.MergeBaseSHA = strings.TrimSpace(string(base))
		}
		if s.Scope == Branch && err != nil {
			return s, ErrBaseNotFound
		}
		if s.Scope == "" {
			s.Scope = Uncommitted
			if err == nil && s.MergeBaseSHA != s.HeadSHA {
				s.Scope = Branch
			}
		}
	}
	if s.Scope == Uncommitted {
		s.BaseRef = ""
		s.BaseSource = ""
		s.MergeBaseSHA = s.HeadSHA
	}
	s.Revision = fmt.Sprintf(
		"sha256:%x",
		sha256.Sum256(fmt.Appendf(nil, "%s\x00%s\x00%s\x00%s", s.Scope, s.BaseRef, s.HeadSHA, s.MergeBaseSHA)),
	)
	if req.Revision != "" && req.Revision != s.Revision {
		return s, ErrStale
	}
	return s, nil
}
