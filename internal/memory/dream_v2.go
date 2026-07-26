package memory

import (
	"context"
	"database/sql"

	"errors"
	"fmt"
	"math"

	"sort"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	dreamV2EFilenamePath    = "  e.filename,"
	dreamV2ENamePath        = "  e.name,"
	dreamV2EScopePath       = "  e.scope,"
	dreamV2ETypePath        = "  e.type,"
	dreamV2EWorkspaceIDPath = "  e.workspace_id,"
	dreamV2SelectValue      = "SELECT"
	dreamErrorFieldKey      = "error"
	dreamV2PromptVersionKey = "prompt_version"
	dreamV2WorkspaceIDKey   = "workspace_id"
)

const (
	defaultDreamMinSignals      = 5
	defaultDreamMinRecallCount  = 2
	defaultDreamPromotionScore  = 0.75
	defaultDreamCandidateLimit  = 20
	defaultDreamHalfLife        = 14 * 24 * time.Hour
	defaultDreamFrequencyWeight = 0.30
	defaultDreamRelevanceWeight = 0.35
	defaultDreamRecencyWeight   = 0.20
	defaultDreamFreshnessWeight = 0.15
	dreamPromptVersion          = "dream.v1"
)

var (
	// ErrDreamGateNotSatisfied reports a lock-protected dream run skipped
	// because recall-signal promotion thresholds were not met.
	ErrDreamGateNotSatisfied = errors.New("memory: dream signal gate is not satisfied")
)

// DreamGateConfig controls the recall-signal gate that precedes dreaming.
type DreamGateConfig struct {
	MinCandidates   int
	MinRecallCount  int
	MinScore        float64
	CandidateLimit  int
	HalfLife        time.Duration
	FrequencyWeight float64
	RelevanceWeight float64
	RecencyWeight   float64
	FreshnessWeight float64
}

// DreamCandidate is one unpromoted recall-signal candidate eligible for dreaming.
type DreamCandidate struct {
	ChunkID            string
	EntryID            string
	WorkspaceID        string
	Scope              memcontract.Scope
	AgentName          string
	AgentTier          memcontract.AgentTier
	Type               memcontract.Type
	Slug               string
	Filename           string
	Title              string
	Body               string
	RecallCount        int
	SessionCount       int
	RecallScore        float64
	LastRecalledAt     time.Time
	FreshnessStartedAt time.Time
	Score              float64
}

type dreamRunWorkspace struct {
	id    string
	store *Store
	scope memcontract.Scope
}

type dreamSignalGateResult struct {
	active     bool
	runID      string
	candidates []DreamCandidate
	reason     string
}

func defaultDreamGateConfig() DreamGateConfig {
	return DreamGateConfig{
		MinCandidates:   defaultDreamMinSignals,
		MinRecallCount:  defaultDreamMinRecallCount,
		MinScore:        defaultDreamPromotionScore,
		CandidateLimit:  defaultDreamCandidateLimit,
		HalfLife:        defaultDreamHalfLife,
		FrequencyWeight: defaultDreamFrequencyWeight,
		RelevanceWeight: defaultDreamRelevanceWeight,
		RecencyWeight:   defaultDreamRecencyWeight,
		FreshnessWeight: defaultDreamFreshnessWeight,
	}
}

func normalizeDreamGateConfig(config DreamGateConfig) DreamGateConfig {
	defaults := defaultDreamGateConfig()
	if config.MinCandidates <= 0 {
		config.MinCandidates = defaults.MinCandidates
	}
	if config.MinRecallCount <= 0 {
		config.MinRecallCount = defaults.MinRecallCount
	}
	if config.MinScore <= 0 {
		config.MinScore = defaults.MinScore
	}
	if config.CandidateLimit <= 0 {
		config.CandidateLimit = defaults.CandidateLimit
	}
	if config.HalfLife <= 0 {
		config.HalfLife = defaults.HalfLife
	}
	if config.FrequencyWeight <= 0 {
		config.FrequencyWeight = defaults.FrequencyWeight
	}
	if config.RelevanceWeight <= 0 {
		config.RelevanceWeight = defaults.RelevanceWeight
	}
	if config.RecencyWeight <= 0 {
		config.RecencyWeight = defaults.RecencyWeight
	}
	if config.FreshnessWeight <= 0 {
		config.FreshnessWeight = defaults.FreshnessWeight
	}
	return config
}

