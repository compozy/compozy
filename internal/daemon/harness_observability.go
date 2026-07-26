package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	eventspkg "github.com/compozy/agh/internal/events"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
)

// Harness lifecycle summaries stay on the existing global event-summary timeline so
// observe.QueryEvents plus HTTP/UDS readers can inspect startup, prompt, and
// reentry decisions without a second harness-specific read model. Startup
// summaries queue until OnSessionCreated because event_summaries.session_id
// references the durable sessions index.
const (
	harnessSummaryContextResolved  = eventspkg.HarnessContextResolved
	harnessSummarySectionSelected  = eventspkg.HarnessSectionSelected
	harnessSummaryAugmenterApplied = eventspkg.HarnessAugmenterApplied
	harnessSummaryAugmenterFailed  = eventspkg.HarnessAugmenterFailed
	harnessSummaryDefaultAgentName = "daemon"
)

type harnessLifecycleRecorder struct {
	mu      sync.Mutex
	store   store.EventSummaryStore
	logger  *slog.Logger
	now     func() time.Time
	pending map[string][]store.EventSummary
}

type harnessAugmenterObservation struct {
	Name           HarnessAugmenter
	Outcome        string
	Critical       bool
	Budget         int
	BudgetBehavior promptInputAugmenterBudgetBehavior
	Consumed       int
	Remaining      int
}

func newHarnessLifecycleRecorder(logger *slog.Logger, now func() time.Time) *harnessLifecycleRecorder {
	if logger == nil {
		logger = slog.Default()
	}
	if now == nil {
		now = time.Now
	}
	return &harnessLifecycleRecorder{
		logger:  logger,
		now:     now,
		pending: make(map[string][]store.EventSummary),
	}
}

func (r *harnessLifecycleRecorder) SetStore(summaryStore store.EventSummaryStore) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = summaryStore
}

func (r *harnessLifecycleRecorder) OnSessionCreated(ctx context.Context, sess *session.Session) {
	if r == nil || sess == nil {
		return
	}
	info := sess.Info()
	if info == nil {
		return
	}

	sessionID := strings.TrimSpace(info.ID)
	if sessionID == "" {
		return
	}

	r.mu.Lock()
	summaryStore := r.store
	if summaryStore == nil {
		r.mu.Unlock()
		return
	}
	summaries := append([]store.EventSummary(nil), r.pending[sessionID]...)
	delete(r.pending, sessionID)
	r.mu.Unlock()

	if len(summaries) == 0 {
		return
	}

	for _, summary := range summaries {
		summary = harnessEventSummaryWithLineage(summary, info.Lineage)
		r.write(ctx, summaryStore, summary)
	}
}

func (r *harnessLifecycleRecorder) OnSessionStopped(_ context.Context, sess *session.Session) {
	if r == nil || sess == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, strings.TrimSpace(sess.ID))
}

func (r *harnessLifecycleRecorder) RecordStartupContextResolved(
	startup session.StartupPromptContext,
	resolved ResolvedHarnessContext,
	timestamp time.Time,
) {
	r.queue(
		store.EventSummary{
			SessionID: strings.TrimSpace(startup.SessionID),
			Type:      harnessSummaryContextResolved,
			AgentName: harnessSummaryAgentName(startup.AgentName),
			Summary:   harnessContextResolutionSummary(resolved),
			Timestamp: r.timestamp(timestamp),
		},
	)
}

func (r *harnessLifecycleRecorder) RecordStartupSectionSelected(
	startup session.StartupPromptContext,
	resolved ResolvedHarnessContext,
	descriptors []PromptSectionDescriptor,
	timestamp time.Time,
) {
	r.queue(
		store.EventSummary{
			SessionID: strings.TrimSpace(startup.SessionID),
			Type:      harnessSummarySectionSelected,
			AgentName: harnessSummaryAgentName(startup.AgentName),
			Summary:   harnessSectionSelectionSummary(resolved, descriptors),
			Timestamp: r.timestamp(timestamp),
		},
	)
}

func (r *harnessLifecycleRecorder) RecordPromptContextResolved(
	ctx context.Context,
	info *session.Info,
	resolved ResolvedHarnessContext,
	timestamp time.Time,
) {
	if info == nil {
		return
	}
	r.record(
		ctx,
		harnessEventSummaryWithLineage(store.EventSummary{
			SessionID: strings.TrimSpace(info.ID),
			Type:      harnessSummaryContextResolved,
			AgentName: harnessSummaryAgentName(info.AgentName),
			Summary:   harnessContextResolutionSummary(resolved),
			Timestamp: r.timestamp(timestamp),
		}, info.Lineage),
	)
}

func (r *harnessLifecycleRecorder) RecordSyntheticContextResolved(
	ctx context.Context,
	sessionID string,
	agentName string,
	resolved ResolvedHarnessContext,
	timestamp time.Time,
) {
	r.record(
		ctx,
		store.EventSummary{
			SessionID: strings.TrimSpace(sessionID),
			Type:      harnessSummaryContextResolved,
			AgentName: harnessSummaryAgentName(agentName),
			Summary:   harnessContextResolutionSummary(resolved),
			Timestamp: r.timestamp(timestamp),
		},
	)
}

