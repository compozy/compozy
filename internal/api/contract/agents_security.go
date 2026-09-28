package contract

import (
	"strings"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

func normalizeAgentCapabilities(values []AgentCapabilityPayload) []AgentCapabilityPayload {
	if values == nil {
		return []AgentCapabilityPayload{}
	}
	return values
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

func normalizeStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
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
