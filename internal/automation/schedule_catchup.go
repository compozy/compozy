package automation

import (
	"strings"
	"time"

	gocron "github.com/go-co-op/gocron/v2"
)

func scheduleCatchUpPolicy(schedule *ScheduleSpec, fallback SchedulerCatchUpPolicy) SchedulerCatchUpPolicy {
	if schedule != nil && schedule.CatchUpPolicy != "" {
		return schedule.CatchUpPolicy
	}
	return schedulerCatchUpPolicyOrDefault(fallback, SchedulerCatchUpPolicySkipMissed)
}

func scheduleMisfireGraceSeconds(schedule *ScheduleSpec) int {
	if schedule == nil || schedule.MisfireGraceSeconds < 0 {
		return 0
	}
	return schedule.MisfireGraceSeconds
}

func nextRunAfterMissed(job Job, missedAt time.Time, now time.Time, location *time.Location) *time.Time {
	if job.Schedule == nil {
		return nil
	}

	switch job.Schedule.Mode {
	case ScheduleModeCron:
		cronImpl := gocron.NewDefaultCron(false)
		expr := strings.TrimSpace(job.Schedule.Expr)
		if err := cronImpl.IsValid(expr, location, now); err != nil {
			return nil
		}
		next := cronImpl.Next(now)
		if next.IsZero() {
			return nil
		}
		return timePointer(next)
	case ScheduleModeEvery:
		interval, err := time.ParseDuration(strings.TrimSpace(job.Schedule.Interval))
		if err != nil || interval <= 0 {
			return nil
		}
		elapsed := now.Sub(missedAt)
		if elapsed < 0 {
			return timePointer(missedAt)
		}
		skippedIntervals := int64(elapsed/interval) + 1
		return timePointer(missedAt.Add(time.Duration(skippedIntervals) * interval))
	case ScheduleModeAt:
		return nil
	default:
		return nil
	}
}

func reconcileMissedSchedulerCursor(
	job Job,
	state SchedulerState,
	now time.Time,
	location *time.Location,
) (SchedulerState, SchedulerSkipReason) {
	missedAt := *state.NextRunAt
	state.LastMisfireAt = timePointer(now)
	state.MisfireCount++
	state.ConsecutiveResumeFailures = 0
	state.UpdatedAt = now

	policy := schedulerCatchUpPolicyOrDefault(state.CatchUpPolicy, SchedulerCatchUpPolicySkipMissed)
	switch policy {
	case SchedulerCatchUpPolicyRunOnce, SchedulerCatchUpPolicyCoalesce:
		coalescedAt := latestMissedRunAt(job, missedAt, now, location)
		state.NextRunAt = timePointer(coalescedAt)
		state.LastScheduledAt = timePointer(coalescedAt)
	case SchedulerCatchUpPolicyReplay:
		state.NextRunAt = timePointer(missedAt)
		state.LastScheduledAt = nil
	default:
		if now.Sub(missedAt) > schedulerCatchUpGrace(state) {
			state.NextRunAt = nextRunAfterMissed(job, missedAt, now, location)
			state.LastScheduledAt = timePointer(missedAt)
			return state, SchedulerSkipReasonGraceExceeded
		}
		coalescedAt := latestMissedRunAt(job, missedAt, now, location)
		state.NextRunAt = timePointer(coalescedAt)
		state.LastScheduledAt = timePointer(coalescedAt)
	}
	return state, ""
}

func scheduledFireIsCatchUp(scheduledAt time.Time, claimedAt time.Time, state SchedulerState) bool {
	if claimedAt.Sub(scheduledAt) > schedulerCatchUpGrace(state) {
		return true
	}
	if scheduledAt.After(claimedAt) || state.LastMisfireAt == nil {
		return false
	}
	switch schedulerCatchUpPolicyOrDefault(state.CatchUpPolicy, SchedulerCatchUpPolicySkipMissed) {
	case SchedulerCatchUpPolicyCoalesce, SchedulerCatchUpPolicyReplay, SchedulerCatchUpPolicyRunOnce:
		return !scheduledAt.After(*state.LastMisfireAt)
	default:
		return false
	}
}

func schedulerCatchUpGrace(state SchedulerState) time.Duration {
	if state.MisfireGraceSeconds > 0 {
		return time.Duration(state.MisfireGraceSeconds) * time.Second
	}
	return schedulerCatchUpJitterGrace
}

func latestMissedRunAt(job Job, missedAt time.Time, now time.Time, location *time.Location) time.Time {
	if job.Schedule == nil || missedAt.After(now) {
		return missedAt
	}
	switch job.Schedule.Mode {
	case ScheduleModeEvery:
		interval, err := time.ParseDuration(strings.TrimSpace(job.Schedule.Interval))
		if err != nil || interval <= 0 {
			return missedAt
		}
		elapsed := now.Sub(missedAt)
		if elapsed <= 0 {
			return missedAt
		}
		return missedAt.Add(time.Duration(int64(elapsed/interval)) * interval)
	case ScheduleModeCron:
		return latestMissedCronRunAt(job.Schedule.Expr, missedAt, now, location)
	default:
		return missedAt
	}
}

func latestMissedCronRunAt(
	expression string,
	missedAt time.Time,
	now time.Time,
	location *time.Location,
) time.Time {
	cronImpl := gocron.NewDefaultCron(false)
	if err := cronImpl.IsValid(strings.TrimSpace(expression), location, missedAt); err != nil {
		return missedAt
	}

	lower := missedAt.Add(-time.Second)
	upper := now
	for upper.Sub(lower) > time.Second {
		midpoint := lower.Add(upper.Sub(lower) / 2)
		next := cronImpl.Next(midpoint)
		if next.IsZero() || next.After(now) {
			upper = midpoint
			continue
		}
		lower = midpoint
	}
	latest := cronImpl.Next(lower)
	if latest.IsZero() || latest.After(now) {
		return missedAt
	}
	return latest
}

func (s *Scheduler) logScheduledSkip(job Job, claim SchedulerClaim, reason SchedulerSkipReason) {
	s.logger.Info(
		"automation.scheduler.job_skipped",
		"job_id", job.ID,
		"job_name", job.Name,
		"fire_id", claim.FireID,
		"scheduled_at", claim.ScheduledAt.Format(time.RFC3339Nano),
		"reason", reason,
	)
}
