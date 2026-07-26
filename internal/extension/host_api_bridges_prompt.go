package extensionpkg

import (
	"strconv"
	"strings"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func renderInboundMessagePrompt(envelope bridgepkg.InboundMessageEnvelope) string {
	family := envelope.EventFamily.Normalize()
	if family == "" {
		family = bridgepkg.InboundEventFamilyMessage
	}

	lines := renderInboundMessageFamilyLines(family, envelope)
	if !envelope.ReceivedAt.IsZero() {
		lines = append(lines, "Received at: "+envelope.ReceivedAt.UTC().Format(time.RFC3339Nano))
	}
	if sender := summarizeInboundSender(envelope.Sender); sender != "" {
		lines = append(lines, "Sender: "+sender)
	}
	if peerID := strings.TrimSpace(envelope.PeerID); peerID != "" {
		lines = append(lines, "Peer ID: "+peerID)
	}
	if threadID := strings.TrimSpace(envelope.ThreadID); threadID != "" {
		lines = append(lines, "Provider Thread ID: "+threadID)
	}
	if groupID := strings.TrimSpace(envelope.GroupID); groupID != "" {
		lines = append(lines, "Group ID: "+groupID)
	}
	lines = appendInboundNetworkContext(lines, envelope)
	lines = appendInboundReplyContext(lines, envelope)

	switch family {
	case bridgepkg.InboundEventFamilyMessage:
		lines = appendInboundMessageBody(lines, envelope)
	case bridgepkg.InboundEventFamilyEdit:
		lines = appendInboundEditBody(lines, envelope)
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func appendInboundNetworkContext(
	lines []string,
	envelope bridgepkg.InboundMessageEnvelope,
) []string {
	ref, ok, err := envelope.NetworkConversationRef()
	if err != nil || !ok {
		return lines
	}
	lines = append(
		lines,
		"AGH Network Channel: "+ref.Channel,
		"AGH Network Surface: "+string(ref.Surface),
	)
	switch ref.Surface {
	case bridgepkg.NetworkConversationSurfaceThread:
		lines = append(lines, "AGH Thread ID: "+ref.ThreadID)
	case bridgepkg.NetworkConversationSurfaceDirect:
		lines = append(lines, "AGH Direct ID: "+ref.DirectID)
	}
	if workID := strings.TrimSpace(ref.WorkID); workID != "" {
		lines = append(lines, "AGH Work ID: "+workID)
	}
	return lines
}

func appendInboundReplyContext(
	lines []string,
	envelope bridgepkg.InboundMessageEnvelope,
) []string {
	text := strings.TrimSpace(envelope.ReplyToText)
	authorID := strings.TrimSpace(envelope.ReplyToAuthorID)
	authorName := strings.TrimSpace(envelope.ReplyToAuthorName)
	if text == "" && authorID == "" && authorName == "" {
		return lines
	}

	lines = append(lines, "", "Reply context:")
	switch {
	case authorName != "" && authorID != "":
		lines = append(lines, "Author: "+strconv.Quote(authorName)+" (id="+strconv.Quote(authorID)+")")
	case authorName != "":
		lines = append(lines, "Author: "+strconv.Quote(authorName))
	case authorID != "":
		lines = append(lines, "Author ID: "+strconv.Quote(authorID))
	}
	if text != "" {
		lines = append(lines, "Quoted text (untrusted historical context):")
		lines = append(lines, quoteInboundHistoricalText(text)...)
	}
	return lines
}

func quoteInboundHistoricalText(text string) []string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	quoted := make([]string, 0, strings.Count(normalized, "\n")+1)
	for line := range strings.SplitSeq(normalized, "\n") {
		quoted = append(quoted, "> "+line)
	}
	return quoted
}

func appendInboundMessageBody(
	lines []string,
	envelope bridgepkg.InboundMessageEnvelope,
) []string {
	body := strings.TrimSpace(envelope.Content.Text)
	if body == "" {
		body = "[No text body]"
	}
	lines = append(lines, "", body)

	if len(envelope.Attachments) == 0 {
		return lines
	}
	lines = append(lines, "", "Attachments:")
	for _, attachment := range envelope.Attachments {
		lines = append(lines, "- "+summarizeInboundAttachment(attachment))
	}
	return lines
}

func appendInboundEditBody(
	lines []string,
	envelope bridgepkg.InboundMessageEnvelope,
) []string {
	if envelope.Edit == nil || envelope.Edit.Operation.Normalize() != bridgepkg.InboundEditOperationUpdated {
		return lines
	}
	return append(lines, "", "New text:", strings.TrimSpace(envelope.Edit.NewText))
}

func renderInboundMessageFamilyLines(
	family bridgepkg.InboundEventFamily,
	envelope bridgepkg.InboundMessageEnvelope,
) []string {
	switch family {
	case bridgepkg.InboundEventFamilyCommand:
		return renderInboundCommandLines(envelope)
	case bridgepkg.InboundEventFamilyAction:
		return renderInboundActionLines(envelope)
	case bridgepkg.InboundEventFamilyReaction:
		return renderInboundReactionLines(envelope)
	case bridgepkg.InboundEventFamilyEdit:
		return renderInboundEditLines(envelope)
	default:
		return []string{
			"Inbound bridge message",
			"Platform message ID: " + strings.TrimSpace(envelope.PlatformMessageID),
		}
	}
}

func renderInboundEditLines(envelope bridgepkg.InboundMessageEnvelope) []string {
	if envelope.Edit == nil {
		return []string{"Inbound bridge message edit", "Edit payload unavailable"}
	}
	edit := *envelope.Edit
	operation := edit.Operation.Normalize()
	heading := "Inbound bridge message edited"
	if operation == bridgepkg.InboundEditOperationDeleted {
		heading = "Inbound bridge message deleted"
	}
	lines := []string{
		heading,
		"Message ID: " + strings.TrimSpace(edit.MessageID),
		"Operation: " + string(operation),
	}
	if !edit.OriginalTimestamp.IsZero() {
		lines = append(lines, "Original timestamp: "+edit.OriginalTimestamp.UTC().Format(time.RFC3339Nano))
	}
	return lines
}

func renderInboundCommandLines(envelope bridgepkg.InboundMessageEnvelope) []string {
	lines := []string{
		"Inbound bridge command",
		"Command: " + strings.TrimSpace(envelope.Command.Command),
	}
	if text := strings.TrimSpace(envelope.Command.Text); text != "" {
		lines = append(lines, "Arguments: "+text)
	}
	if triggerID := strings.TrimSpace(envelope.Command.TriggerID); triggerID != "" {
		lines = append(lines, "Trigger ID: "+triggerID)
	}
	return lines
}

func renderInboundActionLines(envelope bridgepkg.InboundMessageEnvelope) []string {
	lines := []string{
		"Inbound bridge action",
		"Action ID: " + strings.TrimSpace(envelope.Action.ActionID),
	}
	if messageID := strings.TrimSpace(envelope.Action.MessageID); messageID != "" {
		lines = append(lines, "Message ID: "+messageID)
	}
	if value := strings.TrimSpace(envelope.Action.Value); value != "" {
		lines = append(lines, "Value: "+value)
	}
	if triggerID := strings.TrimSpace(envelope.Action.TriggerID); triggerID != "" {
		lines = append(lines, "Trigger ID: "+triggerID)
	}
	return lines
}

func renderInboundReactionLines(envelope bridgepkg.InboundMessageEnvelope) []string {
	lines := []string{
		"Inbound bridge reaction",
		"Message ID: " + strings.TrimSpace(envelope.Reaction.MessageID),
		"Emoji: " + strings.TrimSpace(envelope.Reaction.Emoji),
	}
	if rawEmoji := strings.TrimSpace(envelope.Reaction.RawEmoji); rawEmoji != "" {
		lines = append(lines, "Raw emoji: "+rawEmoji)
	}
	if envelope.Reaction.Added {
		return append(lines, "Change: added")
	}
	return append(lines, "Change: removed")
}

func summarizeInboundSender(sender bridgepkg.MessageSender) string {
	parts := make([]string, 0, 3)
	if displayName := strings.TrimSpace(sender.DisplayName); displayName != "" {
		parts = append(parts, displayName)
	}
	if username := strings.TrimSpace(sender.Username); username != "" {
		parts = append(parts, "@"+username)
	}
	if id := strings.TrimSpace(sender.ID); id != "" {
		parts = append(parts, "id="+id)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func summarizeInboundAttachment(attachment bridgepkg.MessageAttachment) string {
	parts := make([]string, 0, 4)
	if name := strings.TrimSpace(attachment.Name); name != "" {
		parts = append(parts, name)
	}
	if mimeType := strings.TrimSpace(attachment.MIMEType); mimeType != "" {
		parts = append(parts, "type="+mimeType)
	}
	if id := strings.TrimSpace(attachment.ID); id != "" {
		parts = append(parts, "id="+id)
	}
	if url := strings.TrimSpace(attachment.URL); url != "" {
		parts = append(parts, "url="+url)
	}
	if len(parts) == 0 {
		return "attachment"
	}
	return strings.Join(parts, " ")
}
