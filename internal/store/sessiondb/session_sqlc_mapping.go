package sessiondb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

type rowScanner interface {
	Scan(dest ...any) error
}

func boolToSQLite(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func sessionNullableInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *value, Valid: true}
}

func sessionNullableFloat64(value *float64) sql.NullFloat64 {
	if value == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *value, Valid: true}
}

func sessionNullableString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func sessionNullableStringPointer(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sessionNullableString(*value)
}

func sessionEventFromSQLC(
	id string,
	sequence int64,
	turnID string,
	eventType string,
	agentName string,
	content string,
	archived int64,
	timestampRaw string,
	sessionID string,
) (store.SessionEvent, error) {
	timestamp, err := store.ParseTimestamp(timestampRaw)
	if err != nil {
		timestamp, err = time.Parse(time.RFC3339Nano, timestampRaw)
		if err != nil {
			return store.SessionEvent{}, fmt.Errorf("store: parse session event timestamp %q: %w", timestampRaw, err)
		}
	}
	return store.SessionEvent{
		ID:        id,
		SessionID: sessionID,
		Sequence:  sequence,
		TurnID:    turnID,
		Type:      eventType,
		AgentName: agentName,
		Content:   content,
		Archived:  archived != 0,
		Timestamp: timestamp,
	}, nil
}
