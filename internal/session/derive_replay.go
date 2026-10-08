package session

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	deriveContinueLine  = "This session continues %s (agent: %s). "
	deriveForkLine      = "This session was forked from %s through message %s (agent: %s). "
	deriveForkWholeLine = "This session was forked from %s (agent: %s). "
	deriveWorkspaceLine = "The files and git state in the workspace are authoritative; " +
		"inspect them before acting on the transcript. " +
		"Attachments listed in the transcript were not copied to this session."
	deriveOmittedFmt    = "[%d earlier messages omitted to fit the context budget]"
	deriveTruncatedMark = " [content truncated]"
	deriveAbortedNote   = "<compozy_turn_aborted>The source session was still working on a later turn " +
		"when this session was created. That turn is not included; any tools it ran may have " +
		"partially executed.</compozy_turn_aborted>"

	deriveOmittedMessageID = "sys_omitted"
	deriveAbortedMessageID = "sys_turn_aborted"
	// deriveMessageCapPasses bounds the field-trimming loop of one message.
	deriveMessageCapPasses = 64
)

// replayBudget bounds one carried context.
type replayBudget struct {
	// MaxBytes is the hard bound on the serialized message array.
	MaxBytes int
	// MaxMessageBytes caps each serialized message.
	MaxMessageBytes int
	// PinFirstUser preserves the original user request before the omission note.
	PinFirstUser bool
}

// replayStats reports what bounding kept and dropped.
type replayStats struct {
	MessageCount    int
	Bytes           int
	OmittedCount    int
	Truncated       bool
	FirstUserPinned bool
}

// boundReplay preserves the newest fitting suffix, including the omission note and first-user pin.
func boundReplay(messages []transcript.Message, budget replayBudget) ([]transcript.Message, replayStats) {
	capped := make([]transcript.Message, len(messages))
	sizes := make([]int, len(messages))
	truncated := false
	for index := range messages {
		var cut bool
		capped[index], cut = capReplayMessage(messages[index], budget.MaxMessageBytes)
		truncated = truncated || cut
		sizes[index] = replayMessageBytes(capped[index])
	}
	firstUser := -1
	if budget.PinFirstUser {
		firstUser = slices.IndexFunc(capped, func(message transcript.Message) bool {
			return message.Role == transcript.RoleUser
		})
	}
	tailBytes := 0
	for _, size := range sizes {
		tailBytes += size
	}
	start := 0
	for start < len(capped)-1 && replaySuffixBytes(capped, sizes, tailBytes, firstUser, start) > budget.MaxBytes {
		tailBytes -= sizes[start]
		start++
	}
	kept, omitted, pinned := composeReplaySuffix(capped, firstUser, start)
	if len(capped) > 0 && replayArrayBytes(kept) > budget.MaxBytes {
		// Only the original request and newest message remain. Trim those only
		// after every older tail message has been dropped.
		trimIndices := []int{start}
		if pinned && firstUser != start {
			trimIndices = append(trimIndices, firstUser)
		}
		for _, index := range trimIndices {
			excess := replayArrayBytes(kept) - budget.MaxBytes
			if excess <= 0 {
				break
			}
			capped[index], _ = capReplayMessage(capped[index], max(sizes[index]-excess, 1))
			truncated = true
			kept, omitted, pinned = composeReplaySuffix(capped, firstUser, start)
		}
	}
	if replayArrayBytes(kept) > budget.MaxBytes && firstUser >= 0 {
		// If even two minimal message identities cannot fit beside the note,
		// retain the original request and drop the newest message as well.
		start = len(capped)
		kept, omitted, pinned = composeReplaySuffix(capped, firstUser, start)
		if excess := replayArrayBytes(kept) - budget.MaxBytes; excess > 0 {
			capped[firstUser], _ = capReplayMessage(
				capped[firstUser],
				max(replayMessageBytes(capped[firstUser])-excess, 1),
			)
			kept, omitted, pinned = composeReplaySuffix(capped, firstUser, start)
		}
	}
	if replayArrayBytes(kept) > budget.MaxBytes {
		kept, omitted, pinned = []transcript.Message{}, len(messages), false
	}
	return kept, replayStats{
		MessageCount: len(messages) - omitted,
		Bytes:        replayArrayBytes(kept), OmittedCount: omitted,
		Truncated: truncated || omitted > 0, FirstUserPinned: pinned,
	}
}

