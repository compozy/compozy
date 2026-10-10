package cli

import (
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func sessionReplyWatchesHuman(watches []contract.ReplyWatchPayload) string {
	if len(watches) == 0 {
		return ""
	}
	rows := make([][]string, 0, len(watches))
	for _, w := range watches {
		rows = append(
			rows,
			[]string{w.ID, w.TargetSessionID, w.MessageID, w.State, w.CreatedAt.UTC().Format("15:04:05")},
		)
	}
	return renderHumanTable("Waiting for replies", []string{"WATCH", "TARGET", "MESSAGE", "STATE", "SINCE"}, rows)
}

func sessionReplyWatchesToon(watches []contract.ReplyWatchPayload) string {
	var rows []string
	for _, w := range watches {
		rows = append(
			rows,
			renderToonObject(
				"reply_watch",
				[]string{"id", "target_session_id", cliOutputMessageIDKey, stateKey, "created_at"},
				[]string{w.ID, w.TargetSessionID, w.MessageID, w.State, formatTime(w.CreatedAt)},
			),
		)
	}
	return strings.Join(rows, "\n")
}
