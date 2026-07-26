package observe

import (
	"encoding/json"

	"slices"
	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

func summarizeRuns(runs []taskpkg.Run) []TaskRunTotal {
	counts := make(map[string]TaskRunTotal)
	for _, item := range runs {
		channel := taskRunParticipationChannel(item)
		key := item.Status.Normalize().String() + "\x00" + string(item.Origin.Kind.Normalize()) + "\x00" + channel
		current := counts[key]
		current.Status = item.Status.Normalize()
		current.OriginKind = item.Origin.Kind.Normalize()
		current.ChannelID = channel
		current.Count++
		counts[key] = current
	}
	rows := make([]TaskRunTotal, 0, len(counts))
	for _, item := range counts {
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(left, right TaskRunTotal) int {
		if cmp := strings.Compare(left.Status.String(), right.Status.String()); cmp != 0 {
			return cmp
		}
		if cmp := strings.Compare(string(left.OriginKind), string(right.OriginKind)); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.ChannelID, right.ChannelID)
	})
	return rows
}

func summarizeOwners(tasks []taskpkg.Summary) []TaskOwnerTotal {
	counts := make(map[string]TaskOwnerTotal)
	for idx := range tasks {
		item := &tasks[idx]
		if item.Owner == nil {
			continue
		}
		key := string(item.Owner.Kind.Normalize()) + "\x00" + strings.TrimSpace(item.Owner.Ref)
		current := counts[key]
		current.OwnerKind = item.Owner.Kind.Normalize()
		current.OwnerRef = strings.TrimSpace(item.Owner.Ref)
		current.Count++
		counts[key] = current
	}
	rows := make([]TaskOwnerTotal, 0, len(counts))
	for _, item := range counts {
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(left, right TaskOwnerTotal) int {
		if cmp := strings.Compare(string(left.OwnerKind), string(right.OwnerKind)); cmp != 0 {
			return cmp
		}
		return strings.Compare(left.OwnerRef, right.OwnerRef)
	})
	return rows
}

func summarizeQueueDepth(runs []taskpkg.Run, now func() time.Time) []TaskQueueDepth {
	counts := make(map[string]TaskQueueDepth)
	currentTime := time.Now().UTC()
	if now != nil {
		currentTime = now().UTC()
	}
	for _, item := range runs {
		if item.Status.Normalize() != taskpkg.TaskRunStatusQueued {
			continue
		}
		channel := taskRunParticipationChannel(item)
		current := counts[channel]
		current.ChannelID = channel
		current.Count++
		if current.OldestQueuedAt.IsZero() || item.QueuedAt.Before(current.OldestQueuedAt) {
			current.OldestQueuedAt = item.QueuedAt
			age := max(currentTime.Sub(item.QueuedAt), 0)
			current.OldestQueueAgeMilli = age.Milliseconds()
		}
		counts[channel] = current
	}
	rows := make([]TaskQueueDepth, 0, len(counts))
	for _, item := range counts {
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(left, right TaskQueueDepth) int {
		return strings.Compare(left.ChannelID, right.ChannelID)
	})
	return rows
}

func summarizeCancelRequests(events []taskpkg.Event) []TaskCancelRequestTotal {
	counts := make(map[string]TaskCancelRequestTotal)
	for _, item := range events {
		if item.EventType != taskEventCanceled {
			continue
		}
		key := string(item.Origin.Kind.Normalize())
		current := counts[key]
		current.OriginKind = item.Origin.Kind.Normalize()
		current.Count++
		counts[key] = current
	}
	rows := make([]TaskCancelRequestTotal, 0, len(counts))
	for _, item := range counts {
		rows = append(rows, item)
	}
	slices.SortFunc(rows, func(left, right TaskCancelRequestTotal) int {
		return strings.Compare(string(left.OriginKind), string(right.OriginKind))
	})
	return rows
}

func summarizeClaimLatency(runs []taskpkg.Run) LatencyMetric {
	return summarizeLatency(runs, func(run taskpkg.Run) (time.Duration, bool) {
		if run.ClaimedAt.IsZero() {
			return 0, false
		}
		duration := max(run.ClaimedAt.Sub(run.QueuedAt), 0)
		return duration, true
	})
}

