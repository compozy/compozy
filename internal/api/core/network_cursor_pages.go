package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/store"
)

func cursorOverfetchLimit(limit int) int {
	if limit <= 0 || limit == int(^uint(0)>>1) {
		return limit
	}
	return limit + 1
}

func trimNetworkMessagePayloadPage(
	messages []contract.NetworkConversationMessagePayload,
	after string,
	limit int,
) ([]contract.NetworkConversationMessagePayload, contract.CursorPagePayload) {
	if limit <= 0 || len(messages) <= limit {
		return messages, contract.CursorPagePayload{Limit: limit}
	}
	if strings.TrimSpace(after) != "" {
		messages = messages[:limit]
		return messages, contract.CursorPagePayload{
			NextCursor: strings.TrimSpace(messages[len(messages)-1].MessageID),
			HasMore:    true,
			Limit:      limit,
		}
	}
	messages = messages[len(messages)-limit:]
	return messages, contract.CursorPagePayload{
		NextCursor: strings.TrimSpace(messages[0].MessageID),
		HasMore:    true,
		Limit:      limit,
	}
}

func networkThreadPagePayload(page store.NetworkThreadPage) contract.CountedCursorPagePayload {
	return contract.CountedCursorPagePayload{
		NextCursor: strings.TrimSpace(page.NextCursor),
		HasMore:    page.HasMore,
		Total:      page.Total,
		Limit:      page.Limit,
	}
}

func networkDirectRoomPagePayload(page store.NetworkDirectRoomPage) contract.CountedCursorPagePayload {
	return contract.CountedCursorPagePayload{
		NextCursor: strings.TrimSpace(page.NextCursor),
		HasMore:    page.HasMore,
		Total:      page.Total,
		Limit:      page.Limit,
	}
}

func overfetchNetworkMessageQuery(query store.NetworkMessageQuery) store.NetworkMessageQuery {
	query.Limit = cursorOverfetchLimit(query.Limit)
	return query
}

func overfetchNetworkConversationMessageQuery(
	query store.NetworkConversationMessageQuery,
) store.NetworkConversationMessageQuery {
	query.Limit = cursorOverfetchLimit(query.Limit)
	return query
}
