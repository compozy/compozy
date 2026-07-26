package daemon

import (
	"crypto/sha256"
	"encoding/hex"

	"strings"

	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
)

func detachedHarnessActorContext(ownerSessionID string) (taskpkg.ActorContext, error) {
	suffix := strings.TrimSpace(ownerSessionID)
	return taskpkg.DeriveDaemonActorContext(
		harnessDetachedActorRefPrefix+":"+suffix,
		harnessDetachedOriginRefPrefix+"/"+suffix,
	)
}

func detachedHarnessTaskID(ownerSessionID string, submissionKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(ownerSessionID) + "\n" + strings.TrimSpace(submissionKey)))
	return "task-hdet-" + hex.EncodeToString(sum[:8])
}

func detachedHarnessSummary(summary string) string {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return defaultDetachedHarnessSummary
	}
	return trimmed
}

func normalizeDetachedHarnessTurnSource(source session.TurnSource) session.TurnSource {
	switch session.TurnSource(strings.TrimSpace(string(source))) {
	case "", session.TurnSourceUser:
		return session.TurnSourceUser
	case session.TurnSourceNetwork:
		return session.TurnSourceNetwork
	case session.TurnSourceSynthetic:
		return session.TurnSourceSynthetic
	default:
		return session.TurnSourceUser
	}
}
