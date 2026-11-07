package projectfiles

import (
	"bytes"
	"cmp"
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const FindIndexLimit = 200000

var initFuzzy = sync.OnceFunc(func() { algo.Init("path") })

type FileSearch struct {
	runner worktree.StreamingGitRunner
	index  workspacefiles.Index
	reader *FileReader
}

var _ Finder = (*FileSearch)(nil)
var _ Searcher = (*FileSearch)(nil)

type collectorGit struct{ runner worktree.StreamingGitRunner }

var _ workspacefiles.StreamingGitRunner = collectorGit{}

func (g collectorGit) Run(ctx context.Context, root string, args ...string) ([]byte, []byte, error) {
	var output bytes.Buffer
	stderr, err := g.RunStreaming(ctx, root, func(chunk []byte) { output.Write(chunk) }, args...)
	return output.Bytes(), stderr, err
}

func (g collectorGit) RunStreaming(
	ctx context.Context,
	root string,
	visit func([]byte),
	args ...string,
) ([]byte, error) {
	argv, err := GitReadArgs(ctx, g.runner, root, args[0], args[1:]...)
	if err != nil {
		return nil, err
	}
	_, stderr, err := g.runner.RunStreaming(ctx, root, func(out worktree.GitOutput) {
		if out.Stream == worktree.GitOutputStdout {
			visit(out.Chunk)
		}
	}, argv...)
	return stderr, err
}
func NewSearch(runner worktree.StreamingGitRunner, opts ...workspacefiles.Option) *FileSearch {
	if runner == nil {
		reader := NewReader()
		runner = reader.gitRunner
	}
	options := []workspacefiles.Option{
		workspacefiles.WithMaxFiles(FindIndexLimit),
		workspacefiles.WithMaxWalkEntries(4 * FindIndexLimit),
		workspacefiles.WithGitTimeout(10 * time.Second),
	}
	options = append(options, opts...)
	return &FileSearch{
		runner: runner,
		index:  workspacefiles.New(collectorGit{runner}, options...),
		reader: NewReader(WithGitRunner(runner)),
	}
}
func (s *FileSearch) Find(ctx context.Context, root Root, query string, limit int) (FindResult, error) {
	if strings.TrimSpace(query) == "" {
		return FindResult{}, ErrInvalidQuery
	}
	root, err := CanonicalRoot(root)
	if err != nil {
		return FindResult{}, err
	}
	entries, truncated, err := s.index.Paths(ctx, root.AbsPath)
	if err != nil {
		return FindResult{}, err
	}
	initFuzzy()
	result := FindResult{Results: []FindMatch{}, IndexTruncated: truncated}
	pattern := []rune(strings.ToLower(query))
	slab := util.MakeSlab(100*1024, 2048)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return FindResult{}, err
		}
		path := strings.TrimSuffix(entry.Path, "/")
		if entry.Kind == workspacefiles.KindFile {
			result.IndexedFiles++
		}
		chars := util.ToChars([]byte(path))
		match, positions := algo.FuzzyMatchV2(false, true, true, &chars, pattern, true, slab)
		if match.Score < 0 || positions == nil {
			continue
		}
		offsets := []int{}
		for offset := range path {
			offsets = append(offsets, offset)
		}
		offsets = append(offsets, len(path))
		slices.Sort(*positions)
		ranges := make([][2]int, 0, len(*positions))
		for _, pos := range *positions {
			start, end := offsets[pos], offsets[pos+1]
			if len(ranges) > 0 && ranges[len(ranges)-1][1] == start {
				ranges[len(ranges)-1][1] = end
			} else {
				ranges = append(ranges, [2]int{start, end})
			}
		}
		kind := EntryKindFile
		if entry.Kind == workspacefiles.KindFolder {
			kind = EntryKindDirectory
		}
		result.Results = append(
			result.Results,
			FindMatch{Path: path, Kind: kind, Score: match.Score, MatchRanges: ranges},
		)
	}
	slices.SortFunc(
		result.Results,
		func(a, b FindMatch) int { return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Path, b.Path)) },
	)
	if limit <= 0 || limit > 50 {
		limit = 50
	}
	ranked := result.Results
	result.Results = make([]FindMatch, 0, min(limit, len(ranked)))
	for _, match := range ranked {
		_, sensitive, err := resolvePath(root, match.Path)
		if err != nil {
			continue
		}
		match.Sensitive = sensitive
		result.Results = append(result.Results, match)
		if len(result.Results) == limit {
			break
		}
	}
	return result, nil
}
