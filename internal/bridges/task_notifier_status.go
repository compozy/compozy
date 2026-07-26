package bridges

import (
	"fmt"

	"strings"

	"unicode/utf8"

	taskpkg "github.com/compozy/agh/internal/task"
)

func isTerminalTaskNotificationCandidate(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case terminalTaskEventRunCompleted,
		terminalTaskEventRunFailed,
		terminalTaskEventRunCanceled,
		terminalTaskEventCanceled,
		terminalTaskEventRunReviewApproved:
		return true
	default:
		return false
	}
}

func taskStatusForTerminalEvent(eventType string) (taskpkg.Status, bool) {
	switch strings.TrimSpace(eventType) {
	case terminalTaskEventRunCompleted, terminalTaskEventRunReviewApproved:
		return taskpkg.TaskStatusCompleted, true
	case terminalTaskEventRunFailed:
		return taskpkg.TaskStatusFailed, true
	case terminalTaskEventRunCanceled, terminalTaskEventCanceled:
		return taskpkg.TaskStatusCanceled, true
	default:
		return "", false
	}
}

func taskStatusForRunStatus(status taskpkg.RunStatus) (taskpkg.Status, bool) {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusCompleted:
		return taskpkg.TaskStatusCompleted, true
	case taskpkg.TaskRunStatusFailed:
		return taskpkg.TaskStatusFailed, true
	case taskpkg.TaskRunStatusCanceled:
		return taskpkg.TaskStatusCanceled, true
	default:
		return "", false
	}
}

func isTaskTerminalStatus(status taskpkg.Status) bool {
	switch status.Normalize() {
	case taskpkg.TaskStatusCompleted, taskpkg.TaskStatusFailed, taskpkg.TaskStatusCanceled:
		return true
	default:
		return false
	}
}

func terminalTaskNotificationDeliveryID(subscriptionID string, sequence int64) string {
	return fmt.Sprintf("notif:%s:%d", strings.TrimSpace(subscriptionID), sequence)
}

func terminalTaskNotificationText(notification TerminalTaskNotification) string {
	status := notification.Status.Normalize()
	if notification.Error != "" {
		return fmt.Sprintf("Task %s finished as %s: %s", notification.TaskID, status, notification.Error)
	}
	return fmt.Sprintf("Task %s finished as %s", notification.TaskID, status)
}

func truncateTerminalTaskCursorError(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= maxTerminalTaskCursorErrorBytes {
		return trimmed
	}
	cut := maxTerminalTaskCursorErrorBytes
	for cut > 0 && !utf8.ValidString(trimmed[:cut]) {
		cut--
	}
	return trimmed[:cut]
}
