package changes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/worktree"
)

const (
	FileLimit     = 3000
	PatchLimit    = 1 << 20
	metadataLimit = 64 << 20
)

var errSummaryLeaderCanceled = errors.New("shared request leader canceled")

func (e *GitEngine) Summary(ctx context.Context, root projectfiles.Root, req Request) (Summary, error) {
	key := fmt.Sprintf("%#v:%#v", root, req)
	for {
		if err := ctx.Err(); err != nil {
			return Summary{}, err
		}
		result := e.summaries.DoChan(key, func() (any, error) {
			value, err := e.summary(ctx, root, req)
			if ctx.Err() != nil {
				return nil, errSummaryLeaderCanceled
			}
			return value, err
		})
		select {
		case <-ctx.Done():
			return Summary{}, ctx.Err()
		case item := <-result:
			if err := ctx.Err(); err != nil {
				return Summary{}, err
			}
			// A canceled leader releases its Git work. Active waiters elect a new
			// leader with their own bounded runner context instead of inheriting cancellation.
			if errors.Is(item.Err, errSummaryLeaderCanceled) {
				continue
			}
			if item.Err != nil {
				return Summary{}, item.Err
			}
			value, ok := item.Val.(Summary)
			if !ok {
				return Summary{}, ErrUnavailable
			}
			value.Files = slices.Clone(value.Files)
			return value, nil
		}
	}
}
func (e *GitEngine) summary(ctx context.Context, root projectfiles.Root, req Request) (Summary, error) {
	started := time.Now()
	for range 2 {
		before, err := e.snapshot(ctx, root, req)
		if err != nil {
			return Summary{}, err
		}
		after, err := e.snapshot(ctx, root, req)
		if err != nil {
			return Summary{}, err
		}
		if stableSummary(before, after) {
			slog.DebugContext(
				ctx,
				"changes.summary",
				"scope",
				after.Scope,
				"base_source",
				after.BaseSource,
				"files",
				len(after.Files),
				"truncated",
				after.Truncated,
				"duration_ms",
				time.Since(started).Milliseconds(),
			)
			return after, nil
		}
	}
	return Summary{}, ErrStale
}
func (e *GitEngine) snapshot(ctx context.Context, root projectfiles.Root, req Request) (Summary, error) {
	s, err := e.resolve(ctx, root, req)
	if err != nil {
		return s, err
	}
	raw, err := e.git(
		ctx,
		root,
		metadataLimit,
		"diff",
		"-M",
		"--raw",
		"--numstat",
		"-z",
		"--abbrev=40",
		relativeDiffFlag,
		s.MergeBaseSHA,
		"--",
		".",
	)
	if err != nil {
		return s, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	raw, num := splitRawNumstat(raw)
	files := parseRaw(raw)
	if _, err := worktree.ParseNumstat(num); err != nil {
		return s, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	applyNumstat(files, num)
	status, err := e.git(ctx, root, metadataLimit, "status", "--porcelain=v2", "--branch", "-z", "--", ".")
	if err != nil {
		return s, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if _, err := worktree.ParseStatusV2(status); err != nil {
		return s, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	prefix, err := e.git(ctx, root, 4096, "rev-parse", "--show-prefix")
	if err != nil {
		return s, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	for record := range bytes.SplitSeq(status, []byte{0}) {
		if len(record) > 0 && record[0] == 'u' {
			parts := strings.SplitN(string(record), " ", 11)
			if len(parts) == 11 {
				if name, ok := strings.CutPrefix(parts[10], strings.TrimSpace(string(prefix))); ok {
					f := files[name]
					f.Path = name
					f.Status = Conflicted
					files[name] = f
				}
			}
		}
	}
	untracked, err := e.git(ctx, root, metadataLimit, "ls-files", "--others", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return s, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	if err := e.addUntracked(ctx, root, untracked, files, &s.Totals); err != nil {
		return s, err
	}

	return e.decorate(ctx, root, s, files)
}
func parseRaw(raw []byte) map[string]FileChange {
	fields := strings.Split(string(raw), "\x00")
	files := map[string]FileChange{}
	for i := 0; i+1 < len(fields); i++ {
		parts := strings.Fields(fields[i])
		if len(parts) != 5 {
			continue
		}
		i++
		path := fields[i]
		f := FileChange{Path: path, oldOID: parts[2]}
		switch parts[4][0] {
		case 'A':
			f.Status = "added"
		case 'D':
			f.Status = "deleted"
		case 'R':
			f.Status = "renamed"
			f.OldPath = path
			if i+1 >= len(fields) {
				continue
			}
			i++
			f.Path = fields[i]
		case 'T':
			f.Status = "type_changed"
		case 'U':
			f.Status = Conflicted
		default:
			f.Status = "modified"
		}
		files[f.Path] = f
	}
	return files
}
func applyNumstat(files map[string]FileChange, raw []byte) {
	fields := strings.Split(string(raw), "\x00")
	for i := 0; i < len(fields); i++ {
		p := strings.SplitN(fields[i], "\t", 3)
		if len(p) != 3 {
			continue
		}
		path := p[2]
		if path == "" {
			if i+2 >= len(fields) {
				continue
			}
			i += 2
			path = fields[i]
		}
		f := files[path]
		if n, err := strconv.Atoi(p[0]); err == nil {
			f.Additions = n
		}
		if n, err := strconv.Atoi(p[1]); err == nil {
			f.Deletions = n
		}
		f.Binary = p[0] == "-"
		files[path] = f
	}
}
func generatedPath(path string) bool {
	name := filepath.Base(path)
	patterns := []string{
		"bun.lock", "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "go.sum", "Cargo.lock", "*.gen.*", "*_gen.go",
	}
	for _, pattern := range patterns {
		if ok, err := filepath.Match(pattern, name); err == nil && ok {
			return true
		}
	}
	return false
}
func (e *GitEngine) generatedAttributes(ctx context.Context, root projectfiles.Root, paths []string) map[string]bool {
	result := map[string]bool{}
	if len(paths) == 0 {
		return result
	}
	args := append([]string{"-z", "linguist-generated", "--"}, paths...)
	attr, err := e.git(ctx, root, metadataLimit, "check-attr", args...)
	if err != nil {
		return result
	}
	fields := strings.Split(string(attr), "\x00")
	for i := 0; i+2 < len(fields); i += 3 {
		result[fields[i]] = fields[i+2] == "set" || fields[i+2] == "true"
	}
	return result
}

func stableSummary(a, b Summary) bool {
	if a.Revision != b.Revision || a.Totals.Files != b.Totals.Files || len(a.Files) != len(b.Files) {
		return false
	}
	for i, f := range a.Files {
		other := b.Files[i]
		if f.Path != other.Path || f.OldPath != other.OldPath || f.Status != other.Status ||
			f.Fingerprint != other.Fingerprint {
			return false
		}
	}
	return true
}

func splitRawNumstat(data []byte) ([]byte, []byte) {
	fields := bytes.Split(data, []byte{0})
	i := 0
	for i < len(fields) && bytes.HasPrefix(fields[i], []byte(":")) {
		parts := strings.Fields(string(fields[i]))
		step := 2
		if len(parts) == 5 && (strings.HasPrefix(parts[4], "R") || strings.HasPrefix(parts[4], "C")) {
			step = 3
		}
		i += step
	}
	i = min(i, len(fields))
	return append(bytes.Join(fields[:i], []byte{0}), 0), bytes.Join(fields[i:], []byte{0})
}
