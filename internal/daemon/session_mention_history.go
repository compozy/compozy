package daemon

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

// Read bounds for one mentioned session's inline history. Pages grow geometrically so repeated
// assembly stays linear in the events read; the scan stops at whichever bound is reached first.
const (
	mentionHistoryFirstPageEvents = 100
	mentionHistoryScanEvents      = 2000
	mentionHistoryScanBytes       = 4 << 20
)

// readMentionHistory returns the newest user/assistant text messages of sessionID that can fill
// budget characters, at most mentionHistoryMessageLimit of them, oldest first.
//
// It walks the event log backward from the newest event in bounded pages and stops as soon as the
// complete messages already read fill the budget or the message limit, the log start is reached, or
// the scan bounds are spent. The oldest assembled message of a window that does not start at the
// log start may be an assistant reply cut at the page boundary, so it is never returned.
func readMentionHistory(
	ctx context.Context,
	reader nativeSessionPageReader,
	sessionID string,
	budget int,
) ([]transcript.Message, error) {
	if budget <= 0 {
		return nil, nil
	}
	var window []store.SessionEvent
	query := store.EventQuery{Limit: mentionHistoryFirstPageEvents}
	scannedBytes := 0
	for {
		chunk, err := reader.Events(ctx, sessionID, query)
		if err != nil {
			return nil, fmt.Errorf("daemon: read mentioned session %q history: %w", sessionID, err)
		}
		window = append(slices.Clone(chunk), window...)
		for index := range chunk {
			scannedBytes += len(chunk[index].Content)
		}
		atLogStart := len(chunk) < query.Limit
		messages, err := mentionTextMessages(window, atLogStart)
		if err != nil {
			return nil, fmt.Errorf("daemon: assemble mentioned session %q history: %w", sessionID, err)
		}
		if atLogStart || mentionHistoryFilled(messages, budget) ||
			len(window) >= mentionHistoryScanEvents || scannedBytes >= mentionHistoryScanBytes {
			return newestMentionMessages(messages), nil
		}
		query.BeforeSequence = chunk[0].Sequence
		query.Limit = min(2*query.Limit, mentionHistoryScanEvents-len(window))
	}
}

// mentionTextMessages assembles window and keeps the non-empty user/assistant messages. Unless the
// window starts at the log start, a leading assistant message is dropped as possibly partial.
func mentionTextMessages(window []store.SessionEvent, atLogStart bool) ([]transcript.Message, error) {
	messages, err := transcript.Assemble(window)
	if err != nil {
		return nil, err
	}
	if !atLogStart && len(messages) > 0 && messages[0].Role == transcript.RoleAssistant {
		messages = messages[1:]
	}
	kept := make([]transcript.Message, 0, len(messages))
	for _, message := range messages {
		if message.Role != transcript.RoleUser && message.Role != transcript.RoleAssistant {
			continue
		}
		if strings.TrimSpace(message.Content) == "" {
			continue
		}
		kept = append(kept, message)
	}
	return kept, nil
}

func newestMentionMessages(messages []transcript.Message) []transcript.Message {
	if len(messages) > mentionHistoryMessageLimit {
		return messages[len(messages)-mentionHistoryMessageLimit:]
	}
	return messages
}

// mentionHistoryFilled reports whether older messages can no longer change the rendered history.
func mentionHistoryFilled(messages []transcript.Message, budget int) bool {
	if len(messages) >= mentionHistoryMessageLimit {
		return true
	}
	_, used := selectMentionHistory(messages, budget)
	return used >= budget
}

// selectMentionHistory picks the newest messages within budget characters, newest first, and returns
// their rendered entries plus the characters they consume.
func selectMentionHistory(history []transcript.Message, budget int) ([]string, int) {
	selected := make([]string, 0, len(history))
	used := 0
	for _, message := range slices.Backward(history) {
		if used >= budget {
			break
		}
		text, size := truncateMentionText(message.Content, min(mentionHistoryMessageChars, budget-used))
		used += size
		selected = append(selected, "["+string(message.Role)+"] "+text)
	}
	return selected, used
}

// truncateMentionText cuts text to limit characters, appending the truncation suffix when cut, and
// returns the kept text plus the number of content characters it consumed.
func truncateMentionText(text string, limit int) (string, int) {
	text = strings.TrimSpace(text)
	runes := []rune(text)
	if len(runes) <= limit {
		return text, len(runes)
	}
	return string(runes[:limit]) + mentionHistoryTruncatedSuffix, limit
}
