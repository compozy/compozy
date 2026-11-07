package changes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/worktree"
)

func (e *GitEngine) sensitive(ctx context.Context, root projectfiles.Root, path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	c, err := e.reader.Read(ctx, root, path, projectfiles.ReadOptions{})
	if errors.Is(err, projectfiles.ErrNotFound) {
		return projectfiles.Sensitive(path), nil
	}
	return c.Sensitive, err
}

func (e *GitEngine) Patches(
	ctx context.Context,
	root projectfiles.Root,
	req Request,
	targets []PatchTarget,
) ([]Patch, error) {
	if len(targets) == 0 || len(targets) > 20 {
		return nil, ErrTooManyPaths
	}
	size := 0
	for _, t := range targets {
		if err := validateTarget(t); err != nil {
			return nil, err
		}
		size += len(t.Path) + len(t.OldPath)
	}
	if size > 8192 {
		return nil, ErrTooManyPaths
	}
	s, err := e.resolve(ctx, root, req)
	if err != nil {
		return nil, err
	}
	result := make([]Patch, 0, len(targets))
	for _, t := range targets {
		p, err := e.patch(ctx, root, s, req, t)
		if err != nil {
			return nil, err
		}
		result = append(result, p)
	}

	if _, err := e.resolve(ctx, root, Request{Scope: s.Scope, BaseRef: req.BaseRef, Revision: s.Revision}); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *GitEngine) patch(
	ctx context.Context,
	root projectfiles.Root,
	s Summary,
	req Request,
	t PatchTarget,
) (Patch, error) {
	before, err := e.fingerprint(ctx, root, s.MergeBaseSHA, t)
	if err != nil {
		return Patch{}, err
	}
	p := Patch{Path: t.Path, Fingerprint: before}
	for _, path := range []string{t.Path, t.OldPath} {
		sensitive, err := e.sensitive(ctx, root, path)
		if err != nil {
			return Patch{}, err
		}
		p.Sensitive = p.Sensitive || sensitive
	}
	if p.Sensitive && !req.Reveal {
		after, err := e.fingerprint(ctx, root, s.MergeBaseSHA, t)
		if err != nil {
			return Patch{}, err
		}
		if after != before {
			return Patch{}, ErrStale
		}
		return p, nil
	}
	p.Sensitive = false
	p, err = e.renderPatch(ctx, root, s, t, p)
	if err != nil {
		return Patch{}, err
	}

	after, err := e.fingerprint(ctx, root, s.MergeBaseSHA, t)
	if err != nil {
		return Patch{}, err
	}
	if before != after {
		return Patch{}, ErrStale
	}
	return p, nil
}

