package globaldb

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func populateSessionLaterScanParts(session *store.SessionInfo, row *sessionInfoRow) error {
	returnedAt, err := parseOptionalLaterTimestamp(row.laterReturnedAt, "later returned at")
	if err != nil {
		return err
	}
	if session.Attention == nil {
		session.Attention = &store.SessionAttention{}
	}
	session.Attention.LaterReturnedAt = returnedAt
	parkedAt, err := parseOptionalLaterTimestamp(row.parkedAt, "parked at")
	if err != nil || parkedAt == nil {
		return err
	}
	returnAt, err := parseOptionalLaterTimestamp(row.parkUntil, "park until")
	if err != nil {
		return err
	}
	session.Attention.Later = &store.SessionLater{
		ParkedAt:        *parkedAt,
		Reason:          strings.TrimSpace(row.parkReason),
		ReturnAt:        returnAt,
		PendingMessages: row.laterPendingMessages,
	}
	return nil
}

func parseOptionalLaterTimestamp(value sql.NullString, field string) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := store.ParseTimestamp(value.String)
	if err != nil {
		return nil, fmt.Errorf("store: parse session %s: %w", field, err)
	}
	return &parsed, nil
}
