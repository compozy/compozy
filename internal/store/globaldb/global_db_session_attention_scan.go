package globaldb

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func populateSessionAttentionScanParts(session *store.SessionInfo, row *sessionInfoRow) error {
	attention := &store.SessionAttention{
		PendingPermissionCount: row.pendingPermissionCount,
		PendingClarifyCount:    row.pendingClarifyCount,
		AttentionRevision:      row.attentionRevision,
		LastSettledRevision:    row.lastSettledRevision,
		LastSeenRevision:       row.lastSeenRevision,
	}
	if row.lastSeenAt.Valid && strings.TrimSpace(row.lastSeenAt.String) != "" {
		lastSeenAt, err := store.ParseTimestamp(row.lastSeenAt.String)
		if err != nil {
			return fmt.Errorf("store: parse session last seen at: %w", err)
		}
		attention.LastSeenAt = &lastSeenAt
	}
	if row.attentionChangedAt.Valid && strings.TrimSpace(row.attentionChangedAt.String) != "" {
		attentionChangedAt, err := store.ParseTimestamp(row.attentionChangedAt.String)
		if err != nil {
			return fmt.Errorf("store: parse session attention changed at: %w", err)
		}
		attention.AttentionChangedAt = &attentionChangedAt
	}
	session.Attention = attention
	return nil
}
