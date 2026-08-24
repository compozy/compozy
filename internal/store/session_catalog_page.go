package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SessionCatalogAttentionRank is the stable attention-class component of a catalog cursor.
type SessionCatalogAttentionRank int

const (
	SessionCatalogAttentionRankNeedsYou SessionCatalogAttentionRank = iota
	SessionCatalogAttentionRankFinished
	SessionCatalogAttentionRankNone
)

// SessionCatalogPosition is the stable keyset anchor used by bounded catalog reads.
type SessionCatalogPosition struct {
	AttentionRank SessionCatalogAttentionRank `json:"attention_rank"`
	PrimaryAt     time.Time                   `json:"primary_at"`
	SecondaryAt   time.Time                   `json:"secondary_at"`
	CreatedAt     time.Time                   `json:"created_at"`
	ID            string                      `json:"id"`
}

// Validate ensures the keyset anchor is complete.
func (p SessionCatalogPosition) Validate() error {
	if p.AttentionRank < SessionCatalogAttentionRankNeedsYou ||
		p.AttentionRank > SessionCatalogAttentionRankNone {
		return fmt.Errorf("store: session catalog attention rank is invalid: %d", p.AttentionRank)
	}
	if p.PrimaryAt.IsZero() || p.SecondaryAt.IsZero() || p.CreatedAt.IsZero() || strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("store: session catalog position is incomplete")
	}
	return nil
}

// SessionCatalogPageQuery describes one bounded durable-catalog read through one
// explicit profile or the AllProfiles aggregate. Cursor decoding and
// active-session overlay belong to the session manager.
type SessionCatalogPageQuery struct {
	ReadScope           ReadScope
	WorkspaceID         string
	WorktreeID          string
	State               string
	SessionType         string
	AgentName           string
	ParentSessionID     string
	RootSessionID       string
	Search              string
	Resumable           bool
	Archive             SessionArchiveFilter
	Sort                string
	Limit               int
	After               *SessionCatalogPosition
	ExcludeIDs          []string
	ExcludeSessionTypes []string
	ExcludeSpawnRoles   []string
}

// Validate ensures the durable query is bounded and its anchor is usable.
func (q SessionCatalogPageQuery) Validate() error {
	if err := q.ReadScope.Validate(); err != nil {
		return err
	}
	if q.Limit <= 0 {
		return fmt.Errorf("store: session catalog page limit must be positive")
	}
	if err := q.Archive.Validate(); err != nil {
		return err
	}
	if q.After != nil {
		if err := q.After.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SessionCatalogPage is one bounded durable-catalog result. Total is counted
// before the keyset anchor and excludes IDs owned by the active overlay.
type SessionCatalogPage struct {
	Sessions []SessionInfo
	Total    int
}

// SessionCatalogPager exposes the bounded catalog read used by public session lists.
type SessionCatalogPager interface {
	PageSessions(ctx context.Context, query SessionCatalogPageQuery) (SessionCatalogPage, error)
}
