package automation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

func scheduledFireID(jobID string, scheduledAt time.Time) string {
	return stableSchedulerID("fire", jobID, scheduledAt)
}

func scheduledRunID(jobID string, scheduledAt time.Time) string {
	return stableSchedulerID("run", jobID, scheduledAt)
}

func stableSchedulerID(prefix string, jobID string, scheduledAt time.Time) string {
	hash := sha256.Sum256([]byte(strings.TrimSpace(jobID) + "|" + scheduledAt.UTC().Format(time.RFC3339Nano)))
	return prefix + "_" + hex.EncodeToString(hash[:12])
}

func scheduleHash(schedule *ScheduleSpec) string {
	if schedule == nil {
		return ""
	}
	hash := sha256.Sum256([]byte(strings.Join([]string{
		string(schedule.Mode),
		strings.TrimSpace(schedule.Expr),
		strings.TrimSpace(schedule.Interval),
		strings.TrimSpace(schedule.Time),
		string(schedule.CatchUpPolicy),
		fmt.Sprintf("%d", schedule.MisfireGraceSeconds),
	}, "|")))
	return hex.EncodeToString(hash[:])
}
