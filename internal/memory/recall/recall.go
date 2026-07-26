package recall

import (
	"context"

	"errors"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

const (
	defaultTopK          = 5
	maxTopK              = 20
	defaultRawCandidates = 40
	maxRawCandidates     = 200
	recencyHalfLifeDays  = 14.0
	trivialTokenFloor    = 2
	nonASCIITokenFloor   = 3
)

var defaultWeights = Weights{
	Unicode: 0.55,
	Trigram: 0.20,
	Recency: 0.15,
	Signal:  0.10,
}

var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "by": {},
	"for": {}, "from": {}, "how": {}, "in": {}, "is": {}, "it": {}, "of": {}, "on": {},
	"or": {}, "the": {}, "to": {}, "what": {}, "when": {}, "where": {}, "with": {},
}

// Weights controls deterministic score fusion for Slice 1 recall.
type Weights struct {
	Unicode float64
	Trigram float64
	Recency float64
	Signal  float64
}

// Candidate is one catalog chunk candidate returned by the storage source.
type Candidate struct {
	ChunkID      string
	EntryID      string
	WorkspaceID  string
	Scope        memcontract.Scope
	AgentName    string
	AgentTier    memcontract.AgentTier
	Type         memcontract.Type
	Slug         string
	Filename     string
	Title        string
	Body         string
	ContentHash  string
	ModTime      time.Time
	Injection    bool
	UnicodeScore float64
	TrigramScore float64
	RecallScore  float64
}

// Signal records that one chunk was surfaced by recall.
type Signal struct {
	ChunkID      string
	WorkspaceID  string
	SurfaceID    string
	Score        float64
	SurfacedAt   time.Time
	SessionID    string
	SignalReason string
}

// Shadow records one candidate suppressed by a deeper scope owner.
type Shadow struct {
	WinnerChunkID string
	LoserChunkID  string
	WorkspaceID   string
	Scope         memcontract.Scope
	AgentName     string
	AgentTier     memcontract.AgentTier
	Type          memcontract.Type
	Slug          string
}

// Source supplies candidates and stores recall side effects.
type Source interface {
	Candidates(ctx context.Context, query memcontract.Query, opts memcontract.RecallOptions) ([]Candidate, error)
	RecordRecall(ctx context.Context, signals []Signal) error
	RecordRecallExecuted(ctx context.Context, query memcontract.Query, resultCount int) error
	RecordRecallSkipped(ctx context.Context, query memcontract.Query, reason string) error
	RecordRecallSignalFailed(ctx context.Context, query memcontract.Query, cause error) error
	RecordRecallSignalDropped(ctx context.Context, query memcontract.Query, signals []Signal, queueDepth int) error
	RecordShadow(ctx context.Context, shadow Shadow) error
}

// Recaller implements deterministic Slice 1 recall.
type Recaller struct {
	source         Source
	signalRecorder *SignalRecorder
	now            func() time.Time
	weights        Weights
	logger         *slog.Logger
}

var _ memcontract.Recaller = (*Recaller)(nil)

// Option customizes a deterministic Recaller.
type Option func(*Recaller)

// WithClock injects a deterministic clock for tests.
func WithClock(now func() time.Time) Option {
	return func(recaller *Recaller) {
		if now != nil {
			recaller.now = now
		}
	}
}

// WithLogger injects the logger used for failure-safe side effects.
func WithLogger(logger *slog.Logger) Option {
	return func(recaller *Recaller) {
		if logger != nil {
			recaller.logger = logger
		}
	}
}

// WithWeights overrides the deterministic score-fusion weights.
func WithWeights(weights Weights) Option {
	return func(recaller *Recaller) {
		recaller.weights = normalizeWeights(weights)
	}
}

// WithSignalRecorder moves recall-signal writes onto a bounded async worker.
func WithSignalRecorder(recorder *SignalRecorder) Option {
	return func(recaller *Recaller) {
		recaller.signalRecorder = recorder
	}
}

// New constructs a deterministic Recaller over a storage source.
func New(source Source, opts ...Option) *Recaller {
	recaller := &Recaller{
		source:  source,
		now:     func() time.Time { return time.Now().UTC() },
		weights: defaultWeights,
		logger:  slog.Default(),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(recaller)
		}
	}
	recaller.weights = normalizeWeights(recaller.weights)
	return recaller
}

// Recall returns a prompt-ready package using deterministic lexical ranking.
func (r *Recaller) Recall(
	ctx context.Context,
	query memcontract.Query,
	opts memcontract.RecallOptions,
) (memcontract.Packaged, error) {
	if ctx == nil {
		return memcontract.Packaged{}, errors.New("memory recall: context is required")
	}
	if r == nil || r.source == nil {
		return memcontract.Packaged{}, errors.New("memory recall: source is required")
	}

	query.QueryText = strings.TrimSpace(query.QueryText)
	normalizedOpts := normalizeOptions(opts)
	if !normalizedOpts.AllowTrivialQuery && isTrivialQuery(query.QueryText) {
		r.recordSkipped(ctx, query, "trivial_query")
		return emptyPackage(), nil
	}

	candidates, err := r.source.Candidates(ctx, query, normalizedOpts)
	if err != nil {
		return memcontract.Packaged{}, fmt.Errorf("memory recall: load candidates: %w", err)
	}
	now := r.now().UTC()
	ranked, shadows := rankCandidates(candidates, normalizedOpts, r.weights, now)
	for _, shadow := range shadows {
		r.recordShadow(ctx, shadow)
	}

	packaged := packageCandidates(ranked, normalizedOpts.TopK, now)
	if len(packaged.Blocks) > 0 {
		r.recordSignals(ctx, query, signalsForRanked(ranked, normalizedOpts.TopK, now))
	}
	r.recordExecuted(ctx, query, packagedEntryCount(packaged))
	return packaged, nil
}

