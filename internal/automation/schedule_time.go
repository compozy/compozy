package automation

import (
	"errors"

	"strings"

	"time"

	gocron "github.com/go-co-op/gocron/v2"
)

func normalizeScheduledJob(job Job) (Job, error) {
	job.ID = strings.TrimSpace(job.ID)
	if job.ID == "" {
		return Job{}, errors.New("automation: job.id is required for scheduler registration")
	}
	if err := job.Validate("job"); err != nil {
		return Job{}, err
	}
	return job, nil
}

func predictNextRun(job Job, registeredAt time.Time, location *time.Location) time.Time {
	if job.Schedule == nil {
		return time.Time{}
	}

	switch job.Schedule.Mode {
	case ScheduleModeCron:
		cronImpl := gocron.NewDefaultCron(false)
		expr := strings.TrimSpace(job.Schedule.Expr)
		if err := cronImpl.IsValid(expr, location, registeredAt); err != nil {
			return time.Time{}
		}
		return cronImpl.Next(registeredAt)
	case ScheduleModeEvery:
		interval, err := time.ParseDuration(strings.TrimSpace(job.Schedule.Interval))
		if err != nil {
			return time.Time{}
		}
		return registeredAt.Add(interval)
	case ScheduleModeAt:
		atTime, err := time.Parse(time.RFC3339, strings.TrimSpace(job.Schedule.Time))
		if err != nil {
			return time.Time{}
		}
		return atTime
	default:
		return time.Time{}
	}
}

func nextRunAfter(job Job, scheduledAt time.Time, location *time.Location) *time.Time {
	if job.Schedule == nil {
		return nil
	}

	var next time.Time
	switch job.Schedule.Mode {
	case ScheduleModeCron:
		cronImpl := gocron.NewDefaultCron(false)
		expr := strings.TrimSpace(job.Schedule.Expr)
		if err := cronImpl.IsValid(expr, location, scheduledAt); err != nil {
			return nil
		}
		next = cronImpl.Next(scheduledAt)
	case ScheduleModeEvery:
		interval, err := time.ParseDuration(strings.TrimSpace(job.Schedule.Interval))
		if err != nil || interval <= 0 {
			return nil
		}
		next = scheduledAt.Add(interval)
	case ScheduleModeAt:
		return nil
	default:
		return nil
	}
	if next.IsZero() {
		return nil
	}
	return timePointer(next)
}

func schedulerCatchUpPolicyOrDefault(
	policy SchedulerCatchUpPolicy,
	fallback SchedulerCatchUpPolicy,
) SchedulerCatchUpPolicy {
	if policy == "" {
		if fallback != "" {
			return fallback
		}
		return SchedulerCatchUpPolicySkipMissed
	}
	return policy
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil || value.IsZero() {
		return nil
	}
	clone := *value
	return &clone
}

func unregisteredJobState(jobID string) ScheduledJobState {
	return ScheduledJobState{
		JobID:      jobID,
		Registered: false,
	}
}
