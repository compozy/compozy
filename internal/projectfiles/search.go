package projectfiles

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charlievieth/fastwalk"
	"github.com/compozy/compozy/internal/workspacefiles"
	"github.com/compozy/compozy/internal/worktree"
)

type searchState struct {
	result         SearchResult
	pattern        searchPattern
	root           Root
	search         *FileSearch
	seen           map[string]bool
	mu             sync.Mutex
	sensitivePaths []string
}

func (s *FileSearch) Search(ctx context.Context, root Root, q SearchQuery) (SearchResult, error) {
	pattern, err := compileSearch(q)
	if err != nil {
		return SearchResult{}, err
	}
	root, err = CanonicalRoot(root)
	if err != nil {
		return SearchResult{}, err
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	state := searchState{
		result:  SearchResult{Files: []SearchFile{}},
		pattern: pattern,
		root:    root,
		search:  s,
		seen:    map[string]bool{},
	}
	entries, _, err := s.index.Paths(bounded, root.AbsPath)
	if err != nil {
		if ctx.Err() != nil {
			return SearchResult{}, ctx.Err()
		}
		if bounded.Err() != nil {
			state.result.TimedOut = true
			return state.result, nil
		}
		return SearchResult{}, err
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return SearchResult{}, ctx.Err()
		}
		if bounded.Err() != nil {
			state.result.TimedOut = true
			return state.result, nil
		}

		if entry.Kind != workspacefiles.KindFile || !pattern.accepts(entry.Path) {
			continue
		}
		_, sensitive, err := resolvePath(root, entry.Path)
		if err == nil && sensitive {
			state.result.SensitiveExcluded++
			state.sensitivePaths = append(state.sensitivePaths, entry.Path)
		}
	}
	argv, err := GitReadArgs(bounded, s.runner, root.AbsPath, "rev-parse", "--is-inside-work-tree")
	if err != nil {
		return SearchResult{}, err
	}
	_, stderr, gitErr := s.runner.RunStreaming(bounded, root.AbsPath, nil, argv...)
	switch {
	case gitErr == nil:
		state.result.Engine = "git_grep"
		err = state.grep(bounded)
	case strings.Contains(string(stderr), "not a git repository"):
		state.result.Engine = "walk"
		err = state.walk(bounded)
	default:
		err = gitErr
	}
	if ctx.Err() != nil {
		return SearchResult{}, ctx.Err()
	}
	if errors.Is(bounded.Err(), context.DeadlineExceeded) {
		state.result.TimedOut = true
		err = nil
	}
	slices.SortFunc(state.result.Files, func(a, b SearchFile) int { return strings.Compare(a.Path, b.Path) })
	state.result.TotalFiles = len(state.result.Files)
	return state.result, err
}
func (s *searchState) addFile(ctx context.Context, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[path] || !s.pattern.accepts(path) || s.result.TotalMatches >= 2000 {
		return
	}
	s.seen[path] = true
	if _, _, err := resolvePath(s.root, path); err != nil {
		return
	}
	info, err := os.Lstat(filepath.Join(s.root.AbsPath, filepath.FromSlash(path)))
	if err != nil || !info.Mode().IsRegular() || info.Size() > TextLimit {
		return
	}
	content, err := s.search.reader.Read(ctx, s.root, path, ReadOptions{})
	if err != nil || content.Kind != ContentKindText || content.Truncated {
		return
	}
	file := SearchFile{Path: path, Matches: []SearchMatch{}}
	lineNumber := 0
	for line := range strings.SplitSeq(content.Text, "\n") {
		if ctx.Err() != nil {
			break
		}
		lineNumber++
		ranges := s.pattern.ranges(line)
		if len(ranges) == 0 {
			continue
		}
		if len(file.Matches) == 100 {
			file.TruncatedMatches = true
			break
		}
		if s.result.TotalMatches == 2000 {
			s.result.Truncated = true
			break
		}
		column := ranges[0][0] + 1
		text, clipped, offset := excerpt(line, ranges)
		file.Matches = append(
			file.Matches,
			SearchMatch{Line: lineNumber, Column: column, Text: text, Ranges: clipped, TextOffset: offset},
		)
		s.result.TotalMatches++
	}
	if len(file.Matches) > 0 {
		s.result.Files = append(s.result.Files, file)
	}
}
func (s *searchState) grep(ctx context.Context) error {
	bounded, cancel := context.WithCancel(ctx)
	defer cancel()
	args := s.grepArgs()
	argv, err := GitReadArgs(bounded, s.search.runner, s.root.AbsPath, "grep", args...)
	if err != nil {
		return err
	}
	argv = append(
		[]string{"-c", "grep.threads=" + strconv.Itoa(min(8, runtime.NumCPU())), "-c", "core.bigFileThreshold=1m"},
		argv...)

	var pending []byte
	phase := 0
	path := ""
	stopped := false
	_, _, err = s.search.runner.RunStreaming(bounded, s.root.AbsPath, func(out worktree.GitOutput) {
		if out.Stream != worktree.GitOutputStdout || stopped {
			return
		}
		chunk := out.Chunk
		for len(chunk) > 0 {
			delimiter := byte(0)
			if phase == 3 {
				delimiter = '\n'
			}
			end := bytes.IndexByte(chunk, delimiter)
			if end < 0 {
				if phase == 0 && len(pending) < 8192 {
					pending = append(pending, chunk...)
				}
				return
			}
			if phase == 0 {
				pending = append(pending, chunk[:end]...)
				path = string(pending)
				pending = nil
			}
			chunk = chunk[end+1:]
			if phase == 2 {
				s.addFile(ctx, path)
				if s.result.TotalMatches >= 2000 {
					s.result.Truncated = true
					stopped = true
					cancel()
					return
				}
			}
			phase = (phase + 1) % 4
		}
	}, argv...)

	if stopped {
		return nil
	}
	if exit, ok := errors.AsType[*exec.ExitError](err); ok && exit.ExitCode() == 1 {
		return nil
	}
	return err
}
func (s *searchState) walk(ctx context.Context) error {
	err := fastwalk.Walk(
		&fastwalk.Config{NumWorkers: 1, Follow: false},
		s.root.AbsPath,
		func(path string, entry fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if entry.Name() == ".git" || entry.Name() == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return nil
			}
			rel, err := filepath.Rel(s.root.AbsPath, path)
			if err != nil {
				return err
			}
			s.addFile(ctx, filepath.ToSlash(rel))
			if s.result.TotalMatches >= 2000 {
				s.result.Truncated = true
				return filepath.SkipAll
			}
			return nil
		},
	)
	if errors.Is(err, filepath.SkipAll) {
		return nil
	}
	return err
}

