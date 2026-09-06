package sessiondb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

var (
	_ transcript.NavigationReader = (*SessionDB)(nil)
	_ transcript.NavigationReader = (*ReadOnlySessionDB)(nil)
)

// TranscriptSearch reads matching projected messages from the whole retained history.
func (s *SessionDB) TranscriptSearch(
	ctx context.Context,
	query transcript.SearchQuery,
) (transcript.SearchResult, error) {
	if s == nil || s.db == nil {
		return transcript.SearchResult{}, errors.New("store: session database is required")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return transcript.SearchResult{}, store.ErrClosed
	}
	return queryTranscriptSearch(ctx, s.db, query)
}

// TranscriptOutline reads the operator-message trail from the retained projection.
func (s *SessionDB) TranscriptOutline(ctx context.Context) (transcript.OutlineResult, error) {
	if s == nil || s.db == nil {
		return transcript.OutlineResult{}, errors.New("store: session database is required")
	}
	s.acceptMu.RLock()
	defer s.acceptMu.RUnlock()
	if s.state.Load() != sessionStateOpen {
		return transcript.OutlineResult{}, store.ErrClosed
	}
	return queryTranscriptOutline(ctx, s.db)
}

// TranscriptSearch reads matching messages without a live writer.
func (s *ReadOnlySessionDB) TranscriptSearch(
	ctx context.Context,
	query transcript.SearchQuery,
) (transcript.SearchResult, error) {
	if s == nil || s.db == nil {
		return transcript.SearchResult{}, errors.New("store: read-only session database is required")
	}
	return queryTranscriptSearch(ctx, s.db, query)
}

// TranscriptOutline reads the operator-message trail without a live writer.
func (s *ReadOnlySessionDB) TranscriptOutline(ctx context.Context) (transcript.OutlineResult, error) {
	if s == nil || s.db == nil {
		return transcript.OutlineResult{}, errors.New("store: read-only session database is required")
	}
	return queryTranscriptOutline(ctx, s.db)
}

func queryTranscriptSearch(
	ctx context.Context,
	db *sql.DB,
	query transcript.SearchQuery,
) (transcript.SearchResult, error) {
	query, err := query.Normalize()
	if err != nil {
		return transcript.SearchResult{}, err
	}
	result := transcript.SearchResult{Matches: make([]transcript.SearchMatch, 0, query.Limit)}
	err = readTranscriptNavigation(ctx, db, func(queries *sqlcgen.Queries) error {
		var cursor int64
		for {
			rows, readErr := queries.ListTranscriptSearchCandidates(ctx, sqlcgen.ListTranscriptSearchCandidatesParams{
				AfterSequence: cursor, RowLimit: 200,
			})
			if readErr != nil {
				return readErr
			}
			for _, row := range rows {
				message, decodeErr := decodeNavigationMessage(row.MessageJson)
				if decodeErr != nil {
					return decodeErr
				}
				if match, found := navigationMatch(message, query.Query); found {
					if len(result.Matches) == query.Limit {
						result.Truncated = true
						return nil
					}
					match.Sequence, match.TurnID, match.Role = row.StartSequence, row.TurnID, message.Role
					result.Matches = append(result.Matches, match)
				}
				cursor = row.StartSequence
			}
			if len(rows) < 200 {
				return nil
			}
		}
	})
	if err != nil {
		return transcript.SearchResult{}, err
	}
	return result, nil
}

func queryTranscriptOutline(ctx context.Context, db *sql.DB) (transcript.OutlineResult, error) {
	result := transcript.OutlineResult{Entries: []transcript.OutlineEntry{}}
	err := readTranscriptNavigation(ctx, db, func(queries *sqlcgen.Queries) error {
		var cursor int64
		for {
			rows, readErr := queries.ListTranscriptOutlineCandidates(ctx, sqlcgen.ListTranscriptOutlineCandidatesParams{
				AfterSequence: cursor, RowLimit: 200,
			})
			if readErr != nil {
				return readErr
			}
			for _, row := range rows {
				entry, mapErr := navigationOutlineEntry(row)
				if mapErr != nil {
					return mapErr
				}
				result.Entries = append(result.Entries, entry)
				cursor = row.StartSequence
			}
			if len(rows) < 200 {
				return nil
			}
		}
	})
	if err != nil {
		return transcript.OutlineResult{}, err
	}
	return result, nil
}