func (r *harnessLifecycleRecorder) RecordAugmenterApplied(
	ctx context.Context,
	info *session.Info,
	resolved ResolvedHarnessContext,
	observation harnessAugmenterObservation,
	timestamp time.Time,
) {
	if info == nil {
		return
	}
	r.record(
		ctx,
		harnessEventSummaryWithLineage(store.EventSummary{
			SessionID: strings.TrimSpace(info.ID),
			Type:      harnessSummaryAugmenterApplied,
			AgentName: harnessSummaryAgentName(info.AgentName),
			Summary:   harnessAugmenterAppliedSummary(resolved, observation),
			Timestamp: r.timestamp(timestamp),
		}, info.Lineage),
	)
}

func (r *harnessLifecycleRecorder) RecordAugmenterFailed(
	ctx context.Context,
	info *session.Info,
	resolved ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	err error,
	timestamp time.Time,
) {
	if info == nil || err == nil {
		return
	}
	r.record(
		ctx,
		harnessEventSummaryWithLineage(store.EventSummary{
			SessionID: strings.TrimSpace(info.ID),
			Type:      harnessSummaryAugmenterFailed,
			AgentName: harnessSummaryAgentName(info.AgentName),
			Summary:   harnessAugmenterFailedSummary(resolved, descriptor, err),
			Timestamp: r.timestamp(timestamp),
		}, info.Lineage),
	)
}

func (r *harnessLifecycleRecorder) timestamp(timestamp time.Time) time.Time {
	if !timestamp.IsZero() {
		return timestamp.UTC()
	}
	if r == nil || r.now == nil {
		return time.Now().UTC()
	}
	return r.now().UTC()
}

func (r *harnessLifecycleRecorder) queue(summary store.EventSummary) {
	if r == nil || strings.TrimSpace(summary.SessionID) == "" || strings.TrimSpace(summary.AgentName) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending[summary.SessionID] = append(r.pending[summary.SessionID], summary)
}

func (r *harnessLifecycleRecorder) record(ctx context.Context, summary store.EventSummary) {
	if r == nil || strings.TrimSpace(summary.SessionID) == "" || strings.TrimSpace(summary.AgentName) == "" {
		return
	}
	r.mu.Lock()
	summaryStore := r.store
	r.mu.Unlock()
	if summaryStore == nil {
		return
	}
	r.write(ctx, summaryStore, summary)
}

func (r *harnessLifecycleRecorder) write(
	ctx context.Context,
	summaryStore store.EventSummaryStore,
	summary store.EventSummary,
) {
	if summaryStore == nil {
		return
	}
	if err := summaryStore.WriteEventSummary(ctx, summary); err != nil {
		r.logger.Error(
			"daemon: write harness lifecycle summary failed",
			"session_id",
			summary.SessionID,
			"agent_name",
			summary.AgentName,
			"type",
			summary.Type,
			"error",
			err,
		)
	}
}

func harnessEventSummaryWithLineage(
	summary store.EventSummary,
	lineage *store.SessionLineage,
) store.EventSummary {
	normalized := store.NormalizeSessionLineage(summary.SessionID, lineage)
	if normalized == nil {
		return summary
	}
	summary.ParentSessionID = normalized.ParentSessionID
	summary.RootSessionID = normalized.RootSessionID
	summary.SpawnDepth = normalized.SpawnDepth
	return summary
}

func harnessSummaryAgentName(agentName string) string {
	if trimmed := strings.TrimSpace(agentName); trimmed != "" {
		return trimmed
	}
	return harnessSummaryDefaultAgentName
}

func harnessContextResolutionSummary(resolved ResolvedHarnessContext) string {
	parts := []string{
		"surface=" + strings.TrimSpace(string(resolved.Surface)),
		"session_type=" + strings.TrimSpace(string(resolved.Session.Type)),
		"session_class=" + strings.TrimSpace(string(resolved.Policy.SessionClass)),
		"turn_origin=" + strings.TrimSpace(string(resolved.Policy.TurnOrigin)),
		"network_live=" + strconv.FormatBool(resolved.Session.NetworkLive),
		"sections=" + joinHarnessPromptSections(resolved.Policy.IncludeSections),
		"augmenters=" + joinHarnessAugmenters(resolved.Policy.EnableAugmenters),
		"reentry=" + strings.TrimSpace(string(resolved.Policy.ReentryMode)),
		"detached=" + strings.TrimSpace(string(resolved.Policy.DetachedRunMode)),
		"label=" + truncateHarnessToken(resolved.Policy.DiagnosticLabel, 120),
	}
	if workspaceID := strings.TrimSpace(resolved.Session.WorkspaceID); workspaceID != "" {
		parts = append(parts, "workspace_id="+truncateHarnessToken(workspaceID, 120))
	}
	if channel := strings.TrimSpace(resolved.Session.NetworkParticipation.ChannelID); channel != "" {
		parts = append(parts, "channel="+truncateHarnessQuoted(channel, 80))
	}
	if synthetic := resolved.Turn.Synthetic; synthetic != nil {
		if reason := strings.TrimSpace(synthetic.Reason); reason != "" {
			parts = append(parts, "synthetic_reason="+truncateHarnessToken(reason, 120))
		}
		if trigger := strings.TrimSpace(synthetic.Trigger); trigger != "" {
			parts = append(parts, "synthetic_trigger="+truncateHarnessToken(trigger, 120))
		}
		if sourceTask := strings.TrimSpace(synthetic.SourceTask); sourceTask != "" {
			parts = append(parts, "source_task="+truncateHarnessToken(sourceTask, 120))
		}
		if sourceRunID := strings.TrimSpace(synthetic.SourceRunID); sourceRunID != "" {
			parts = append(parts, "source_run="+truncateHarnessToken(sourceRunID, 120))
		}
	}
	if detached := resolved.Turn.Detached; detached != nil {
		if taskID := strings.TrimSpace(detached.TaskID); taskID != "" {
			parts = append(parts, "detached_task="+truncateHarnessToken(taskID, 120))
		}
		if runID := strings.TrimSpace(detached.TaskRunID); runID != "" {
			parts = append(parts, "detached_run="+truncateHarnessToken(runID, 120))
		}
	}
	return strings.Join(parts, " ")
}