func replaySuffixBytes(capped []transcript.Message, sizes []int, tailBytes, firstUser, start int) int {
	count, omitted := len(capped)-start, start
	bytes := 2 + tailBytes
	if firstUser >= 0 && firstUser < start {
		bytes += sizes[firstUser]
		count++
		omitted--
	}
	if omitted > 0 {
		bytes += replayMessageBytes(omittedReplayNote(omitted, capped[start-1].Timestamp))
		count++
	}
	return bytes + max(count-1, 0)
}

func composeReplaySuffix(capped []transcript.Message, firstUser, start int) ([]transcript.Message, int, bool) {
	pinned := firstUser >= 0 && start > 0
	omitted := start
	kept := make([]transcript.Message, 0, len(capped)-start+2)
	if pinned {
		kept = append(kept, capped[firstUser])
		if firstUser < start {
			omitted--
		}
	}
	if omitted > 0 {
		kept = append(kept, omittedReplayNote(omitted, capped[start-1].Timestamp))
	}
	for index := start; index < len(capped); index++ {
		if !pinned || index != firstUser {
			kept = append(kept, capped[index])
		}
	}
	return kept, omitted, pinned && omitted > 0
}

func omittedReplayNote(omitted int, timestamp time.Time) transcript.Message {
	return transcript.Message{
		ID:        deriveOmittedMessageID,
		Role:      transcript.RoleSystem,
		Content:   fmt.Sprintf(deriveOmittedFmt, omitted),
		Timestamp: timestamp,
	}
}

func abortedReplayNote(timestamp time.Time) transcript.Message {
	return transcript.Message{
		ID:        deriveAbortedMessageID,
		Role:      transcript.RoleSystem,
		Content:   deriveAbortedNote,
		Timestamp: timestamp,
	}
}

func replayMessageBytes(message transcript.Message) int {
	encoded, err := json.Marshal(message)
	if err != nil {
		return 0
	}
	return len(encoded)
}

func replayArrayBytes(messages []transcript.Message) int {
	if messages == nil {
		messages = []transcript.Message{}
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		return 0
	}
	return len(encoded)
}

// capReplayMessage trims the largest text fields of one message (marking them with
// " [content truncated]") and drops raw JSON payloads until the serialized message
// fits limit. A message that cannot fit keeps only its identity and the mark.
func capReplayMessage(message transcript.Message, limit int) (transcript.Message, bool) {
	if limit <= 0 || replayMessageBytes(message) <= limit {
		return message, false
	}
	capped := cloneReplayMessage(message)
	for range deriveMessageCapPasses {
		size := replayMessageBytes(capped)
		if size <= limit {
			return capped, true
		}
		if !trimLargestReplayField(&capped, size-limit) {
			break
		}
	}
	if replayMessageBytes(capped) <= limit {
		return capped, true
	}
	return transcript.Message{
		ID: message.ID, Role: message.Role, Content: strings.TrimSpace(deriveTruncatedMark),
		Timestamp: message.Timestamp,
	}, true
}

func cloneReplayMessage(message transcript.Message) transcript.Message {
	cloned := message
	cloned.Attachments = append(cloned.Attachments[:0:0], message.Attachments...)
	cloned.ToolInput = append(json.RawMessage(nil), message.ToolInput...)
	if message.ToolResult != nil {
		result := *message.ToolResult
		result.StructuredPatch = append(json.RawMessage(nil), message.ToolResult.StructuredPatch...)
		result.RawOutput = append(json.RawMessage(nil), message.ToolResult.RawOutput...)
		cloned.ToolResult = &result
	}
	return cloned
}

