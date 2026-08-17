package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

func sessionNullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}

func parseSessionInfoTimestamps(createdAtRaw string, updatedAtRaw string) (time.Time, time.Time, error) {
	createdAt, err := store.ParseTimestamp(createdAtRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	updatedAt, err := store.ParseTimestamp(updatedAtRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return createdAt, updatedAt, nil
}

func scanSessionLiveness(
	subprocessPID int,
	subprocessStartedAt sql.NullString,
	lastUpdateAt sql.NullString,
	stallState string,
	stallReason string,
	activityJSON string,
) (*store.SessionLivenessMeta, error) {
	if subprocessPID <= 0 &&
		(!subprocessStartedAt.Valid || strings.TrimSpace(subprocessStartedAt.String) == "") &&
		(!lastUpdateAt.Valid || strings.TrimSpace(lastUpdateAt.String) == "") &&
		strings.TrimSpace(stallState) == "" &&
		strings.TrimSpace(stallReason) == "" &&
		strings.TrimSpace(activityJSON) == "" {
		return nil, nil
	}

	meta := &store.SessionLivenessMeta{
		SubprocessPID: subprocessPID,
		StallState:    strings.TrimSpace(stallState),
		StallReason:   strings.TrimSpace(stallReason),
	}
	if subprocessStartedAt.Valid && strings.TrimSpace(subprocessStartedAt.String) != "" {
		parsed, err := store.ParseTimestamp(subprocessStartedAt.String)
		if err != nil {
			return nil, fmt.Errorf("store: parse session subprocess started at: %w", err)
		}
		meta.SubprocessStartedAt = &parsed
	}
	if lastUpdateAt.Valid && strings.TrimSpace(lastUpdateAt.String) != "" {
		parsed, err := store.ParseTimestamp(lastUpdateAt.String)
		if err != nil {
			return nil, fmt.Errorf("store: parse session last update at: %w", err)
		}
		meta.LastUpdateAt = &parsed
	}
	if strings.TrimSpace(activityJSON) != "" {
		activity, err := parseSessionActivityJSON(activityJSON)
		if err != nil {
			return nil, err
		}
		meta.Activity = activity
	}
	if err := meta.Validate(); err != nil {
		return nil, err
	}
	return meta, nil
}

func parseSessionActivityJSON(raw string) (*store.SessionActivityMeta, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var activity store.SessionActivityMeta
	if err := json.Unmarshal([]byte(trimmed), &activity); err != nil {
		return nil, fmt.Errorf("store: parse session activity json: %w", err)
	}
	if err := activity.Validate(); err != nil {
		return nil, fmt.Errorf("store: validate session activity json: %w", err)
	}
	return store.CloneSessionActivityMeta(&activity), nil
}
