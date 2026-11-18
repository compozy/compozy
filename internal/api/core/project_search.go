package core

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) FindFiles(c *gin.Context)                 { h.findProjectFiles(c) }
func (h *BaseHandlers) FindSessionFiles(c *gin.Context)          { h.findProjectFiles(c) }
func (h *BaseHandlers) SearchFileContents(c *gin.Context)        { h.searchProjectFiles(c) }
func (h *BaseHandlers) SearchSessionFileContents(c *gin.Context) { h.searchProjectFiles(c) }
func (h *BaseHandlers) searchError(c *gin.Context, err error) {
	if errors.Is(err, projectfiles.ErrInvalidQuery) {
		h.respondSessionFilesError(c, 400, "invalid_query", err)
		return
	}
	h.projectFileError(c, err)
}
func (h *BaseHandlers) findProjectFiles(c *gin.Context) {
	root, ok := h.projectFileRoot(c)
	if !ok {
		return
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			h.searchError(c, projectfiles.ErrInvalidQuery)
			return
		}
		limit = value
	}
	result, err := h.FileFinder.Find(c.Request.Context(), root, c.Query("q"), limit)
	if err != nil {
		h.searchError(c, err)
		return
	}
	matches := make([]contract.FileFindMatch, 0, len(result.Results))
	for _, match := range result.Results {
		matches = append(
			matches,
			contract.FileFindMatch{
				Path:        match.Path,
				Kind:        string(match.Kind),
				Sensitive:   match.Sensitive,
				Score:       match.Score,
				MatchRanges: fileSearchRanges(match.MatchRanges),
			},
		)
	}
	c.JSON(
		http.StatusOK,
		contract.FileFindResponse{
			Root:           fileRootPayload(root),
			Query:          c.Query("q"),
			Results:        matches,
			IndexedFiles:   result.IndexedFiles,
			IndexTruncated: result.IndexTruncated,
		},
	)
}
func (h *BaseHandlers) searchProjectFiles(c *gin.Context) {
	root, ok := h.projectFileRoot(c)
	if !ok {
		return
	}
	q := projectfiles.SearchQuery{
		Pattern: c.Query("q"),
		Include: c.QueryArray("include"),
		Exclude: c.QueryArray("exclude"),
	}
	toggles := map[string]*bool{"regex": &q.Regex, "case_sensitive": &q.CaseSensitive, "whole_word": &q.WholeWord}
	for key, dest := range toggles {
		if raw := c.Query(key); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				h.searchError(c, projectfiles.ErrInvalidQuery)
				return
			}
			*dest = value
		}
	}
	result, err := h.FileSearcher.Search(c.Request.Context(), root, q)
	if err != nil {
		h.searchError(c, err)
		return
	}
	files := make([]contract.FileSearchFile, 0, len(result.Files))
	for _, file := range result.Files {
		matches := make([]contract.FileSearchMatch, 0, len(file.Matches))
		for _, m := range file.Matches {
			matches = append(
				matches,
				contract.FileSearchMatch{
					Line:       m.Line,
					Column:     m.Column,
					Text:       m.Text,
					Ranges:     fileSearchRanges(m.Ranges),
					TextOffset: m.TextOffset,
				},
			)
		}
		files = append(
			files,
			contract.FileSearchFile{Path: file.Path, Matches: matches, TruncatedMatches: file.TruncatedMatches},
		)
	}
	if q.Include == nil {
		q.Include = []string{}
	}
	if q.Exclude == nil {
		q.Exclude = []string{}
	}
	c.JSON(
		http.StatusOK,
		contract.FileSearchResponse{
			Root: fileRootPayload(root),
			Query: contract.FileSearchQuery{
				Pattern:       q.Pattern,
				Regex:         q.Regex,
				CaseSensitive: q.CaseSensitive,
				WholeWord:     q.WholeWord,
				Include:       q.Include,
				Exclude:       q.Exclude,
			},
			Engine:            result.Engine,
			Files:             files,
			Totals:            contract.FileSearchTotals{Files: result.TotalFiles, Matches: result.TotalMatches},
			Truncated:         result.Truncated,
			TimedOut:          result.TimedOut,
			SensitiveExcluded: result.SensitiveExcluded,
		},
	)
}

func fileSearchRanges(ranges [][2]int) [][]int {
	result := make([][]int, 0, len(ranges))
	for _, r := range ranges {
		result = append(result, []int{r[0], r[1]})
	}
	return result
}