func navigationOutlineEntry(row sqlcgen.ListTranscriptOutlineCandidatesRow) (transcript.OutlineEntry, error) {
	message, err := decodeNavigationMessage(row.MessageJson)
	if err != nil {
		return transcript.OutlineEntry{}, err
	}
	entry := transcript.OutlineEntry{
		Sequence: row.StartSequence, TurnID: row.TurnID,
		Preview: navigationPreview(transcript.UIMessageText(message), 160),
	}
	if row.ReplyJson.Valid {
		reply, decodeErr := decodeNavigationMessage(row.ReplyJson)
		if decodeErr != nil {
			return transcript.OutlineEntry{}, decodeErr
		}
		entry.ReplyPreview = navigationPreview(transcript.UIMessageText(reply), 160)
	}
	entry.At, err = time.Parse(time.RFC3339Nano, row.At)
	if err != nil {
		return transcript.OutlineEntry{}, fmt.Errorf("%w: missing or invalid outline timestamp at %d: %v",
			transcript.ErrProjectionCorrupt, row.StartSequence, err)
	}
	return entry, nil
}

func readTranscriptNavigation(ctx context.Context, db *sql.DB, read func(*sqlcgen.Queries) error) (err error) {
	if ctx == nil {
		return errors.New("store: transcript navigation context is required")
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("store: begin transcript navigation: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			joinSessionCleanupError(&err, fmt.Errorf("store: close transcript navigation: %w", rollbackErr))
		}
	}()
	if _, err := loadProjectionState(ctx, tx); err != nil {
		return err
	}
	if err := read(sqlcgen.New(tx)); err != nil {
		return fmt.Errorf("store: read transcript navigation: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: commit transcript navigation: %w", err)
	}
	return nil
}

func decodeNavigationMessage(raw sql.NullString) (transcript.UIMessage, error) {
	var message transcript.UIMessage
	if !raw.Valid {
		return message, fmt.Errorf("%w: navigation message is absent", transcript.ErrProjectionCorrupt)
	}
	if err := json.Unmarshal([]byte(raw.String), &message); err != nil {
		return message, fmt.Errorf("%w: decode navigation message: %v", transcript.ErrProjectionCorrupt, err)
	}
	return message, nil
}

func navigationMatch(message transcript.UIMessage, query string) (transcript.SearchMatch, bool) {
	for index, part := range message.Parts {
		for _, field := range []struct{ name, text string }{
			{"text", part.Text}, {sessionEventPayloadTitleKey, part.Title}, {"tool_name", part.ToolName},
			{"filename", part.Filename},
			{"error", part.ErrorText}, {"input", string(part.Input)}, {"output", string(part.Output)},
		} {
			text := field.text
			lower := strings.ToLower(text)
			at := strings.Index(lower, strings.ToLower(query))
			if at < 0 {
				continue
			}
			// Count runes in the lowercased prefix: case mapping can change UTF-8 byte lengths.
			start := max(0, len([]rune(lower[:at]))-60)
			runes := []rune(text)
			snippet := navigationPreview(string(runes[start:]), max(200, len([]rune(query))+60))
			if start > 0 {
				snippet = "…" + snippet
			}
			return transcript.SearchMatch{Snippet: snippet, PartIndex: &index, Field: field.name}, true
		}
	}
	return transcript.SearchMatch{}, false
}

func navigationPreview(text string, limit int) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= limit {
		return string(runes)
	}
	return string(runes[:limit]) + "…"
}
