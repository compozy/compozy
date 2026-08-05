package task

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
)

const wakeSummaryMaxRunes = 512

func terminalWakeSummary(taskRecord Task, run Run) string {
	label := wakeTaskLabel(taskRecord)
	switch run.Status.Normalize() {
	case TaskRunStatusCompleted:
		return boundWakeSummary(fmt.Sprintf("Task %s completed.", label))
	case TaskRunStatusFailed:
		if !isTerminalTaskStatus(taskRecord.Status.Normalize()) {
			if errText := strings.TrimSpace(run.Error); errText != "" {
				return boundWakeSummary(fmt.Sprintf("Run for task %s failed: %s", label, errText))
			}
			return boundWakeSummary(fmt.Sprintf("Run for task %s failed.", label))
		}
		if errText := strings.TrimSpace(run.Error); errText != "" {
			return boundWakeSummary(fmt.Sprintf("Task %s failed: %s", label, errText))
		}
		return boundWakeSummary(fmt.Sprintf("Task %s failed.", label))
	case TaskRunStatusCanceled:
		if !isTerminalTaskStatus(taskRecord.Status.Normalize()) {
			return boundWakeSummary(fmt.Sprintf("Run for task %s was canceled.", label))
		}
		return boundWakeSummary(fmt.Sprintf("Task %s was canceled.", label))
	default:
		return boundWakeSummary(fmt.Sprintf("Task %s reached %s.", label, run.Status.Normalize()))
	}
}

func blockedWakeSummary(taskRecord Task, block TaskBlock) string {
	label := wakeTaskLabel(taskRecord)
	reason := strings.TrimSpace(block.Reason)
	if reason == "" {
		return boundWakeSummary(fmt.Sprintf("Task %s is blocked.", label))
	}
	return boundWakeSummary(fmt.Sprintf("Task %s is blocked: %s", label, reason))
}

func needsAttentionWakeSummary(taskRecord Task, reason string) string {
	label := wakeTaskLabel(taskRecord)
	trimmedReason := strings.TrimSpace(reason)
	if trimmedReason == "" {
		return boundWakeSummary(fmt.Sprintf("Task %s needs attention.", label))
	}
	return boundWakeSummary(fmt.Sprintf("Task %s needs attention: %s", label, trimmedReason))
}

func wakeTaskLabel(taskRecord Task) string {
	title := strings.TrimSpace(taskRecord.Title)
	if title != "" {
		return fmt.Sprintf("%q", title)
	}
	id := strings.TrimSpace(taskRecord.ID)
	if id != "" {
		return id
	}
	return "task"
}

func wakeEventID(parts ...string) string {
	cleaned := make([]string, 0, len(parts)+1)
	cleaned = append(cleaned, "wake")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		trimmed = strings.ReplaceAll(trimmed, " ", "_")
		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, "-")
}

func wakeEventKey(taskID string, wakeEventID string) string {
	trimmedTaskID := strings.TrimSpace(taskID)
	trimmedWakeEventID := strings.TrimSpace(wakeEventID)
	if trimmedTaskID == "" || trimmedWakeEventID == "" {
		return ""
	}
	return trimmedTaskID + "/" + trimmedWakeEventID
}

func boundWakeSummary(value string) string {
	redacted := strings.TrimSpace(diagnostics.Redact(RedactClaimTokens(value)))
	if redacted == "" {
		return ""
	}
	runes := []rune(redacted)
	if len(runes) <= wakeSummaryMaxRunes {
		return redacted
	}
	if wakeSummaryMaxRunes <= 3 {
		return string(runes[:wakeSummaryMaxRunes])
	}
	return string(runes[:wakeSummaryMaxRunes-3]) + "..."
}