func (s *Store) dreamCandidates(
	ctx context.Context,
	workspaceID string,
	config DreamGateConfig,
	now time.Time,
) ([]DreamCandidate, error) {
	if s == nil || s.catalog == nil {
		return nil, nil
	}
	config = normalizeDreamGateConfig(config)
	db, err := s.catalog.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, nil
	}

	rows, err := queryDreamCandidateRows(ctx, db, workspaceID, config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			s.warn("memory: close dream candidate rows failed", dreamErrorFieldKey, closeErr)
		}
	}()

	candidates := make([]DreamCandidate, 0, config.CandidateLimit)
	for rows.Next() {
		candidate, scanErr := scanDreamCandidate(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		candidate.Score = dreamPromotionScore(candidate, config, now)
		if candidate.Score < config.MinScore {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate dream candidates: %w", err)
	}
	sortDreamCandidates(candidates)
	if len(candidates) > config.CandidateLimit {
		candidates = candidates[:config.CandidateLimit]
	}
	return candidates, nil
}

func queryDreamCandidateRows(
	ctx context.Context,
	db *sql.DB,
	workspaceID string,
	config DreamGateConfig,
) (*sql.Rows, error) {
	base := strings.Join([]string{
		dreamV2SelectValue,
		`  sig.chunk_id,`,
		`  c.file_id,`,
		dreamV2EWorkspaceIDPath,
		dreamV2EScopePath,
		`  e.agent_name,`,
		`  e.agent_tier,`,
		dreamV2ETypePath,
		`  e.slug,`,
		dreamV2EFilenamePath,
		dreamV2ENamePath,
		`  c.content,`,
		`  sig.recall_count,`,
		`  sig.session_count,`,
		`  sig.recall_score,`,
		`  sig.last_recalled_at,`,
		`  sig.freshness_started_at`,
		`FROM memory_recall_signals sig`,
		`JOIN memory_chunks c ON c.id = sig.chunk_id`,
		`JOIN memory_catalog_entries e ON e.id = c.file_id`,
		`WHERE sig.promoted_at IS NULL`,
		`  AND sig.recall_count >= ?`,
		`  AND e.injection = 1`,
	}, "\n")
	args := []any{config.MinRecallCount}
	base, args = appendDreamVisibilityFilter(base, args, workspaceID)
	base += "\nORDER BY sig.recall_score DESC, sig.last_recalled_at DESC, sig.chunk_id ASC\nLIMIT ?"
	args = append(args, max(config.CandidateLimit*4, config.CandidateLimit))

	rows, err := db.QueryContext(ctx, base, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: query dream candidates: %w", err)
	}
	return rows, nil
}

func appendDreamVisibilityFilter(base string, args []any, workspaceID string) (string, []any) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return base + "\n  AND e.scope = 'global'", args
	}
	return base + "\n  AND (e.scope = 'global' OR (e.scope IN ('workspace', 'agent') AND e.workspace_id = ?))",
		append(args, workspaceID)
}

func scanDreamCandidate(scanner interface{ Scan(dest ...any) error }) (DreamCandidate, error) {
	var (
		candidate    DreamCandidate
		scopeRaw     string
		agentTierRaw string
		typeRaw      string
		lastRecall   sql.NullInt64
		freshness    int64
	)
	if err := scanner.Scan(
		&candidate.ChunkID,
		&candidate.EntryID,
		&candidate.WorkspaceID,
		&scopeRaw,
		&candidate.AgentName,
		&agentTierRaw,
		&typeRaw,
		&candidate.Slug,
		&candidate.Filename,
		&candidate.Title,
		&candidate.Body,
		&candidate.RecallCount,
		&candidate.SessionCount,
		&candidate.RecallScore,
		&lastRecall,
		&freshness,
	); err != nil {
		return DreamCandidate{}, fmt.Errorf("memory: scan dream candidate: %w", err)
	}
	candidate.Scope = memcontract.Scope(scopeRaw).Normalize()
	candidate.AgentTier = memcontract.AgentTier(agentTierRaw).Normalize()
	candidate.Type = memcontract.Type(typeRaw).Normalize()
	if lastRecall.Valid {
		candidate.LastRecalledAt = timeFromUnixMillis(lastRecall.Int64)
	}
	if freshness > 0 {
		candidate.FreshnessStartedAt = timeFromUnixMillis(freshness)
	}
	return candidate, nil
}

func sortDreamCandidates(candidates []DreamCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			if candidates[i].LastRecalledAt.Equal(candidates[j].LastRecalledAt) {
				return candidates[i].ChunkID < candidates[j].ChunkID
			}
			return candidates[i].LastRecalledAt.After(candidates[j].LastRecalledAt)
		}
		return candidates[i].Score > candidates[j].Score
	})
}

func dreamPromotionScore(candidate DreamCandidate, config DreamGateConfig, now time.Time) float64 {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	frequency := clamp01(float64(candidate.RecallCount) / float64(config.MinRecallCount))
	relevance := clamp01(candidate.RecallScore)
	recency := decayScore(candidate.LastRecalledAt, now, config.HalfLife)
	freshness := decayScore(candidate.FreshnessStartedAt, now, config.HalfLife)
	return (config.FrequencyWeight * frequency) +
		(config.RelevanceWeight * relevance) +
		(config.RecencyWeight * recency) +
		(config.FreshnessWeight * freshness)
}

func decayScore(timestamp time.Time, now time.Time, halfLife time.Duration) float64 {
	if timestamp.IsZero() {
		return 0
	}
	if halfLife <= 0 {
		halfLife = defaultDreamHalfLife
	}
	age := now.Sub(timestamp.UTC())
	if age <= 0 {
		return 1
	}
	return clamp01(math.Pow(0.5, age.Hours()/halfLife.Hours()))
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