type rankedCandidate struct {
	Candidate
	score float64
	why   []string
}

func rankCandidates(
	candidates []Candidate,
	opts memcontract.RecallOptions,
	weights Weights,
	now time.Time,
) ([]rankedCandidate, []Shadow) {
	alreadySurfaced := surfacedSet(opts.AlreadySurfaced)
	merged := mergeCandidates(candidates)
	ranked := make([]rankedCandidate, 0, len(merged))
	for _, candidate := range merged {
		if !opts.IncludeSystem && !candidate.Injection {
			continue
		}
		if _, seen := alreadySurfaced[candidate.ChunkID]; seen && !opts.IncludeAlreadySurfaced {
			continue
		}
		if _, seen := alreadySurfaced[candidate.EntryID]; seen && !opts.IncludeAlreadySurfaced {
			continue
		}
		score, why := scoreCandidate(candidate, weights, now)
		if score <= 0 {
			continue
		}
		ranked = append(ranked, rankedCandidate{Candidate: candidate, score: score, why: why})
	}
	sortRanked(ranked)
	return applyShadowRules(ranked)
}

func mergeCandidates(candidates []Candidate) []Candidate {
	byID := make(map[string]Candidate, len(candidates))
	for _, candidate := range candidates {
		candidate = normalizeCandidate(candidate)
		if candidate.ChunkID == "" {
			continue
		}
		current, exists := byID[candidate.ChunkID]
		if !exists {
			byID[candidate.ChunkID] = candidate
			continue
		}
		current.UnicodeScore = math.Max(current.UnicodeScore, candidate.UnicodeScore)
		current.TrigramScore = math.Max(current.TrigramScore, candidate.TrigramScore)
		current.RecallScore = math.Max(current.RecallScore, candidate.RecallScore)
		if current.Body == "" {
			current.Body = candidate.Body
		}
		byID[candidate.ChunkID] = current
	}

	merged := make([]Candidate, 0, len(byID))
	for _, candidate := range byID {
		merged = append(merged, candidate)
	}
	return merged
}

func normalizeCandidate(candidate Candidate) Candidate {
	candidate.ChunkID = strings.TrimSpace(candidate.ChunkID)
	candidate.EntryID = strings.TrimSpace(candidate.EntryID)
	candidate.WorkspaceID = strings.TrimSpace(candidate.WorkspaceID)
	candidate.Scope = candidate.Scope.Normalize()
	candidate.AgentName = strings.TrimSpace(candidate.AgentName)
	candidate.AgentTier = candidate.AgentTier.Normalize()
	candidate.Type = candidate.Type.Normalize()
	candidate.Slug = strings.TrimSpace(candidate.Slug)
	candidate.Filename = strings.TrimSpace(candidate.Filename)
	candidate.Title = strings.TrimSpace(candidate.Title)
	candidate.Body = strings.TrimSpace(candidate.Body)
	candidate.ContentHash = strings.TrimSpace(candidate.ContentHash)
	if candidate.Title == "" {
		candidate.Title = candidate.Filename
	}
	if candidate.Slug == "" {
		candidate.Slug = strings.TrimSuffix(candidate.Filename, ".md")
	}
	if candidate.ModTime.IsZero() {
		candidate.ModTime = time.Unix(0, 0).UTC()
	}
	return candidate
}

func scoreCandidate(candidate Candidate, weights Weights, now time.Time) (float64, []string) {
	unicodeScore := clamp01(candidate.UnicodeScore)
	trigramScore := clamp01(candidate.TrigramScore)
	recencyScore := recency(candidate.ModTime, now)
	signalScore := clamp01(candidate.RecallScore)
	score := weights.Unicode*unicodeScore +
		weights.Trigram*trigramScore +
		weights.Recency*recencyScore +
		weights.Signal*signalScore
	why := []string{
		fmt.Sprintf("unicode=%.3f", unicodeScore),
		fmt.Sprintf("trigram=%.3f", trigramScore),
		fmt.Sprintf("recency=%.3f", recencyScore),
		fmt.Sprintf("signal=%.3f", signalScore),
		fmt.Sprintf("score=%.3f", score),
	}
	return score, why
}

func recency(modTime time.Time, now time.Time) float64 {
	if modTime.IsZero() {
		return 0
	}
	ageHours := now.Sub(modTime.UTC()).Hours()
	if ageHours <= 0 {
		return 1
	}
	return math.Pow(0.5, (ageHours/24.0)/recencyHalfLifeDays)
}

func sortRanked(ranked []rankedCandidate) {
	sort.SliceStable(ranked, func(i, j int) bool {
		left := ranked[i]
		right := ranked[j]
		if left.score != right.score {
			return left.score > right.score
		}
		if left.scopeDepth() != right.scopeDepth() {
			return left.scopeDepth() > right.scopeDepth()
		}
		if !left.ModTime.Equal(right.ModTime) {
			return left.ModTime.After(right.ModTime)
		}
		return left.ChunkID < right.ChunkID
	})
}