func (s *searchState) grepArgs() []string {
	args := []string{"--no-color", "-I", "-n", "--column", "-z", "--untracked", "--exclude-standard"}
	if s.pattern.query.Regex {
		args = append(args, "-E")
	} else {
		args = append(args, "-F")
	}
	if !s.pattern.query.CaseSensitive {
		args = append(args, "-i")
	}
	if s.pattern.query.WholeWord {
		args = append(args, "-w")
	}
	args = append(args, "-e", s.pattern.query.Pattern, "--")
	if len(s.pattern.query.Include) == 0 {
		args = append(args, ".")
	}
	for _, glob := range s.pattern.query.Include {
		args = append(args, ":(glob)"+glob)
	}
	for _, glob := range s.pattern.query.Exclude {
		args = append(args, ":(exclude,glob)"+glob)
	}
	sensitiveGlobs := []string{
		".env", "*.pem", "*.key", "*.p12", "*.pfx", "id_rsa*", "id_ed25519*", "id_ecdsa*",
		".npmrc", ".pypirc", ".netrc", "credentials.json", "service-account*.json", "*.keystore", ".git-credentials",
	}
	for _, glob := range sensitiveGlobs {
		args = append(args, ":(exclude,glob)**/"+glob)
	}
	for _, path := range s.sensitivePaths {
		args = append(args, ":(exclude,literal)"+path)
	}

	return args
}
