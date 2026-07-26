package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/transcript"
)

type networkCorrelationExpectation struct {
	MessageID       string
	Kind            string
	Surface         string
	ThreadID        string
	DirectID        string
	WorkID          string
	ReplyTo         string
	TraceID         string
	CausationID     string
	Trust           string
	TranscriptFrom  string
	PeerFrom        *string
	PeerTo          *string
	AuditDirections []string
}

type networkAuditExpectation struct {
	MessageID string
	Direction string
	Kind      string
	Surface   *string
	ThreadID  *string
	DirectID  *string
	WorkID    *string
	PeerFrom  *string
	PeerTo    *string
	Reason    string
}

func validateNetworkCorrelationSurfaces(
	messages []transcript.UIMessage,
	audit []store.NetworkAuditEntry,
	expectation networkCorrelationExpectation,
) error {
	checks := []struct {
		label  string
		needle string
	}{
		{label: "message id", needle: attributeNeedle("id", expectation.MessageID)},
		{label: "kind", needle: attributeNeedle("kind", expectation.Kind)},
		{label: "surface", needle: attributeNeedle("surface", expectation.Surface)},
		{label: "thread-id", needle: attributeNeedle("thread-id", expectation.ThreadID)},
		{label: "direct-id", needle: attributeNeedle("direct-id", expectation.DirectID)},
		{label: "work-id", needle: attributeNeedle("work-id", expectation.WorkID)},
		{label: "reply-to", needle: attributeNeedle("reply-to", expectation.ReplyTo)},
		{label: "trace-id", needle: attributeNeedle("trace-id", expectation.TraceID)},
		{label: "causation-id", needle: attributeNeedle("causation-id", expectation.CausationID)},
		{label: "trust", needle: attributeNeedle("trust", expectation.Trust)},
	}

	matched := false
	for _, message := range messages {
		content := strings.TrimSpace(transcript.UIMessageText(message))
		if content == "" {
			continue
		}

		allPresent := true
		for _, check := range checks {
			if check.needle != "" && !strings.Contains(content, check.needle) {
				allPresent = false
				break
			}
		}
		if allPresent {
			matched = true
			break
		}
	}
	if !matched {
		matched = transcriptHasNetworkWakeMessage(
			messages,
			expectation.MessageID,
			expectation.Kind,
			expectation.TranscriptFrom,
		)
	}
	if !matched {
		return fmt.Errorf("transcript missing correlated attributes for message %q", expectation.MessageID)
	}

	for _, direction := range expectation.AuditDirections {
		if err := validateNetworkAuditEntry(audit, networkAuditExpectation{
			MessageID: expectation.MessageID,
			Direction: direction,
			Kind:      expectation.Kind,
			Surface:   auditFieldValue(expectation.Surface),
			ThreadID:  auditFieldValue(expectation.ThreadID),
			DirectID:  auditFieldValue(expectation.DirectID),
			WorkID:    auditFieldValue(expectation.WorkID),
			PeerFrom:  expectation.PeerFrom,
			PeerTo:    expectation.PeerTo,
		}); err != nil {
			return err
		}
	}

	return nil
}

func transcriptHasNetworkWakeMessage(
	messages []transcript.UIMessage,
	messageID string,
	kind string,
	from string,
) bool {
	type wakeMessage struct {
		MessageID string `json:"message_id"`
		From      string `json:"from"`
		Kind      string `json:"kind"`
	}
	for _, message := range messages {
		for line := range strings.SplitSeq(transcript.UIMessageText(message), "\n") {
			var candidate wakeMessage
			if json.Unmarshal([]byte(strings.TrimSpace(line)), &candidate) != nil {
				continue
			}
			if strings.TrimSpace(candidate.MessageID) != strings.TrimSpace(messageID) ||
				strings.TrimSpace(candidate.Kind) != strings.TrimSpace(kind) {
				continue
			}
			if strings.TrimSpace(from) != "" && strings.TrimSpace(candidate.From) != strings.TrimSpace(from) {
				continue
			}
			return true
		}
	}
	return false
}

