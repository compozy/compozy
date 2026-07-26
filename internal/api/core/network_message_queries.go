package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

func parseNetworkMessageQuery(c *gin.Context) (store.NetworkMessageQuery, error) {
	limit, err := ParseOptionalInt(c.Query("limit"))
	if err != nil {
		return store.NetworkMessageQuery{}, err
	}
	includePresence, err := ParseOptionalBool(c.Query("include_presence"))
	if err != nil {
		return store.NetworkMessageQuery{}, err
	}
	query := store.NetworkMessageQuery{
		BeforeMessageID: strings.TrimSpace(c.Query("before")),
		AfterMessageID:  strings.TrimSpace(c.Query("after")),
		IncludePresence: includePresence,
		Limit:           store.NormalizeNetworkMessageLimit(limit),
	}
	if strings.TrimSpace(query.BeforeMessageID) != "" && strings.TrimSpace(query.AfterMessageID) != "" {
		return store.NetworkMessageQuery{}, NewNetworkValidationError(
			fmt.Errorf(
				"%w: network message query cannot specify both before and after cursors",
				network.ErrInvalidField,
			),
		)
	}
	return query, nil
}

func (h *BaseHandlers) loadPublicChannelTimeline(
	ctx context.Context,
	networkStore NetworkStore,
	query store.NetworkMessageQuery,
) ([]store.NetworkMessageEntry, []store.NetworkMessageEntry, error) {
	if !query.IncludePresence {
		query.PublicOnly = true
		messages, err := networkStore.ListNetworkMessages(ctx, query)
		return messages, messages, err
	}
	rawMessages, err := listTimelineRawMessages(ctx, networkStore, query)
	if err != nil {
		return nil, nil, err
	}
	return rawMessages, filterVisiblePublicChannelMessages(rawMessages, query.IncludePresence), nil
}

func (h *BaseHandlers) loadVisiblePeerMessages(
	ctx context.Context,
	networkStore NetworkStore,
	query store.NetworkMessageQuery,
) ([]store.NetworkMessageEntry, error) {
	if !query.IncludePresence {
		query.DirectedOnly = true
		return networkStore.ListNetworkMessages(ctx, query)
	}
	rawMessages, err := listTimelineRawMessages(ctx, networkStore, query)
	if err != nil {
		return nil, err
	}
	return filterVisiblePeerMessages(rawMessages, query.IncludePresence), nil
}

func networkTimelinePayloadQuery(query store.NetworkMessageQuery) store.NetworkMessageQuery {
	if query.IncludePresence {
		return query
	}
	query.BeforeMessageID = ""
	query.AfterMessageID = ""
	query.Limit = 0
	return query
}

func listTimelineRawMessages(
	ctx context.Context,
	networkStore NetworkStore,
	query store.NetworkMessageQuery,
) ([]store.NetworkMessageEntry, error) {
	rawQuery := query
	rawQuery.Limit = 0
	rawQuery.BeforeMessageID = ""
	rawQuery.AfterMessageID = ""
	return networkStore.ListNetworkMessages(ctx, rawQuery)
}

func (h *BaseHandlers) respondNetworkMessageError(c *gin.Context, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		h.respondError(c, http.StatusBadRequest, NewNetworkValidationError(errors.New("message cursor not found")))
		return
	}
	h.respondError(c, http.StatusInternalServerError, err)
}
