package transcript

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// SearchQuery selects literal, case-insensitive matches across retained history.
type SearchQuery struct {
	Query string
	Limit int
}

// Normalize applies the same bounded result limit as other transcript reads.
func (q SearchQuery) Normalize() (SearchQuery, error) {
	q.Query = strings.TrimSpace(q.Query)
	if q.Query == "" {
		return SearchQuery{}, errors.New("transcript: search query is required")
	}
	if len(q.Query) > 4096 {
		return SearchQuery{}, errors.New("transcript: search query must be at most 4096 bytes")
	}
	if q.Limit < 0 || q.Limit > maxPageLimit {
		return SearchQuery{}, fmt.Errorf("transcript: search limit must be between 0 and %d", maxPageLimit)
	}
	if q.Limit == 0 {
		q.Limit = defaultPageLimit
	}
	return q, nil
}

// SearchMatch identifies a matching materialized message by its stable start cursor.
type SearchMatch struct {
	Sequence int64  `json:"sequence"`
	TurnID   string `json:"turn_id"`
	Role     string `json:"role"`
	Snippet  string `json:"snippet"`
	// PartIndex and Field identify the first matching field in the projected message.
	// They are additive navigation hints; older readers can continue using the entry cursor.
	PartIndex *int   `json:"part_index,omitempty"`
	Field     string `json:"field,omitempty"`
}

// SearchResult reports whether additional matching messages were omitted.
type SearchResult struct {
	Matches   []SearchMatch `json:"matches"`
	Truncated bool          `json:"truncated"`
}

// OutlineEntry is one operator message and its latest reply in the same turn.
type OutlineEntry struct {
	Sequence     int64     `json:"sequence"`
	TurnID       string    `json:"turn_id"`
	Preview      string    `json:"preview"`
	ReplyPreview string    `json:"reply_preview"`
	At           time.Time `json:"at"`
}

// OutlineResult contains the lightweight operator-message trail over retained history.
type OutlineResult struct {
	Entries []OutlineEntry `json:"entries"`
}

// NavigationReader queries the existing projection without loading a UI page window.
type NavigationReader interface {
	TranscriptSearch(context.Context, SearchQuery) (SearchResult, error)
	TranscriptOutline(context.Context) (OutlineResult, error)
}
