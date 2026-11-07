package changes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/worktree"
)

const relativeDiffFlag = "--relative"
const untrackedStatus Status = "untracked"

func batchPaths(targets []PatchTarget) []string {
	paths := make([]string, 0, len(targets)*2)
	for _, t := range targets {
		paths = append(paths, t.Path)
		if t.OldPath != "" {
			paths = append(paths, t.OldPath)
		}
	}
	return paths
}

func (e *GitEngine) batchFingerprints(
	ctx context.Context,
	root projectfiles.Root,
	s Summary,
	targets []PatchTarget,
) (map[string]string, error) {
	paths := batchPaths(targets)
	args := append([]string{"-M", "--raw", "-z", "--abbrev=40", relativeDiffFlag, s.MergeBaseSHA, "--"}, paths...)
	raw, err := e.git(ctx, root, metadataLimit, "diff", args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	files := parseRaw(raw)
	args = append([]string{"--porcelain=v2", "-z", "--"}, paths...)
	status, err := e.git(ctx, root, metadataLimit, "status", args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	prefix, err := e.git(ctx, root, 4096, "rev-parse", "--show-prefix")
	if err != nil {
		return nil, err
	}
	for record := range strings.SplitSeq(string(status), "\x00") {
		if strings.HasPrefix(record, "u ") {
			parts := strings.SplitN(record, " ", 11)
			if len(parts) == 11 {
				name, ok := strings.CutPrefix(parts[10], strings.TrimSuffix(string(prefix), "\n"))
				if ok {
					f := files[name]
					f.Status = Conflicted
					files[name] = f
				}
			}
		}
	}
	result := make(map[string]string, len(targets))
	for _, t := range targets {
		f, ok := files[t.Path]
		if !ok {
			f = FileChange{Status: untrackedStatus}
		}
		value, err := e.fingerprintWithOID(ctx, root, t, f.oldOID, f.Status)
		if err != nil {
			if !patchItemUnavailable(err) {
				return nil, err
			}
			// An empty fingerprint marks an unavailable target, not a failed batch.
			value = ""
		}
		result[t.Path] = value
	}
	return result, nil
}

func (e *GitEngine) baseObjects(
	ctx context.Context,
	root projectfiles.Root,
	s Summary,
	targets []PatchTarget,
) (map[string]bool, error) {
	args := append([]string{"-z", s.MergeBaseSHA, "--"}, batchPaths(targets)...)
	raw, err := e.git(ctx, root, metadataLimit, "ls-tree", args...)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	var input strings.Builder
	for entry := range strings.SplitSeq(string(raw), "\x00") {
		metadata, path, ok := strings.Cut(entry, "\t")
		fields := strings.Fields(metadata)
		if !ok || len(fields) != 3 {
			continue
		}
		// A former tree has no blob at this exact path. If Git only emits
		// deleted children, render the replacement file as an untracked addition.
		found[path] = fields[1] != "tree"
		if fields[1] == "blob" {
			input.WriteString(fields[2] + "\n")
		}
	}
	if runner, ok := e.runner.(worktree.GitInputRunner); ok && input.Len() > 0 {
		argv, err := projectfiles.GitReadArgs(ctx, e.runner, root.AbsPath, "cat-file", "--batch-check")
		if err != nil {
			return nil, err
		}
		data, _, err := runner.RunWithInput(
			ctx,
			root.AbsPath,
			input.String(),
			append([]string{"--literal-pathspecs"}, argv...)...)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, &gitReadError{command: "cat-file", err: err})
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 3 {
				return nil, ErrUnavailable
			}
		}
	}
	return found, nil
}

func (e *GitEngine) patchBatch(
	ctx context.Context,
	root projectfiles.Root,
	s Summary,
	req Request,
	targets []PatchTarget,
) ([]Patch, error) {
	before, err := e.batchFingerprints(ctx, root, s, targets)
	if err != nil {
		return nil, err
	}
	objects, err := e.baseObjects(ctx, root, s, targets)
	if err != nil {
		return nil, err
	}
	result, allowed, err := e.preparePatches(ctx, root, req, targets, before)
	if err != nil {
		return nil, err
	}
	rendered := map[string]Patch{}
	if len(allowed) > 0 {
		splitter := newPatchSplitter()
		args := append(
			[]string{"-M", relativeDiffFlag, "--src-prefix=a/", "--dst-prefix=b/", "-U3", s.MergeBaseSHA, "--"},
			batchPaths(allowed)...)
		err = e.stream(ctx, root, func(out worktree.GitOutput) {
			if out.Stream == worktree.GitOutputStdout {
				splitter.feed(out.Chunk)
			}
		}, "diff", args...)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		rendered, err = splitter.finish(ctx)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
	}
	for i, t := range targets {
		if result[i].Sensitive || result[i].Fingerprint == "" {
			continue
		}
		if p, ok := rendered[t.Path]; ok {
			result[i].Patch = p.Patch
			result[i].TooLarge = p.TooLarge
			continue
		}
		if !objects[t.Path] && t.OldPath == "" {
			p, err := e.untrackedPatch(ctx, root, t.Path)
			if errors.Is(err, projectfiles.ErrNotFound) {
				continue
			}
			if err != nil {
				if !patchItemUnavailable(err) {
					return nil, err
				}
				result[i].Fingerprint = ""
				continue
			}
			result[i].Patch = p.Patch
			result[i].TooLarge = p.TooLarge
		}
	}
	after, err := e.batchFingerprints(ctx, root, s, targets)
	if err != nil {
		return nil, err
	}
	for _, t := range targets {
		if before[t.Path] != after[t.Path] {
			return nil, ErrStale
		}
	}
	return result, nil
}

func (e *GitEngine) preparePatches(
	ctx context.Context,
	root projectfiles.Root,
	req Request,
	targets []PatchTarget,
	before map[string]string,
) ([]Patch, []PatchTarget, error) {
	result := make([]Patch, len(targets))
	allowed := make([]PatchTarget, 0, len(targets))
	for i, t := range targets {
		result[i] = Patch{Path: t.Path, Fingerprint: before[t.Path]}
		if before[t.Path] == "" {
			continue
		}
		unavailable := false
		for _, path := range []string{t.Path, t.OldPath} {
			sensitive, err := e.sensitive(ctx, root, path)
			if err != nil {
				if !patchItemUnavailable(err) {
					return nil, nil, err
				}
				unavailable = true
				break
			}
			result[i].Sensitive = result[i].Sensitive || sensitive
		}
		if unavailable {
			result[i].Fingerprint = ""
			continue
		}
		if result[i].Sensitive && !req.Reveal {
			continue
		}
		result[i].Sensitive = false
		allowed = append(allowed, t)
	}
	return result, allowed, nil
}

// Only file-kind changes and the Reader's transient not-found result are item-local.
// Confinement/validation failures (including joined errors) must retain their domain error.
func patchItemUnavailable(err error) bool {
	if errors.Is(err, projectfiles.ErrPathDenied) || errors.Is(err, projectfiles.ErrPathOutsideRoot) ||
		errors.Is(err, projectfiles.ErrInvalidPath) {
		return false
	}
	return errors.Is(err, projectfiles.ErrNotAFile) || errors.Is(err, projectfiles.ErrNotFound)
}
