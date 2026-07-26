package recall

import (
	"context"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func signalsForRanked(ranked []rankedCandidate, topK int, now time.Time) []Signal {
	if len(ranked) > topK {
		ranked = ranked[:topK]
	}
	signals := make([]Signal, 0, len(ranked))
	for _, candidate := range ranked {
		signals = append(signals, Signal{
			ChunkID:      candidate.ChunkID,
			WorkspaceID:  candidate.WorkspaceID,
			SurfaceID:    candidate.ChunkID,
			Score:        candidate.score,
			SurfacedAt:   now,
			SignalReason: strings.Join(candidate.why, ";"),
		})
	}
	return signals
}

func (r *Recaller) recordSignals(ctx context.Context, query memcontract.Query, signals []Signal) {
	if len(signals) == 0 {
		return
	}
	if r.signalRecorder != nil {
		r.signalRecorder.Submit(ctx, query, signals)
		return
	}
	if err := r.source.RecordRecall(ctx, signals); err != nil {
		r.warn("memory recall: record recall signal failed", "error", err)
		if eventErr := r.source.RecordRecallSignalFailed(ctx, query, err); eventErr != nil {
			r.warn("memory recall: record signal failure event failed", "error", eventErr)
		}
	}
}

func (r *Recaller) recordExecuted(ctx context.Context, query memcontract.Query, resultCount int) {
	if err := r.source.RecordRecallExecuted(ctx, query, resultCount); err != nil {
		r.warn("memory recall: record executed event failed", "error", err)
	}
}

func (r *Recaller) recordSkipped(ctx context.Context, query memcontract.Query, reason string) {
	if err := r.source.RecordRecallSkipped(ctx, query, reason); err != nil {
		r.warn("memory recall: record skipped event failed", "error", err)
	}
}

func (r *Recaller) recordShadow(ctx context.Context, shadow Shadow) {
	if err := r.source.RecordShadow(ctx, shadow); err != nil {
		r.warn("memory recall: record shadow event failed", "error", err)
	}
}

func (r *Recaller) warn(msg string, args ...any) {
	if r != nil && r.logger != nil {
		r.logger.Warn(msg, args...)
	}
}
