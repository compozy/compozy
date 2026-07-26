package sessiondb

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/store"
)

const (
	sessionEventColumns         = "id, sequence, turn_id, type, agent_name, content, archived, timestamp"
	sessionEventMetadataColumns = "id, sequence, turn_id, type, agent_name, timestamp"
)

// dynamic-sql: selected projection, optional event predicates, cursor bounds, and reverse-limit paging change shape.
func buildEventQuerySQL(columns string, query store.EventQuery) (string, []any, error) {
	projection := strings.TrimSpace(columns)
	if projection == "" {
		return "", nil, fmt.Errorf("store: session event query projection is required")
	}
	if err := query.Validate(); err != nil {
		return "", nil, err
	}

	baseQuery := "SELECT " + projection + " FROM events"
	where, args := store.BuildClauses(
		store.StringClause("type", query.Type),
		store.StringClause("agent_name", query.AgentName),
		store.StringClause("turn_id", query.TurnID),
		store.TimeClause("timestamp", ">=", query.Since),
		store.Int64Clause("sequence", ">", query.AfterSequence),
		store.Int64Clause("sequence", "<", query.BeforeSequence),
	)
	switch query.Archive {
	case store.EventArchiveUnarchived:
		where = append(where, "archived = ?")
		args = append(args, 0)
	case store.EventArchiveArchived:
		where = append(where, "archived = ?")
		args = append(args, 1)
	}
	baseQuery = store.AppendWhere(baseQuery, where)
	if query.Limit <= 0 {
		return baseQuery + sessionEventsOrderASCClause, args, nil
	}

	return "SELECT " + projection +
			" FROM (" + baseQuery + " ORDER BY sequence DESC LIMIT ?) AS recent_events" +
			" ORDER BY sequence ASC",
		append(args, query.Limit),
		nil
}
