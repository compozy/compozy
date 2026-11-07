package projectfiles

import (
	"context"
	"errors"
)

var ErrInvalidQuery = errors.New("projectfiles: invalid query")

type Finder interface {
	Find(context.Context, Root, string, int) (FindResult, error)
}
type Searcher interface {
	Search(context.Context, Root, SearchQuery) (SearchResult, error)
}
type FindResult struct {
	Results        []FindMatch
	IndexedFiles   int
	IndexTruncated bool
}
type FindMatch struct {
	Path        string
	Kind        EntryKind
	Sensitive   bool
	Score       int
	MatchRanges [][2]int
}
type SearchQuery struct {
	Pattern                         string
	Regex, CaseSensitive, WholeWord bool
	Include, Exclude                []string
}
type SearchResult struct {
	Files                    []SearchFile
	TotalFiles, TotalMatches int
	Truncated, TimedOut      bool
	SensitiveExcluded        int
	Engine                   string
}
type SearchFile struct {
	Path             string
	Matches          []SearchMatch
	TruncatedMatches bool
}
type SearchMatch struct {
	Line, Column int
	Text         string
	Ranges       [][2]int
	TextOffset   int
}
