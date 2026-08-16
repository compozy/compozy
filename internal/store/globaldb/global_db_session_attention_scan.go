package globaldb

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func populateSessionAttentionScanParts(session *store.SessionInfo, row *sessionInfoRow) error {
	session.PendingPermissionCount = row.pendingPermissionCount
	session.PendingClarifyCount = row.pendingClarifyCount
	session.AttentionRevision = row.attentionRevision
	session.LastSettledRevision = row.lastSettledRevision
	session.LastSeenRevision = row.lastSeenRevision
	if row.lastSeenAt.Valid && strings.TrimSpace(row.lastSeenAt.String) != "" {
		lastSeenAt, err := store.ParseTimestamp(row.lastSeenAt.String)
		if err != nil {
			return fmt.Errorf("store: parse session last seen at: %w", err)
		}
		session.LastSeenAt = &lastSeenAt
	}
	if row.attentionChangedAt.Valid && strings.TrimSpace(row.attentionChangedAt.String) != "" {
		attentionChangedAt, err := store.ParseTimestamp(row.attentionChangedAt.String)
		if err != nil {
			return fmt.Errorf("store: parse session attention changed at: %w", err)
		}
		session.AttentionChangedAt = &attentionChangedAt
	}
	return nil
}