func summarizeStartLatency(runs []taskpkg.Run) LatencyMetric {
	return summarizeLatency(runs, func(run taskpkg.Run) (time.Duration, bool) {
		if run.StartedAt.IsZero() {
			return 0, false
		}
		base := run.ClaimedAt
		if base.IsZero() {
			base = run.QueuedAt
		}
		duration := max(run.StartedAt.Sub(base), 0)
		return duration, true
	})
}

func summarizeLatency(runs []taskpkg.Run, measure func(taskpkg.Run) (time.Duration, bool)) LatencyMetric {
	var total time.Duration
	var maxDuration time.Duration
	samples := 0
	for _, item := range runs {
		duration, ok := measure(item)
		if !ok {
			continue
		}
		samples++
		total += duration
		if duration > maxDuration {
			maxDuration = duration
		}
	}
	if samples == 0 {
		return LatencyMetric{}
	}
	return LatencyMetric{
		Samples:       samples,
		AverageMillis: (total / time.Duration(samples)).Milliseconds(),
		MaximumMillis: maxDuration.Milliseconds(),
	}
}

func summarizeRecovery(events []taskpkg.Event) TaskRecoveryTotals {
	totals := TaskRecoveryTotals{}
	for _, item := range events {
		if item.EventType != taskEventRunRecovered {
			continue
		}
		var payload taskRecoveryPayload
		if len(item.Payload) > 0 {
			if err := json.Unmarshal(item.Payload, &payload); err != nil {
				continue
			}
		}
		switch payload.Action.Normalize() {
		case taskpkg.RunBootRecoveryRequeue:
			totals.Requeued++
		case taskpkg.RunBootRecoveryMarkRunning:
			totals.MarkedRunning++
		case taskpkg.RunBootRecoveryFail:
			totals.Failed++
		}
	}
	return totals
}

func countEventsByType(events []taskpkg.Event, eventType string) int {
	count := 0
	for _, item := range events {
		if item.EventType == eventType {
			count++
		}
	}
	return count
}

func findStuckRuns(runs []taskpkg.Run, now time.Time, cfg TaskHealthConfig) []StuckTaskRun {
	stuck := make([]StuckTaskRun, 0)
	for _, item := range runs {
		threshold, age, ok := runStuckAge(item, now, cfg)
		if !ok || threshold <= 0 || age < threshold {
			continue
		}
		stuck = append(stuck, StuckTaskRun{
			TaskID:     strings.TrimSpace(item.TaskID),
			RunID:      strings.TrimSpace(item.ID),
			Status:     item.Status.Normalize(),
			OriginKind: item.Origin.Kind.Normalize(),
			ChannelID:  taskRunParticipationChannel(item),
			SessionID:  strings.TrimSpace(item.SessionID),
			AgeMillis:  age.Milliseconds(),
		})
	}
	return stuck
}

func runStuckAge(run taskpkg.Run, now time.Time, cfg TaskHealthConfig) (time.Duration, time.Duration, bool) {
	switch run.Status.Normalize() {
	case taskpkg.TaskRunStatusClaimed:
		base := run.ClaimedAt
		if base.IsZero() {
			base = run.QueuedAt
		}
		return cfg.ClaimedStuckAfter, safeSince(now, base), true
	case taskpkg.TaskRunStatusStarting:
		base := run.ClaimedAt
		if base.IsZero() {
			base = run.QueuedAt
		}
		return cfg.StartingStuckAfter, safeSince(now, base), true
	case taskpkg.TaskRunStatusRunning:
		base := run.StartedAt
		if base.IsZero() {
			base = run.ClaimedAt
		}
		if base.IsZero() {
			base = run.QueuedAt
		}
		return cfg.RunningStuckAfter, safeSince(now, base), true
	default:
		return 0, 0, false
	}
}

func safeSince(now time.Time, started time.Time) time.Duration {
	if started.IsZero() {
		return 0
	}
	duration := now.Sub(started)
	if duration < 0 {
		return 0
	}
	return duration
}

func sortStuckRuns(runs []StuckTaskRun) {
	slices.SortFunc(runs, func(left, right StuckTaskRun) int {
		switch {
		case left.AgeMillis > right.AgeMillis:
			return -1
		case left.AgeMillis < right.AgeMillis:
			return 1
		default:
			return strings.Compare(left.RunID, right.RunID)
		}
	})
}