func (e *GitEngine) renderPatch(
	ctx context.Context,
	root projectfiles.Root,
	s Summary,
	t PatchTarget,
	p Patch,
) (Patch, error) {
	args := []string{"-M", "--relative", "--src-prefix=a/", "--dst-prefix=b/", "-U3", s.MergeBaseSHA, "--", t.Path}
	if t.OldPath != "" {
		args = append(args, t.OldPath)
	}
	untracked := false
	if _, err := e.git(
		ctx,
		root,
		256,
		"rev-parse",
		"--verify",
		s.MergeBaseSHA+":./"+t.Path,
	); err != nil &&
		t.OldPath == "" {
		untracked = true
		args = []string{"--no-index", "--src-prefix=a/", "--dst-prefix=b/", "-U3", "--", "/dev/null", t.Path}
	}
	var patch bytes.Buffer
	err := e.stream(ctx, root, func(o worktree.GitOutput) {
		if o.Stream != worktree.GitOutputStdout || p.TooLarge {
			return
		}
		if patch.Len()+len(o.Chunk) > PatchLimit {
			p.TooLarge = true
			patch.Reset()
			return
		}
		patch.Write(o.Chunk)
	}, "diff", args...)
	if err != nil {
		exit, ok := errors.AsType[*exec.ExitError](err)
		if !untracked || !ok || exit.ExitCode() != 1 {
			return Patch{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
	}
	if !p.TooLarge && patch.Len() > 0 {
		files, _, err := gitdiff.Parse(bytes.NewReader(patch.Bytes()))
		if err != nil {
			return Patch{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		for _, file := range files {
			if file.NewName == t.Path || (file.IsDelete && file.OldName == t.Path) {
				p.Patch = file.String()
				break
			}
		}
		if p.Patch == "" {
			return Patch{}, projectfiles.ErrInvalidPath
		}
	}

	return p, nil
}

func (e *GitEngine) Contents(
	ctx context.Context,
	root projectfiles.Root,
	req Request,
	t PatchTarget,
) (Contents, error) {
	if err := validateTarget(t); err != nil {
		return Contents{}, err
	}
	s, err := e.resolve(ctx, root, req)
	if err != nil {
		return Contents{}, err
	}
	before, err := e.fingerprint(ctx, root, s.MergeBaseSHA, t)
	if err != nil {
		return Contents{}, err
	}
	result := Contents{Path: t.Path, OldPath: t.OldPath, Fingerprint: before}
	old := t.OldPath
	if old == "" {
		old = t.Path
	}
	if _, err := e.git(ctx, root, 256, "rev-parse", "--verify", s.MergeBaseSHA+":./"+old); err == nil {
		result.Old = &Side{}
		sensitive, err := e.sensitive(ctx, root, old)
		if err != nil {
			return Contents{}, err
		}
		if sensitive && !req.Reveal {
			result.Old.Sensitive = true
		} else {
			var data bytes.Buffer
			overflow := false
			err = e.stream(ctx, root, func(o worktree.GitOutput) {
				if o.Stream != worktree.GitOutputStdout {
					return
				}
				if data.Len()+len(o.Chunk) > PatchLimit {
					overflow = true
				}
				data.Write(o.Chunk[:min(len(o.Chunk), PatchLimit-data.Len())])
			}, "cat-file", "blob", s.MergeBaseSHA+":./"+old)
			if err != nil {
				return Contents{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
			}
			if overflow {
				return Contents{}, ErrTooLargeToExpand
			}
			result.Old.Text = data.String()
		}
	}
	c, err := e.reader.Read(ctx, root, t.Path, projectfiles.ReadOptions{Reveal: req.Reveal})
	if err == nil {
		if c.Kind != projectfiles.ContentKindSensitive && c.Size > PatchLimit {
			return Contents{}, ErrTooLargeToExpand
		}
		result.New = &Side{Text: c.Text, Sensitive: c.Kind == projectfiles.ContentKindSensitive}
	} else if !errors.Is(err, projectfiles.ErrNotFound) {
		return Contents{}, err
	}
	after, err := e.fingerprint(ctx, root, s.MergeBaseSHA, t)
	if err != nil {
		return Contents{}, err
	}
	if before != after {
		return Contents{}, ErrStale
	}
	if _, err := e.resolve(ctx, root, Request{Scope: s.Scope, BaseRef: req.BaseRef, Revision: s.Revision}); err != nil {
		return Contents{}, err
	}
	return result, nil
}
func (e *GitEngine) Bases(ctx context.Context, root projectfiles.Root, q string, limit int) (BaseList, error) {
	result := BaseList{Refs: []BaseRef{}}
	if limit == 0 {
		limit = 200
	}
	if limit < 1 || limit > 500 {
		return result, ErrTooManyPaths
	}
	raw, err := e.git(
		ctx,
		root,
		metadataLimit,
		"for-each-ref",
		"--format=%(refname:short)%00%(refname)",
		"refs/heads",
		"refs/remotes",
	)
	if err != nil {
		return result, ErrNotARepository
	}
	def, source, baseErr := e.resolveBase(ctx, root, "")
	for line := range strings.SplitSeq(string(raw), "\n") {
		short, full, ok := strings.Cut(line, "\x00")
		if !ok || strings.HasSuffix(full, "/HEAD") {
			continue
		}
		ref := BaseRef{Ref: short, Kind: "local", IsDefault: baseErr == nil && short == def}
		if strings.HasPrefix(full, "refs/remotes/") {
			ref.Kind = "remote"
		}
		if ref.IsDefault {
			ref.Source = source
			result.Default = new(ref)
			result.Resolvable = true
			continue
		}
		if strings.Contains(short, q) {
			result.Refs = append(result.Refs, ref)
		}
	}
	if result.Default != nil {
		result.Refs = append([]BaseRef{*result.Default}, result.Refs...)
	}
	if len(result.Refs) > limit {
		result.Truncated = true
		result.Refs = result.Refs[:limit]
	}
	return result, nil
}
