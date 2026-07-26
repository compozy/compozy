package scheduler

import (
	"slices"
	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

type selectionResult struct {
	targets                []WakeTarget
	noMatch                []RunSnapshot
	starved                []RunSnapshot
	capacityWaiting        []RunSnapshot
	capacityWaitingReasons map[string]string
	convergenceCandidates  []RunSnapshot
	recentlyNotified       int
	unclaimable            int
}

func (s *Scheduler) selectWakeTargets(
	now time.Time,
	starvationNotBefore time.Time,
	pending []RunSnapshot,
	sessions []SessionSnapshot,
	active []taskpkg.Run,
) selectionResult {
	orderedPending := append([]RunSnapshot(nil), pending...)
	sortRunsForWake(orderedPending)
	orderedSessions := append([]SessionSnapshot(nil), sessions...)
	sortSessionsForWake(orderedSessions)

	occupied := activeSessionIDs(active)
	state := s.wakeStateSnapshot(now)
	result := selectionResult{capacityWaitingReasons: make(map[string]string)}

	for idx := range orderedPending {
		work := &orderedPending[idx]
		if !isPotentiallyClaimable(work) {
			result.unclaimable++
			continue
		}

		disposition := classifyRunCapacity(work, orderedSessions, occupied)
		queuedAge, hasQueuedAge := effectiveQueuedAge(now, work.Run.QueuedAt, starvationNotBefore)
		starved := s.starveThresholds.MinQueuedAge > 0 && hasQueuedAge &&
			queuedAge >= s.starveThresholds.MinQueuedAge

		switch disposition.Kind {
		case CapacityWaiting, CapacityIndeterminate:
			result.capacityWaiting = append(result.capacityWaiting, *work)
			result.capacityWaitingReasons[work.Run.ID] = disposition.Reason
			continue
		case CapacityUnmatched:
			result.noMatch = append(result.noMatch, *work)
			if starved {
				result.starved = append(result.starved, *work)
				result.convergenceCandidates = append(result.convergenceCandidates, *work)
			}
			continue
		case CapacityAvailable:
		default:
			result.noMatch = append(result.noMatch, *work)
			continue
		}

		if starved {
			result.convergenceCandidates = append(result.convergenceCandidates, *work)
			for _, candidate := range disposition.Available {
				occupied[strings.TrimSpace(candidate.ID)] = struct{}{}
				result.targets = append(result.targets, WakeTarget{
					Work:    *work,
					Session: candidate,
					Reason:  s.wakeReason,
				})
			}
			if len(disposition.Available) > 0 {
				result.starved = append(result.starved, *work)
			} else {
				result.recentlyNotified++
			}
			continue
		}

		chosen, ok := firstNotRecentlyWoken(now, s.wakeCooldown, state, work, disposition.Available)
		if !ok {
			result.recentlyNotified++
			continue
		}
		occupied[strings.TrimSpace(chosen.ID)] = struct{}{}
		result.targets = append(result.targets, WakeTarget{Work: *work, Session: chosen, Reason: s.wakeReason})
	}
	return result
}

func isPotentiallyClaimable(work *RunSnapshot) bool {
	if work == nil {
		return false
	}
	if strings.TrimSpace(work.Run.ID) == "" || strings.TrimSpace(work.Task.ID) == "" {
		return false
	}
	if work.Run.Status.Normalize() != taskpkg.TaskRunStatusQueued || work.Task.Paused {
		return false
	}
	switch work.Task.Status.Normalize() {
	case taskpkg.TaskStatusDraft,
		taskpkg.TaskStatusBlocked,
		taskpkg.TaskStatusNeedsAttention,
		taskpkg.TaskStatusCanceled:
		return false
	default:
		return true
	}
}

func activeSessionIDs(active []taskpkg.Run) map[string]struct{} {
	ids := make(map[string]struct{})
	for _, run := range active {
		switch run.Status.Normalize() {
		case taskpkg.TaskRunStatusClaimed, taskpkg.TaskRunStatusStarting, taskpkg.TaskRunStatusRunning:
		default:
			continue
		}
		if sessionID := strings.TrimSpace(run.SessionID); sessionID != "" {
			ids[sessionID] = struct{}{}
		}
	}
	return ids
}

func firstNotRecentlyWoken(
	now time.Time,
	cooldown time.Duration,
	state map[wakeKey]time.Time,
	work *RunSnapshot,
	candidates []SessionSnapshot,
) (SessionSnapshot, bool) {
	if work == nil {
		return SessionSnapshot{}, false
	}
	for _, candidate := range candidates {
		key := wakeKey{
			runID:     strings.TrimSpace(work.Run.ID),
			sessionID: strings.TrimSpace(candidate.ID),
		}
		last, exists := state[key]
		if !exists || cooldown <= 0 || now.Sub(last) >= cooldown {
			return candidate, true
		}
	}
	return SessionSnapshot{}, false
}

func sortRunsForWake(runs []RunSnapshot) {
	slices.SortStableFunc(runs, func(left, right RunSnapshot) int {
		if lv, rv := priorityValue(left.Task.Priority), priorityValue(right.Task.Priority); lv != rv {
			return rv - lv
		}
		if !left.Run.QueuedAt.Equal(right.Run.QueuedAt) {
			if left.Run.QueuedAt.Before(right.Run.QueuedAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(left.Run.ID, right.Run.ID)
	})
}

func sortSessionsForWake(sessions []SessionSnapshot) {
	slices.SortStableFunc(sessions, func(left, right SessionSnapshot) int {
		if !left.CreatedAt.Equal(right.CreatedAt) {
			if left.CreatedAt.Before(right.CreatedAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(left.ID, right.ID)
	})
}

func priorityValue(priority taskpkg.Priority) int {
	switch priority.Normalize() {
	case taskpkg.PriorityLow:
		return 10
	case taskpkg.PriorityHigh:
		return 30
	case taskpkg.PriorityUrgent:
		return 40
	default:
		return 20
	}
}

func runIDs(runs []RunSnapshot) []string {
	ids := make([]string, 0, len(runs))
	for idx := range runs {
		if id := strings.TrimSpace(runs[idx].Run.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
