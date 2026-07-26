package situation

import (
	"encoding/json"

	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/network"

	taskpkg "github.com/compozy/agh/internal/task"
)

func inboxSummary(
	envelopes []network.Envelope,
	limit int,
	activeCoordinationChannelID string,
) contract.AgentInboxSummaryPayload {
	items := make([]contract.AgentInboxItemPayload, 0, len(envelopes))
	for _, envelope := range envelopes {
		metadata, ok := coordinationMetadataFromEnvelope(envelope)
		if !ok {
			continue
		}
		if active := strings.TrimSpace(activeCoordinationChannelID); active != "" &&
			strings.TrimSpace(metadata.ChannelID) != active {
			continue
		}
		items = append(items, contract.AgentInboxItemPayload{
			MessageID: strings.TrimSpace(envelope.ID),
			ChannelID: firstTrimmed(
				metadata.ChannelID,
				envelope.Channel,
			),
			Kind:      metadata.MessageKind,
			Metadata:  metadata,
			Preview:   envelopePreview(envelope),
			Timestamp: envelopeTimestamp(envelope),
		})
	}
	slices.SortStableFunc(items, func(left, right contract.AgentInboxItemPayload) int {
		if !left.Timestamp.Equal(right.Timestamp) {
			if left.Timestamp.After(right.Timestamp) {
				return -1
			}
			return 1
		}
		return strings.Compare(left.MessageID, right.MessageID)
	})

	return contract.AgentInboxSummaryPayload{
		Section:     sectionMeta(len(items), limit),
		UnreadCount: len(items),
		Items:       boundedInbox(items, limit),
	}
}

func peerRoster(peers []network.PeerInfo, sessionID string, limit int) contract.AgentPeerRosterPayload {
	roster := make([]contract.AgentPeerSummaryPayload, 0, len(peers))
	for _, peer := range peers {
		if peer.SessionID != nil && strings.TrimSpace(*peer.SessionID) == strings.TrimSpace(sessionID) {
			continue
		}
		roster = append(roster, contract.AgentPeerSummaryPayload{
			PeerID:       strings.TrimSpace(peer.PeerID),
			SessionID:    peerSessionID(peer),
			DisplayName:  peerDisplayName(peer),
			ChannelID:    strings.TrimSpace(peer.Channel),
			Capabilities: peerCapabilities(peer),
		})
	}
	slices.SortStableFunc(roster, func(left, right contract.AgentPeerSummaryPayload) int {
		if left.ChannelID != right.ChannelID {
			return strings.Compare(left.ChannelID, right.ChannelID)
		}
		return strings.Compare(left.PeerID, right.PeerID)
	})
	return contract.AgentPeerRosterPayload{
		Section: sectionMeta(len(roster), limit),
		Peers:   boundedPeers(roster, limit),
	}
}

func selectActiveRun(runs []taskpkg.Run) (taskpkg.Run, bool) {
	active := make([]taskpkg.Run, 0, len(runs))
	for _, run := range runs {
		if activeRunRank(run.Status) < 0 {
			continue
		}
		active = append(active, run)
	}
	if len(active) == 0 {
		return taskpkg.Run{}, false
	}
	slices.SortStableFunc(active, func(left, right taskpkg.Run) int {
		leftRank := activeRunRank(left.Status)
		rightRank := activeRunRank(right.Status)
		if leftRank != rightRank {
			return leftRank - rightRank
		}
		if leftTime, rightTime := runActivityTime(left), runActivityTime(right); !leftTime.Equal(rightTime) {
			if leftTime.After(rightTime) {
				return -1
			}
			return 1
		}
		return strings.Compare(left.ID, right.ID)
	})
	return active[0], true
}

func activeRunRank(status taskpkg.RunStatus) int {
	switch status.Normalize() {
	case taskpkg.TaskRunStatusRunning:
		return 0
	case taskpkg.TaskRunStatusStarting:
		return 1
	case taskpkg.TaskRunStatusClaimed:
		return 2
	case taskpkg.TaskRunStatusQueued:
		return 3
	default:
		return -1
	}
}

func runActivityTime(run taskpkg.Run) time.Time {
	return latestTime(run.QueuedAt, run.ClaimedAt, run.StartedAt, run.EndedAt)
}

func runMetadata(raw json.RawMessage) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil
	}
	values := make(map[string]string, len(decoded))
	for key, rawValue := range decoded {
		var stringValue string
		if err := json.Unmarshal(rawValue, &stringValue); err == nil {
			values[strings.TrimSpace(key)] = strings.TrimSpace(stringValue)
		}
	}
	return values
}

func coordinationMetadataFromEnvelope(
	envelope network.Envelope,
) (contract.CoordinationMessageMetadataPayload, bool) {
	for _, key := range []string{"coordination", "coordination_metadata", "agh_coordination", "metadata"} {
		if raw, ok := envelope.Ext[key]; ok {
			if metadata, decodeOK := decodeCoordinationMetadata(raw); decodeOK {
				return metadata, true
			}
		}
	}
	if len(envelope.Ext) > 0 {
		raw, err := json.Marshal(envelope.Ext)
		if err == nil {
			if metadata, ok := decodeCoordinationMetadata(raw); ok {
				return metadata, true
			}
		}
	}
	return contract.CoordinationMessageMetadataPayload{}, false
}

func decodeCoordinationMetadata(raw json.RawMessage) (contract.CoordinationMessageMetadataPayload, bool) {
	var metadata contract.CoordinationMessageMetadataPayload
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return contract.CoordinationMessageMetadataPayload{}, false
	}
	return metadata, true
}

func envelopePreview(envelope network.Envelope) string {
	body, err := envelope.DecodeBody()
	if err != nil {
		return ""
	}
	var preview string
	switch typed := body.(type) {
	case network.SayBody:
		preview = typed.Text
	case network.TraceBody:
		preview = typed.Message
	case network.CapabilityBody:
		preview = typed.Capability.Summary
	case network.ReceiptBody:
		if typed.Detail != nil {
			preview = *typed.Detail
		}
	case network.GreetBody:
		preview = typed.Summary
	}
	return truncateRunes(singleLine(preview), inboxPreviewLimit)
}

func envelopeTimestamp(envelope network.Envelope) time.Time {
	if envelope.TS <= 0 {
		return time.Time{}
	}
	return time.Unix(envelope.TS, 0).UTC()
}

func peerSessionID(peer network.PeerInfo) string {
	if peer.SessionID == nil {
		return ""
	}
	return strings.TrimSpace(*peer.SessionID)
}
