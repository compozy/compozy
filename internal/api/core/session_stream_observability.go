package core

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

func (h *BaseHandlers) beginSessionStreamTelemetry(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	query store.EventQuery,
	catchUpBatchSize int,
	options sessionStreamOptions,
	subscriptionActive bool,
) func() {
	activeStreams := h.activeSessionStreams.Add(1)
	attrs := h.sessionStreamAttrs(
		sessionID,
		info,
		options.frameMode,
		query.AfterSequence,
		catchUpBatchSize,
		activeStreams,
	)
	attrs = append(attrs,
		"expected_epoch_supplied", options.expectedEpoch != nil,
		"expected_generation_supplied", options.expectedGeneration != nil,
		"subscription_active", subscriptionActive,
	)
	h.sessionStreamLogger().InfoContext(ctx, "api: session stream opened", attrs...)
	return func() {
		activeStreams := h.activeSessionStreams.Add(-1)
		attrs := h.sessionStreamAttrs(
			sessionID,
			info,
			options.frameMode,
			query.AfterSequence,
			catchUpBatchSize,
			activeStreams,
		)
		attrs = append(attrs,
			"expected_epoch_supplied", options.expectedEpoch != nil,
			"expected_generation_supplied", options.expectedGeneration != nil,
			"subscription_active", subscriptionActive,
		)
		h.sessionStreamLogger().InfoContext(ctx, "api: session stream closed", attrs...)
	}
}

func (h *BaseHandlers) logSessionStreamPoll(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	frameMode string,
	afterSequence int64,
	catchUpBatchSize int,
	duration time.Duration,
	err error,
) {
	attrs := h.sessionStreamAttrs(
		sessionID,
		info,
		frameMode,
		afterSequence,
		catchUpBatchSize,
		h.activeSessionStreams.Load(),
	)
	attrs = append(attrs, "poll_duration_ms", durationMilliseconds(duration))
	if err != nil {
		attrs = append(attrs, "error", err)
		h.sessionStreamLogger().WarnContext(ctx, "api: session stream poll failed", attrs...)
		return
	}
	h.sessionStreamLogger().DebugContext(ctx, "api: session stream poll tick", attrs...)
}

func (h *BaseHandlers) logSessionStreamFallback(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	frameMode string,
	reason string,
	afterSequence int64,
	nextSequence int64,
) {
	attrs := h.sessionStreamAttrs(
		sessionID,
		info,
		frameMode,
		afterSequence,
		0,
		h.activeSessionStreams.Load(),
	)
	attrs = append(attrs,
		"fallback_reason", reason,
		"next_sequence", nextSequence,
	)
	h.sessionStreamLogger().InfoContext(ctx, "api: session stream catch-up fallback", attrs...)
}

func (h *BaseHandlers) logSessionStreamSubscriptionClosed(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	frameMode string,
	afterSequence int64,
) {
	h.logSessionStreamFallback(ctx, sessionID, info, frameMode, "subscription_closed", afterSequence, 0)
}

func (h *BaseHandlers) logSessionStreamSequenceGap(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	frameMode string,
	afterSequence int64,
	nextSequence int64,
) {
	h.logSessionStreamFallback(ctx, sessionID, info, frameMode, "sequence_gap", afterSequence, nextSequence)
}

func (h *BaseHandlers) logTranscriptAssembly(
	ctx context.Context,
	sessionID string,
	info *session.Info,
	phase string,
	afterSequence int64,
	eventCount int,
	entryCount int,
	duration time.Duration,
	err error,
) {
	attrs := h.sessionStreamAttrs(
		sessionID,
		info,
		"transcript",
		afterSequence,
		eventCount,
		h.activeSessionStreams.Load(),
	)
	attrs = append(attrs,
		"assembly_duration_ms", durationMilliseconds(duration),
		"entry_count", entryCount,
		"phase", phase,
	)
	if errors.Is(err, session.ErrSessionNotFound) {
		attrs = append(attrs, "error", err)
		h.sessionStreamLogger().DebugContext(ctx, "api: session transcript no longer available", attrs...)
		return
	}
	if err != nil {
		attrs = append(attrs, "error", err)
		h.sessionStreamLogger().WarnContext(ctx, "api: session transcript assembly failed", attrs...)
		return
	}
	h.sessionStreamLogger().DebugContext(ctx, "api: session transcript assembled", attrs...)
}

func (h *BaseHandlers) sessionStreamAttrs(
	sessionID string,
	info *session.Info,
	frameMode string,
	cursor int64,
	catchUpBatchSize int,
	activeStreams int64,
) []any {
	workspaceID, workspacePath := h.streamWorkspaceFields(info)
	return []any{
		"session_id", sessionID,
		"workspace_id", workspaceID,
		"workspace_path", workspacePath,
		"frame_mode", frameMode,
		"cursor", cursor,
		"catch_up_batch_size", catchUpBatchSize,
		"active_streams", activeStreams,
	}
}

func (h *BaseHandlers) sessionStreamLogger() *slog.Logger {
	if h != nil && h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}

func durationMilliseconds(duration time.Duration) float64 {
	return float64(duration.Microseconds()) / 1000
}
