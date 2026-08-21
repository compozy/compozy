package cmdpalette

import (
	"context"
	"time"
)

// Weights is the versioned scorer configuration served to attached clients.
type Weights struct {
	Version                    int      `json:"version"`
	FrecencyHalfLifeDays       int      `json:"frecency_half_life_days"`
	QueryHalfLifeDays          int      `json:"query_half_life_days"`
	Deadband                   float64  `json:"deadband"`
	FallbackWeakMatchThreshold float64  `json:"fallback_weak_match_threshold"`
	MatchAliasExact            float64  `json:"match_alias_exact"`
	MatchExact                 float64  `json:"match_exact"`
	MatchPrefix                float64  `json:"match_prefix"`
	MatchTokenPrefix           float64  `json:"match_token_prefix"`
	MatchCompactPrefix         float64  `json:"match_compact_prefix"`
	MatchWordBoundaryMin       float64  `json:"match_word_boundary_min"`
	MatchWordBoundaryMax       float64  `json:"match_word_boundary_max"`
	MatchContains              float64  `json:"match_contains"`
	MatchSubsequenceMin        float64  `json:"match_subsequence_min"`
	MatchSubsequenceMax        float64  `json:"match_subsequence_max"`
	SecondaryFieldMultiplier   float64  `json:"secondary_field_multiplier"`
	SecondaryFieldCap          float64  `json:"secondary_field_cap"`
	FrecencyScale              float64  `json:"frecency_scale"`
	FrecencyCap                float64  `json:"frecency_cap"`
	QueryLearningCap           float64  `json:"query_learning_cap"`
	ContextBoost               float64  `json:"context_boost"`
	PromotionCommandFloor      float64  `json:"promotion_command_floor"`
	PromotionPathFloor         float64  `json:"promotion_path_floor"`
	PromotionDefaultFloor      float64  `json:"promotion_default_floor"`
	PromotionTabFloor          float64  `json:"promotion_tab_floor"`
	GhostMinScore              float64  `json:"ghost_min_score"`
	MinEntityQueryLength       int      `json:"min_entity_query_length"`
	MaxQueryLength             int      `json:"max_query_length"`
	PruneAfterDays             int      `json:"prune_after_days"`
	PruneThreshold             float64  `json:"prune_threshold"`
	EntitySectionVisibleCap    int      `json:"entity_section_visible_cap"`
	DomainViewMountCap         int      `json:"domain_view_mount_cap"`
	GroupOrder                 []string `json:"group_order"`
}

var WeightsV1 = Weights{
	Version:                    1,
	FrecencyHalfLifeDays:       30,
	QueryHalfLifeDays:          14,
	Deadband:                   12,
	FallbackWeakMatchThreshold: 120,
	MatchAliasExact:            1_040,
	MatchExact:                 1_000,
	MatchPrefix:                900,
	MatchTokenPrefix:           780,
	MatchCompactPrefix:         740,
	MatchWordBoundaryMin:       620,
	MatchWordBoundaryMax:       730,
	MatchContains:              560,
	MatchSubsequenceMin:        380,
	MatchSubsequenceMax:        520,
	SecondaryFieldMultiplier:   0.72,
	SecondaryFieldCap:          720,
	FrecencyScale:              70,
	FrecencyCap:                180,
	QueryLearningCap:           260,
	ContextBoost:               80,
	PromotionCommandFloor:      500,
	PromotionPathFloor:         600,
	PromotionDefaultFloor:      700,
	PromotionTabFloor:          820,
	GhostMinScore:              900,
	MinEntityQueryLength:       2,
	MaxQueryLength:             256,
	PruneAfterDays:             120,
	PruneThreshold:             0.05,
	EntitySectionVisibleCap:    6,
	DomainViewMountCap:         150,
	GroupOrder: []string{
		"Pinned", "Recents", "Curated", "Views", "Shell", "Window", "Tabs", "Tiling", "Layout",
		"Sessions", "Desktops", "Workspaces", "Agents", "Tasks", "Loops", "Jobs", "Triggers",
		"Bridges", "Knowledge", "Vault", "Network channels", "Marketplace", "Extensions", "Apps",
		"Settings", "Commands",
	},
}

type UsageSignal struct {
	CommandID  CommandID `json:"command_id"`
	UseCount   int64     `json:"-"`
	Weight     float64   `json:"weight"`
	LastUsedAt int64     `json:"last_used_at"`
}

type QueryHit struct {
	Query      string    `json:"query"`
	CommandID  CommandID `json:"command_id"`
	Weight     float64   `json:"weight"`
	LastUsedAt int64     `json:"-"`
}

type Pin struct {
	CommandID CommandID `json:"command_id"`
	PinnedAt  int64     `json:"pinned_at"`
}

type PersonalizationRows struct {
	Usage     []UsageSignal
	QueryHits []QueryHit
	Pins      []Pin
}

type Snapshot struct {
	Weights   Weights       `json:"weights"`
	Usage     []UsageSignal `json:"usage"`
	QueryHits []QueryHit    `json:"query_hits"`
	Pins      []Pin         `json:"pins"`
	Revision  string        `json:"revision"`
}

type PersonalizationSummary struct {
	Workspace         WorkspaceID `json:"workspace"`
	Pins              []CommandID `json:"pins"`
	Recents           int         `json:"recents"`
	FrecencyEntries   int         `json:"frecency_entries"`
	QueryAssociations int         `json:"query_associations"`
}

type PersonalizationStore interface {
	RecordCmdPaletteUsage(context.Context, Usage, Weights) error
	CmdPalettePersonalization(context.Context, WorkspaceID) (PersonalizationRows, error)
	PutCmdPalettePin(context.Context, WorkspaceID, CommandID, time.Time) error
	DeleteCmdPalettePin(context.Context, WorkspaceID, CommandID) error
	PruneCmdPaletteCommand(context.Context, WorkspaceID, CommandID) error
	PruneCmdPaletteUsage(context.Context, WorkspaceID, CommandID) error
	PruneCmdPaletteQueryHit(context.Context, WorkspaceID, string, CommandID) error
	ResetCmdPalettePersonalization(context.Context, WorkspaceID) error
}

// PersonalizationPolicy resolves whether workspace ranking signals are active.
type PersonalizationPolicy interface {
	PersonalizationEnabled(context.Context, WorkspaceID) (bool, error)
}