func harnessSectionSelectionSummary(
	resolved ResolvedHarnessContext,
	descriptors []PromptSectionDescriptor,
) string {
	names := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if name := strings.TrimSpace(descriptor.Name); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join([]string{
		"surface=" + strings.TrimSpace(string(resolved.Surface)),
		"selected=" + joinHarnessNames(names),
		"count=" + strconv.Itoa(len(names)),
		"label=" + truncateHarnessToken(resolved.Policy.DiagnosticLabel, 120),
	}, " ")
}

func harnessAugmenterAppliedSummary(
	resolved ResolvedHarnessContext,
	observation harnessAugmenterObservation,
) string {
	return strings.Join([]string{
		"surface=" + strings.TrimSpace(string(resolved.Surface)),
		"augmenter=" + strings.TrimSpace(string(observation.Name)),
		"outcome=" + truncateHarnessToken(observation.Outcome, 64),
		"critical=" + strconv.FormatBool(observation.Critical),
		"budget=" + strconv.Itoa(max(observation.Budget, 0)),
		"budget_behavior=" + strings.TrimSpace(string(observation.BudgetBehavior)),
		"consumed=" + strconv.Itoa(max(observation.Consumed, 0)),
		"remaining=" + strconv.Itoa(max(observation.Remaining, 0)),
		"label=" + truncateHarnessToken(resolved.Policy.DiagnosticLabel, 120),
	}, " ")
}

func harnessAugmenterFailedSummary(
	resolved ResolvedHarnessContext,
	descriptor promptInputAugmenterDescriptor,
	err error,
) string {
	disposition := "warn_continue"
	if descriptor.Critical {
		disposition = "abort"
	}
	return strings.Join([]string{
		"surface=" + strings.TrimSpace(string(resolved.Surface)),
		"augmenter=" + strings.TrimSpace(string(descriptor.Name)),
		"critical=" + strconv.FormatBool(descriptor.Critical),
		"disposition=" + disposition,
		"label=" + truncateHarnessToken(resolved.Policy.DiagnosticLabel, 120),
		"error=" + truncateHarnessQuoted(err.Error(), 160),
	}, " ")
}

func joinHarnessPromptSections(sections []HarnessPromptSection) string {
	if len(sections) == 0 {
		return "-"
	}
	names := make([]string, 0, len(sections))
	for _, section := range sections {
		if name := strings.TrimSpace(string(section)); name != "" {
			names = append(names, name)
		}
	}
	return joinHarnessNames(names)
}

func joinHarnessAugmenters(augmenters []HarnessAugmenter) string {
	if len(augmenters) == 0 {
		return "-"
	}
	names := make([]string, 0, len(augmenters))
	for _, augmenter := range augmenters {
		if name := strings.TrimSpace(string(augmenter)); name != "" {
			names = append(names, name)
		}
	}
	return joinHarnessNames(names)
}

func joinHarnessNames(names []string) string {
	if len(names) == 0 {
		return "-"
	}
	filtered := make([]string, 0, len(names))
	for _, name := range names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			filtered = append(filtered, truncateHarnessToken(trimmed, 80))
		}
	}
	if len(filtered) == 0 {
		return "-"
	}
	return strings.Join(filtered, "|")
}

func truncateHarnessQuoted(value string, maxRunes int) string {
	return fmt.Sprintf("%q", truncateHarnessText(value, maxRunes))
}

func truncateHarnessToken(value string, maxRunes int) string {
	return strings.ReplaceAll(truncateHarnessText(value, maxRunes), " ", "_")
}

func truncateHarnessText(value string, maxRunes int) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || maxRunes <= 0 || utf8.RuneCountInString(trimmed) <= maxRunes {
		return trimmed
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))

	count := 0
	for _, r := range trimmed {
		if count >= maxRunes {
			break
		}
		builder.WriteRune(r)
		count++
	}
	return strings.TrimSpace(builder.String()) + "..."
}
