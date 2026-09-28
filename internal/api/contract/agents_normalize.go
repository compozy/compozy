package contract

import (
	"encoding/json"

	"errors"
	"fmt"

	taskpkg "github.com/compozy/compozy/internal/task"
)

// NormalizeAgentMePayload returns a payload with nil list sections converted to empty arrays.
func NormalizeAgentMePayload(payload AgentMePayload) AgentMePayload {
	payload.Capabilities = normalizeAgentCapabilities(payload.Capabilities)
	payload.ActiveTaskLeases = normalizeTaskRunLeases(payload.ActiveTaskLeases)
	payload.Session = NormalizeAgentSessionPayload(payload.Session)
	return payload
}

// NormalizeAgentContextPayload returns a context payload with stable bounded list sections.
func NormalizeAgentContextPayload(source *AgentContextPayload) AgentContextPayload {
	if source == nil {
		return AgentContextPayload{}
	}
	payload := *source
	payload.Session = NormalizeAgentSessionPayload(payload.Session)
	payload.Soul.Tone = normalizeStrings(payload.Soul.Tone)
	payload.Soul.Principles = normalizeStrings(payload.Soul.Principles)
	if payload.Task.Lease != nil {
		lease := NormalizeTaskRunLeaseSummaryPayload(*payload.Task.Lease)
		payload.Task.Lease = &lease
	}
	if payload.Task.Bundle != nil {
		bundle := taskpkg.NormalizeContextBundle(*payload.Task.Bundle)
		payload.Task.Bundle = &bundle
	}
	payload.Capabilities.Capabilities = normalizeAgentCapabilities(payload.Capabilities.Capabilities)
	payload.Capabilities.Section.Returned = len(payload.Capabilities.Capabilities)
	return payload
}

// NormalizeSessionLineagePayload returns a lineage payload with stable nested permission arrays.
func NormalizeSessionLineagePayload(payload *SessionLineagePayload) *SessionLineagePayload {
	if payload == nil {
		return nil
	}
	clone := *payload
	clone.PermissionPolicy = NormalizeSpawnPermissionPolicyPayload(clone.PermissionPolicy)
	return &clone
}

// NormalizeSpawnPermissionPolicyPayload returns a permission policy with stable empty arrays.
func NormalizeSpawnPermissionPolicyPayload(payload SpawnPermissionPolicyPayload) SpawnPermissionPolicyPayload {
	payload.Tools = normalizeStrings(payload.Tools)
	payload.Skills = normalizeStrings(payload.Skills)
	payload.MCPServers = normalizeStrings(payload.MCPServers)
	payload.WorkspacePaths = normalizeStrings(payload.WorkspacePaths)
	return payload
}

// NormalizeTaskRunLeaseSummaryPayload returns a lease summary with normalized nested channel metadata.
func NormalizeTaskRunLeaseSummaryPayload(payload TaskRunLeaseSummaryPayload) TaskRunLeaseSummaryPayload {
	return payload
}

// ErrRawClaimTokenMetadata reports raw lease credentials in a public payload.
var ErrRawClaimTokenMetadata = errors.New("contract: public payload must not contain raw lease credentials")

// ContainsRawClaimTokenField reports whether a JSON payload includes a raw claim_token field.
func ContainsRawClaimTokenField(payload any) (bool, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("marshal claim-token safety payload: %w", err)
	}
	return containsRawClaimTokenJSON(content), nil
}

// ValidateNoRawClaimTokenField rejects payloads that include a raw claim_token JSON field.
func ValidateNoRawClaimTokenField(payload any) error {
	found, err := ContainsRawClaimTokenField(payload)
	if err != nil {
		return err
	}
	if found {
		return ErrRawClaimTokenMetadata
	}
	return nil
}
