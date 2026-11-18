package changes

import (
	"context"
	"errors"
	"slices"
	"sync"

	"github.com/compozy/compozy/internal/projectfiles"
)

const fileReadConcurrency = 8

type decoratedFile struct {
	file FileChange
	err  error
}

func validChange(f FileChange) bool {
	return projectfiles.ValidatePath(f.Path) == nil && (f.OldPath == "" || projectfiles.ValidatePath(f.OldPath) == nil)
}

func (e *GitEngine) readDecorations(
	ctx context.Context,
	root projectfiles.Root,
	paths []string,
	files map[string]FileChange,
	attributes map[string]bool,
) []decoratedFile {
	results := make([]decoratedFile, len(paths))
	var work sync.WaitGroup
	for i, path := range paths {
		work.Go(func() {
			f := files[path]
			if !validChange(f) {
				return
			}
			f.Sensitive = projectfiles.Sensitive(path) || projectfiles.Sensitive(f.OldPath)
			f.Generated = generatedPath(path) || attributes[path]
			fingerprint, sensitive, err := e.fingerprintDetails(
				ctx,
				root,
				PatchTarget{path, f.OldPath},
				f.oldOID,
				f.Status,
			)
			f.Fingerprint = fingerprint
			f.Sensitive = f.Sensitive || sensitive
			results[i] = decoratedFile{file: f, err: err}
		})
	}
	work.Wait()
	return results
}

func (e *GitEngine) decorate(
	ctx context.Context,
	root projectfiles.Root,
	s Summary,
	files map[string]FileChange,
) (Summary, error) {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	attributes := e.generatedAttributes(ctx, root, paths[:min(len(paths), FileLimit)])
	for i := 0; i < len(paths); {
		n := min(fileReadConcurrency, len(paths)-i)
		var decorated []decoratedFile
		if len(s.Files) < FileLimit {
			n = min(n, FileLimit-len(s.Files))
			decorated = e.readDecorations(ctx, root, paths[i:i+n], files, attributes)
		}
		for j, path := range paths[i : i+n] {
			f := files[path]
			if !validChange(f) {
				continue
			}
			s.Totals.Files++
			s.Totals.Additions += f.Additions
			s.Totals.Deletions += f.Deletions
			s.Totals.Partial = s.Totals.Partial || f.Binary
			if len(s.Files) >= FileLimit {
				s.Truncated = true
				continue
			}
			result := decorated[j]
			if errors.Is(result.err, projectfiles.ErrPathOutsideRoot) ||
				errors.Is(result.err, projectfiles.ErrPathDenied) ||
				errors.Is(result.err, projectfiles.ErrNotAFile) {
				s.Totals.Partial = true
				continue
			}
			if result.err != nil {
				return s, result.err
			}
			s.Files = append(s.Files, result.file)
		}
		i += n
		if err := ctx.Err(); err != nil {
			return s, err
		}
	}
	return s, nil
}