func validateNetworkAuditEntry(
	entries []store.NetworkAuditEntry,
	expectation networkAuditExpectation,
) error {
	for _, entry := range entries {
		if strings.TrimSpace(entry.MessageID) != strings.TrimSpace(expectation.MessageID) {
			continue
		}
		if strings.TrimSpace(entry.Direction) != strings.TrimSpace(expectation.Direction) {
			continue
		}
		if strings.TrimSpace(entry.Kind) != strings.TrimSpace(expectation.Kind) {
			continue
		}
		if !optionalAuditFieldMatches(expectation.Surface, entry.Surface) {
			continue
		}
		if !optionalAuditFieldMatches(expectation.ThreadID, entry.ThreadID) {
			continue
		}
		if !optionalAuditFieldMatches(expectation.DirectID, entry.DirectID) {
			continue
		}
		if !optionalAuditFieldMatches(expectation.WorkID, entry.WorkID) {
			continue
		}
		if !optionalAuditFieldMatches(expectation.PeerFrom, entry.PeerFrom) {
			continue
		}
		if !optionalAuditFieldMatches(expectation.PeerTo, entry.PeerTo) {
			continue
		}
		if trimmedReason := strings.TrimSpace(expectation.Reason); trimmedReason != "" &&
			strings.TrimSpace(entry.Reason) != trimmedReason {
			continue
		}
		return nil
	}

	return fmt.Errorf(
		"audit missing message_id=%q direction=%q kind=%q surface=%q thread_id=%q direct_id=%q work_id=%q reason=%q",
		expectation.MessageID,
		expectation.Direction,
		expectation.Kind,
		auditExpectationValue(expectation.Surface),
		auditExpectationValue(expectation.ThreadID),
		auditExpectationValue(expectation.DirectID),
		auditExpectationValue(expectation.WorkID),
		expectation.Reason,
	)
}

func optionalAuditFieldMatches(want *string, got string) bool {
	if want == nil {
		return true
	}
	trimmedWant := strings.TrimSpace(*want)
	if trimmedWant == "" {
		return strings.TrimSpace(got) == ""
	}
	return strings.TrimSpace(got) == trimmedWant
}

func auditFieldValue(value string) *string {
	cloned := value
	return &cloned
}

func auditExpectationValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func emptyAuditField() *string {
	return auditFieldValue("")
}

func attributeNeedle(name string, value string) string {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return ""
	}
	return name + `="` + trimmedValue + `"`
}

func TestValidateNetworkCorrelationSurfacesUsesTargetedAttributes(t *testing.T) {
	t.Parallel()

	t.Run("Should use targeted transcript attributes", func(t *testing.T) {
		t.Parallel()

		messages := []transcript.UIMessage{
			{
				Role: transcript.UIRoleAssistant,
				Parts: []transcript.UIMessagePart{
					{
						Type:  "text",
						Text:  `<network-message id="msg_direct_01" kind="say" surface="direct" direct-id="direct_test_01" work-id="work_patch_42" reply-to="msg_say_01" trace-id="trace_ops_patch_42" causation-id="msg_say_01" trust="untrusted"></network-message>`,
						State: "done",
					},
				},
			},
		}
		audit := []store.NetworkAuditEntry{
			{
				MessageID: "msg_direct_01",
				Direction: "sent",
				Kind:      "say",
				Surface:   "direct",
				DirectID:  "direct_test_01",
				WorkID:    "work_patch_42",
			},
			{
				MessageID: "msg_direct_01",
				Direction: "delivered",
				Kind:      "say",
				Surface:   "direct",
				DirectID:  "direct_test_01",
				WorkID:    "work_patch_42",
			},
		}

		if err := validateNetworkCorrelationSurfaces(messages, audit, networkCorrelationExpectation{
			MessageID:       "msg_direct_01",
			Kind:            "say",
			Surface:         "direct",
			DirectID:        "direct_test_01",
			WorkID:          "work_patch_42",
			ReplyTo:         "msg_say_01",
			TraceID:         "trace_ops_patch_42",
			CausationID:     "msg_say_01",
			Trust:           "untrusted",
			AuditDirections: []string{"sent", "delivered"},
		}); err != nil {
			t.Fatalf("validateNetworkCorrelationSurfaces() error = %v", err)
		}
	})
}

