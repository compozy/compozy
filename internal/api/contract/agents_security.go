package contract

import (
	"encoding/json"

	"slices"
	"strings"

	redactpkg "github.com/compozy/agh/internal/redact"
)

func normalizeAgentCapabilities(values []AgentCapabilityPayload) []AgentCapabilityPayload {
	if values == nil {
		return []AgentCapabilityPayload{}
	}
	return values
}

func normalizeCoordinationChannels(values []CoordinationChannelPayload) []CoordinationChannelPayload {
	if values == nil {
		return []CoordinationChannelPayload{}
	}
	normalized := make([]CoordinationChannelPayload, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, NormalizeCoordinationChannelPayload(value))
	}
	return normalized
}

func normalizeTaskRunLeases(values []TaskRunLeaseSummaryPayload) []TaskRunLeaseSummaryPayload {
	if values == nil {
		return []TaskRunLeaseSummaryPayload{}
	}
	normalized := make([]TaskRunLeaseSummaryPayload, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, NormalizeTaskRunLeaseSummaryPayload(value))
	}
	return normalized
}

func normalizeInboxItems(values []AgentInboxItemPayload) []AgentInboxItemPayload {
	if values == nil {
		return []AgentInboxItemPayload{}
	}
	return values
}

func normalizePeers(values []AgentPeerSummaryPayload) []AgentPeerSummaryPayload {
	if values == nil {
		return []AgentPeerSummaryPayload{}
	}
	normalized := make([]AgentPeerSummaryPayload, 0, len(values))
	for _, value := range values {
		value.Capabilities = normalizeStrings(value.Capabilities)
		normalized = append(normalized, value)
	}
	return normalized
}

func normalizeStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func validCoordinationMessageKind(kind CoordinationMessageKind) bool {
	return slices.Contains(CoordinationMessageKinds(), kind)
}

func containsRawClaimTokenMap(values map[string]json.RawMessage) bool {
	if len(values) == 0 {
		return false
	}
	for key, value := range values {
		if isRawClaimTokenKey(key) || containsRawClaimTokenJSON(value) {
			return true
		}
	}
	return false
}

func containsRawClaimTokenJSON(data []byte) bool {
	return containsUnsafeJSON(data, isRawClaimTokenKey, isRawClaimTokenString)
}

func isRawClaimTokenKey(key string) bool {
	return strings.EqualFold(strings.TrimSpace(key), "claim_token")
}

func isRawClaimTokenString(value string) bool {
	return redactpkg.ContainsRawClaimToken(value)
}
