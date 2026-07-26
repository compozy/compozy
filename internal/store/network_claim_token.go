package store

import (
	"encoding/json"
	"slices"
	"strings"
)

func networkAuditEntryContainsRawClaimToken(entry NetworkAuditEntry) bool {
	values := []string{
		entry.ID,
		entry.SessionID,
		entry.Direction,
		entry.Kind,
		entry.Channel,
		entry.Surface,
		entry.ThreadID,
		entry.DirectID,
		entry.WorkID,
		entry.PeerFrom,
		entry.PeerTo,
		entry.MessageID,
		entry.Reason,
	}
	return slices.ContainsFunc(values, networkStringContainsRawClaimToken)
}

func networkConversationMessageContainsRawClaimToken(entry NetworkConversationMessage) bool {
	values := []string{
		entry.MessageID,
		entry.SessionID,
		entry.Channel,
		entry.Surface,
		entry.ThreadID,
		entry.DirectID,
		entry.Direction,
		entry.PeerFrom,
		entry.PeerTo,
		strings.Join(entry.Mentions, ","),
		entry.Kind,
		entry.WorkID,
		entry.ReplyTo,
		entry.TraceID,
		entry.CausationID,
		entry.Intent,
		entry.Text,
		entry.PreviewText,
	}
	return slices.ContainsFunc(values, networkStringContainsRawClaimToken)
}

func networkRawJSONContainsClaimToken(raw json.RawMessage) bool {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return networkStringContainsRawClaimToken(string(raw))
	}
	return networkValueContainsClaimToken("", value)
}

func networkValueContainsClaimToken(key string, value any) bool {
	if networkStringContainsRawClaimToken(key) || networkClaimTokenKeyHasValue(key, value) {
		return true
	}
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return networkStringContainsRawClaimToken(typed)
	case []any:
		for _, item := range typed {
			if networkValueContainsClaimToken("", item) {
				return true
			}
		}
	case map[string]any:
		for nestedKey, nestedValue := range typed {
			if networkValueContainsClaimToken(nestedKey, nestedValue) {
				return true
			}
		}
	}
	return false
}

func networkClaimTokenKeyHasValue(key string, value any) bool {
	normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(strings.TrimSpace(key)))
	if normalized != "claimtoken" {
		return false
	}
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(typed) != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}

func networkStringContainsRawClaimToken(value string) bool {
	return strings.Contains(strings.TrimSpace(value), "agh_claim_")
}
