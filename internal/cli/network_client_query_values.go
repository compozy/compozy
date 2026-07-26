package cli

import (
	"net/url"
	"strconv"
	"strings"
)

func networkThreadsValues(query NetworkThreadsQuery) url.Values {
	return networkConversationListValues(
		query.Limit,
		query.After,
		query.Query,
		query.SessionID,
		query.Sort,
		query.HasWork,
	)
}

func networkDirectsValues(query NetworkDirectsQuery) url.Values {
	return networkConversationListValues(
		query.Limit,
		query.After,
		query.Query,
		query.SessionID,
		query.Sort,
		query.HasWork,
	)
}

func networkConversationListValues(
	limit int,
	after string,
	search string,
	sessionID string,
	sortOrder string,
	hasWork *bool,
) url.Values {
	values := networkListValues(limit, after)
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		values.Set("query", trimmed)
	}
	if trimmed := strings.TrimSpace(sessionID); trimmed != "" {
		values.Set("session_id", trimmed)
	}
	if trimmed := strings.TrimSpace(sortOrder); trimmed != "" {
		values.Set("sort", trimmed)
	}
	if hasWork != nil {
		values.Set("has_work", strconv.FormatBool(*hasWork))
	}
	return values
}
