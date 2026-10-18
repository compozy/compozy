package globaldb

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func sessionLivenessPID(meta *store.SessionLivenessMeta) int {
	if meta == nil {
		return 0
	}
	if meta.SubprocessPID < 0 {
		return 0
	}
	return meta.SubprocessPID
}

func sessionLivenessStartedAt(meta *store.SessionLivenessMeta) any {
	if meta == nil || meta.SubprocessStartedAt == nil || meta.SubprocessStartedAt.IsZero() {
		return nil
	}
	return store.FormatTimestamp(meta.SubprocessStartedAt.UTC())
}

func sessionLivenessLastUpdateAt(meta *store.SessionLivenessMeta) any {
	if meta == nil || meta.LastUpdateAt == nil || meta.LastUpdateAt.IsZero() {
		return nil
	}
	return store.FormatTimestamp(meta.LastUpdateAt.UTC())
}

func sessionLivenessStallState(meta *store.SessionLivenessMeta) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta.StallState)
}

func sessionLivenessStallReason(meta *store.SessionLivenessMeta) string {
	if meta == nil {
		return ""
	}
	return strings.TrimSpace(meta.StallReason)
}

func sessionLivenessActivityJSON(meta *store.SessionLivenessMeta) (string, error) {
	if meta == nil || meta.Activity == nil {
		return "", nil
	}
	activity := store.CloneSessionActivityMeta(meta.Activity)
	data, err := json.Marshal(activity)
	if err != nil {
		return "", fmt.Errorf("store: session liveness activity marshal: %w", err)
	}
	return string(data), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}
