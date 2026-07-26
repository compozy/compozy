package contract

import (
	"encoding/json"

	"fmt"

	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

// CoordinationMessageKinds returns the accepted MVP coordination message kinds.
func CoordinationMessageKinds() []CoordinationMessageKind {
	return []CoordinationMessageKind{
		CoordinationMessageStatus,
		CoordinationMessageRequest,
		CoordinationMessageReply,
		CoordinationMessageBlocker,
		CoordinationMessageHandoff,
		CoordinationMessageResult,
		CoordinationMessageReviewRequest,
	}
}

// NormalizeAgentMePayload returns a payload with nil list sections converted to empty arrays.
func NormalizeAgentMePayload(payload AgentMePayload) AgentMePayload {
	payload.Capabilities = normalizeAgentCapabilities(payload.Capabilities)
	payload.Channels = normalizeCoordinationChannels(payload.Channels)
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
	if payload.CoordinationChannel.Channel != nil {
		channel := NormalizeCoordinationChannelPayload(*payload.CoordinationChannel.Channel)
		payload.CoordinationChannel.Channel = &channel
	}
	payload.InboxSummary.Items = normalizeInboxItems(payload.InboxSummary.Items)
	payload.InboxSummary.Section.Returned = len(payload.InboxSummary.Items)
	payload.PeerRoster.Peers = normalizePeers(payload.PeerRoster.Peers)
	payload.PeerRoster.Section.Returned = len(payload.PeerRoster.Peers)
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
	payload.NetworkChannels = normalizeStrings(payload.NetworkChannels)
	payload.SandboxProfiles = normalizeStrings(payload.SandboxProfiles)
	return payload
}

// NormalizeCoordinationChannelPayload returns a channel payload with stable message-kind arrays.
func NormalizeCoordinationChannelPayload(payload CoordinationChannelPayload) CoordinationChannelPayload {
	if payload.AllowedMessageKinds == nil {
		payload.AllowedMessageKinds = CoordinationMessageKinds()
	}
	return payload
}

// NormalizeTaskRunLeaseSummaryPayload returns a lease summary with normalized nested channel metadata.
func NormalizeTaskRunLeaseSummaryPayload(payload TaskRunLeaseSummaryPayload) TaskRunLeaseSummaryPayload {
	if payload.CoordinationChannel != nil {
		channel := NormalizeCoordinationChannelPayload(*payload.CoordinationChannel)
		payload.CoordinationChannel = &channel
	}
	return payload
}

// Validate rejects missing correlation fields, unknown message kinds, and raw claim token metadata.
func (p CoordinationMessageMetadataPayload) Validate() error {
	if strings.TrimSpace(p.TaskID) == "" {
		return fmt.Errorf("%w: task_id is required", ErrInvalidCoordinationMessageMetadata)
	}
	if strings.TrimSpace(p.RunID) == "" {
		return fmt.Errorf("%w: run_id is required", ErrInvalidCoordinationMessageMetadata)
	}
	if strings.TrimSpace(p.ChannelID) == "" {
		return fmt.Errorf("%w: channel_id is required", ErrInvalidCoordinationMessageMetadata)
	}
	if strings.TrimSpace(p.CorrelationID) == "" {
		return fmt.Errorf("%w: correlation_id is required", ErrInvalidCoordinationMessageMetadata)
	}
	if !validCoordinationMessageKind(p.MessageKind) {
		return fmt.Errorf("%w: unsupported message_kind %q", ErrInvalidCoordinationMessageMetadata, p.MessageKind)
	}
	if containsRawClaimTokenMap(p.Ext) {
		return ErrRawClaimTokenMetadata
	}
	return nil
}

// UnmarshalJSON rejects raw claim tokens in typed channel message metadata.
func (p *CoordinationMessageMetadataPayload) UnmarshalJSON(data []byte) error {
	if containsRawClaimTokenJSON(data) {
		return ErrRawClaimTokenMetadata
	}

	type metadataAlias CoordinationMessageMetadataPayload
	var decoded metadataAlias
	if err := decodeStrictContractJSON(data, &decoded); err != nil {
		return err
	}
	*p = CoordinationMessageMetadataPayload(decoded)
	return p.Validate()
}

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