// trimLargestReplayField shrinks the largest remaining field by at least excess bytes.
func trimLargestReplayField(message *transcript.Message, excess int) bool {
	type field struct {
		text *string
		raw  *json.RawMessage
		size int
	}
	fields := []field{{text: &message.Content}, {text: &message.Thinking}, {raw: &message.ToolInput}}
	if result := message.ToolResult; result != nil {
		fields = append(fields,
			field{text: &result.Stdout}, field{text: &result.Stderr}, field{text: &result.Content},
			field{text: &result.Error}, field{text: &result.FilePath},
			field{raw: &result.StructuredPatch}, field{raw: &result.RawOutput},
		)
	}
	best := -1
	for index := range fields {
		if fields[index].text != nil {
			fields[index].size = len(strings.TrimSuffix(*fields[index].text, deriveTruncatedMark))
		} else {
			fields[index].size = len(*fields[index].raw)
		}
		if fields[index].size > 0 && (best < 0 || fields[index].size > fields[best].size) {
			best = index
		}
	}
	if best < 0 {
		return false
	}
	target := fields[best]
	if target.raw != nil {
		*target.raw = nil
		if !strings.HasSuffix(message.Content, deriveTruncatedMark) {
			message.Content += deriveTruncatedMark
		}
		return true
	}
	body := strings.TrimSuffix(*target.text, deriveTruncatedMark)
	keep := max(len(body)-excess-len(deriveTruncatedMark)-8, 0)
	*target.text = truncateReplayUTF8(body, keep) + deriveTruncatedMark
	return true
}

func truncateReplayUTF8(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	truncated := value[:maxBytes]
	for !utf8.ValidString(truncated) {
		truncated = truncated[:len(truncated)-1]
	}
	return truncated
}

// importedContextHeader renders the framing lines that open the carried context.
func importedContextHeader(ic store.SessionImportedContext) string {
	var line string
	switch {
	case ic.Kind == store.LineageKindFork && strings.TrimSpace(ic.OriginMessageID) != "":
		line = fmt.Sprintf(deriveForkLine, ic.SourceSessionID, ic.OriginMessageID, ic.OriginAgentName)
	case ic.Kind == store.LineageKindFork:
		line = fmt.Sprintf(deriveForkWholeLine, ic.SourceSessionID, ic.OriginAgentName)
	default:
		line = fmt.Sprintf(deriveContinueLine, ic.SourceSessionID, ic.OriginAgentName)
	}
	return contextRebuiltMarkerSummary + "\n" + line + resumeReplayInstruction + " " + deriveWorkspaceLine
}

// renderImportedReplayBlock composes the framing lines and one fenced transcript array.
func renderImportedReplayBlock(ic store.SessionImportedContext, historyPointer string, messagesJSON string) string {
	parts := []string{importedContextHeader(ic)}
	if trimmed := strings.TrimSpace(historyPointer); trimmed != "" {
		parts = append(parts, trimmed)
	}
	parts = append(parts, resumeReplayOpenTag+"\n"+messagesJSON+"\n"+resumeReplayCloseTag)
	return strings.Join(parts, "\n")
}

// composeImportedContextBlock renders a child's imported context on its own.
func composeImportedContextBlock(ic store.SessionImportedContext) string {
	return renderImportedReplayBlock(ic, "", ic.MessagesJSON)
}

func decodeImportedMessages(ic *store.SessionImportedContext) ([]transcript.Message, error) {
	if ic == nil || strings.TrimSpace(ic.MessagesJSON) == "" {
		return nil, nil
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(ic.MessagesJSON), &messages); err != nil {
		return nil, fmt.Errorf("session: decode imported context of %q: %w", ic.SourceSessionID, err)
	}
	return messages, nil
}