func TestValidateNetworkCorrelationSurfacesRejectsSplitTranscriptMatches(t *testing.T) {
	t.Parallel()

	t.Run("Should reject split transcript matches", func(t *testing.T) {
		t.Parallel()

		messages := []transcript.UIMessage{
			{
				Role: transcript.UIRoleAssistant,
				Parts: []transcript.UIMessagePart{
					{
						Type:  "text",
						Text:  `<network-message id="msg_direct_01" kind="say" surface="direct" direct-id="direct_test_01" causation-id="msg_say_01" trust="untrusted"></network-message>`,
						State: "done",
					},
				},
			},
			{
				Role: transcript.UIRoleAssistant,
				Parts: []transcript.UIMessagePart{
					{
						Type:  "text",
						Text:  `<network-message work-id="work_patch_42" reply-to="msg_say_01" trace-id="trace_ops_patch_42"></network-message>`,
						State: "done",
					},
				},
			},
		}
		audit := []store.NetworkAuditEntry{
			{
				MessageID: "msg_direct_01",
				Direction: "sent",
				Kind:      "say",
				Surface:   "direct",
				DirectID:  "direct_test_01",
				WorkID:    "work_patch_42",
			},
			{
				MessageID: "msg_direct_01",
				Direction: "delivered",
				Kind:      "say",
				Surface:   "direct",
				DirectID:  "direct_test_01",
				WorkID:    "work_patch_42",
			},
		}

		if err := validateNetworkCorrelationSurfaces(messages, audit, networkCorrelationExpectation{
			MessageID:       "msg_direct_01",
			Kind:            "say",
			Surface:         "direct",
			DirectID:        "direct_test_01",
			WorkID:          "work_patch_42",
			ReplyTo:         "msg_say_01",
			TraceID:         "trace_ops_patch_42",
			CausationID:     "msg_say_01",
			Trust:           "untrusted",
			AuditDirections: []string{"sent", "delivered"},
		}); err == nil {
			t.Fatal("validateNetworkCorrelationSurfaces() error = nil, want split-message correlation failure")
		}
	})
}

func TestValidateNetworkAuditEntryMatchesDuplicateRejection(t *testing.T) {
	t.Parallel()

	t.Run("Should match duplicate rejection entries", func(t *testing.T) {
		t.Parallel()

		entries := []store.NetworkAuditEntry{
			{
				MessageID: "msg_direct_01",
				Direction: "rejected",
				Kind:      "say",
				Surface:   "direct",
				DirectID:  "direct_test_01",
				WorkID:    "work_patch_42",
				Reason:    "duplicate",
			},
		}

		if err := validateNetworkAuditEntry(entries, networkAuditExpectation{
			MessageID: "msg_direct_01",
			Direction: "rejected",
			Kind:      "say",
			Surface:   auditFieldValue("direct"),
			ThreadID:  emptyAuditField(),
			DirectID:  auditFieldValue("direct_test_01"),
			WorkID:    auditFieldValue("work_patch_42"),
			Reason:    "duplicate",
		}); err != nil {
			t.Fatalf("validateNetworkAuditEntry() error = %v", err)
		}
	})
}

func TestValidateNetworkAuditEntryRejectsWrongContainer(t *testing.T) {
	t.Parallel()

	t.Run("Should reject audit entries for the wrong container", func(t *testing.T) {
		t.Parallel()

		entries := []store.NetworkAuditEntry{
			{
				MessageID: "msg_direct_01",
				Direction: "delivered",
				Kind:      "say",
				Surface:   "direct",
				DirectID:  "direct_wrong",
				WorkID:    "work_patch_42",
			},
		}

		if err := validateNetworkAuditEntry(entries, networkAuditExpectation{
			MessageID: "msg_direct_01",
			Direction: "delivered",
			Kind:      "say",
			Surface:   auditFieldValue("direct"),
			ThreadID:  emptyAuditField(),
			DirectID:  auditFieldValue("direct_test_01"),
			WorkID:    auditFieldValue("work_patch_42"),
		}); err == nil {
			t.Fatal("validateNetworkAuditEntry() error = nil, want direct_id mismatch")
		}
	})
}
