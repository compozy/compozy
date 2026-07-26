package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"

	"strconv"
	"strings"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/gin-gonic/gin"
)

// ClearSessionConversation clears persisted conversation history and restarts the
// session with a fresh ACP conversation context while preserving the same id.
func (h *BaseHandlers) ClearSessionConversation(c *gin.Context) {
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	sess, err := h.Sessions.ClearConversation(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SessionResponse{Session: SessionPayloadFromInfo(sess.Info())})
}

// SessionEvents returns the filtered session event list.
func (h *BaseHandlers) SessionEvents(c *gin.Context) {
	if err := rejectTranscriptBackwardCursor(c); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err := ParseSessionEventQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err = applyBoundedSessionReadDefault(query)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	if lastEventID := strings.TrimSpace(c.GetHeader("Last-Event-ID")); lastEventID != "" {
		query.AfterSequence, err = parseLastEventID(lastEventID, h.transportName())
		if err != nil {
			h.respondError(c, http.StatusBadRequest, err)
			return
		}
	}

	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	if !h.IncludeSessionWorkspaceInSSE {
		info = nil
	}

	events, err := h.Sessions.Events(c.Request.Context(), sessionID, query)
	if err != nil {
		h.logSessionReadError(
			"events",
			sessionID,
			err,
			"after_sequence",
			query.AfterSequence,
			"type",
			query.Type,
			"turn_id",
			query.TurnID,
		)
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	payload := make([]contract.SessionEventPayload, 0, len(events))
	for _, event := range events {
		payload = append(payload, SessionEventPayloadFromEvent(event, info))
	}

	c.JSON(http.StatusOK, contract.SessionEventsResponse{Events: payload})
}

// SessionHistory returns the grouped turn history for a session.
func (h *BaseHandlers) SessionHistory(c *gin.Context) {
	if err := rejectTranscriptBackwardCursor(c); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err := ParseSessionEventQuery(c)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	query, err = applyBoundedSessionReadDefault(query)
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	if !h.IncludeSessionWorkspaceInSSE {
		info = nil
	}

	history, err := h.Sessions.History(c.Request.Context(), sessionID, query)
	if err != nil {
		h.logSessionReadError("history", sessionID, err)
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	payload := make([]contract.TurnHistoryPayload, 0, len(history))
	for _, turn := range history {
		events := make([]contract.SessionEventPayload, 0, len(turn.Events))
		for _, event := range turn.Events {
			events = append(events, SessionEventPayloadFromEvent(event, info))
		}
		payload = append(payload, contract.TurnHistoryPayload{
			TurnID: turn.TurnID,
			Events: events,
		})
	}

	c.JSON(http.StatusOK, contract.SessionHistoryResponse{History: payload})
}

func repairBoolQuery(c *gin.Context, names ...string) (bool, error) {
	var (
		value bool
		seen  bool
	)
	for _, name := range names {
		raw, ok := c.GetQuery(name)
		if !ok {
			continue
		}
		parsed, err := ParseOptionalBool(raw)
		if err != nil {
			return false, fmt.Errorf("invalid %s query: %w", name, err)
		}
		if seen && parsed != value {
			return false, fmt.Errorf(
				"conflicting boolean query values for %s",
				strings.Join(names, ", "),
			)
		}
		value = parsed
		seen = true
	}
	return value, nil
}

func parseOptionalPositiveIntQuery(c *gin.Context, name string, fallback int, maxValue int) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s query: %w", name, err)
	}
	if value < 0 {
		return 0, fmt.Errorf("invalid %s query: must be zero or positive", name)
	}
	if maxValue > 0 && value > maxValue {
		return 0, fmt.Errorf("invalid %s query: must be <= %d", name, maxValue)
	}
	return value, nil
}

func decodeStrictCreateAgentRequest(c *gin.Context, req *contract.CreateAgentRequest) error {
	return decodeStrictJSONBody(c, req)
}

func decodeStrictJSONBody(c *gin.Context, target any) error {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return normalizeStrictJSONDecodeError(err)
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != nil && !errors.Is(err, io.EOF) {
		return normalizeStrictJSONDecodeError(err)
	} else if err == nil {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
