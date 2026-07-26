package extensiontest

import (
	"fmt"

	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func validateDeliveryConformance(
	deliveries []DeliveryRecord,
	expect ConformanceExpectation,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	if expect.RequireDelivery && len(deliveries) == 0 {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_delivery",
			Message: "adapter did not receive any delivery requests",
		})
	}

	lastSeq := make(map[string]int64)
	pendingAckResume := make(map[deliveryAckKey]struct{})
	remoteMessageSeen := make(map[string]bool)
	sawResume := false
	for _, record := range deliveries {
		deliveryIssues, resumed := validateDeliveryRecord(
			record,
			expectedByID,
			lastSeq,
			pendingAckResume,
			remoteMessageSeen,
		)
		issues = append(issues, deliveryIssues...)
		sawResume = sawResume || resumed
	}

	for key := range pendingAckResume {
		issues = append(issues, ConformanceIssue{
			Code: "missing_ack",
			Message: fmt.Sprintf(
				"delivery %q sequence %d did not return an ack or later resume",
				key.deliveryID,
				key.seq,
			),
		})
	}
	if expect.RequireResume && !sawResume {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_resume",
			Message: "adapter did not observe a resume delivery after restart",
		})
	}
	return issues
}

func validateDeliveryRecord(
	record DeliveryRecord,
	expectedByID map[string]ManagedInstanceExpectation,
	lastSeq map[string]int64,
	pendingAckResume map[deliveryAckKey]struct{},
	remoteMessageSeen map[string]bool,
) ([]ConformanceIssue, bool) {
	issues := make([]ConformanceIssue, 0)
	event := record.Request.Event
	deliveryID := strings.TrimSpace(event.DeliveryID)
	instanceID := strings.TrimSpace(event.BridgeInstanceID)
	eventType := normalizeEventType(event.EventType)
	sawResume := false

	if err := record.Request.Validate(); err != nil {
		issues = append(issues, ConformanceIssue{
			Code:    "invalid_delivery_request",
			Message: fmt.Sprintf("delivery %q failed request validation: %v", deliveryID, err),
		})
	}

	if len(expectedByID) > 0 {
		if _, ok := expectedByID[instanceID]; !ok {
			issues = append(issues, ConformanceIssue{
				Code:    "unexpected_delivery_instance",
				Message: fmt.Sprintf("delivery %q targeted unexpected instance %q", deliveryID, instanceID),
			})
		}
	}
	if targetID := strings.TrimSpace(event.DeliveryTarget.BridgeInstanceID); targetID != "" && targetID != instanceID {
		issues = append(issues, ConformanceIssue{
			Code: "mismatched_delivery_target",
			Message: fmt.Sprintf(
				"delivery %q event instance %q did not match target instance %q",
				deliveryID,
				instanceID,
				targetID,
			),
		})
	}
	if eventType == bridgepkg.DeliveryEventTypeResume {
		sawResume = true
		if record.Request.Snapshot == nil {
			issues = append(issues, ConformanceIssue{
				Code:    "missing_resume_snapshot",
				Message: fmt.Sprintf("resume delivery %q omitted its snapshot", deliveryID),
			})
		}
		clearPendingDeliveryAcks(pendingAckResume, deliveryID)
	}
	if eventType != bridgepkg.DeliveryEventTypeResume {
		if previous, ok := lastSeq[deliveryID]; ok && event.Seq <= previous {
			issues = append(issues, ConformanceIssue{
				Code:    "out_of_order_delivery",
				Message: fmt.Sprintf("delivery %q sequence %d arrived after %d", deliveryID, event.Seq, previous),
			})
		}
		lastSeq[deliveryID] = event.Seq
	}

	ackIssues := validateDeliveryAck(record, deliveryID, eventType, pendingAckResume, remoteMessageSeen)
	issues = append(issues, ackIssues...)
	return issues, sawResume
}

func validateDeliveryAck(
	record DeliveryRecord,
	deliveryID string,
	eventType bridgepkg.DeliveryEventType,
	pendingAckResume map[deliveryAckKey]struct{},
	remoteMessageSeen map[string]bool,
) []ConformanceIssue {
	event := record.Request.Event
	ackKey := deliveryAckKey{deliveryID: deliveryID, seq: event.Seq}
	if record.Ack == nil {
		pendingAckResume[ackKey] = struct{}{}
		return nil
	}

	issues := make([]ConformanceIssue, 0)
	if err := record.Ack.ValidateFor(event); err != nil {
		issues = append(issues, ConformanceIssue{
			Code:    "invalid_ack",
			Message: err.Error(),
		})
	}
	delete(pendingAckResume, ackKey)
	noopAck := eventType == bridgepkg.DeliveryEventTypeProgress || event.IsLifecycleOnlyFinal()
	if event.Seq > 0 && !noopAck &&
		strings.TrimSpace(record.Ack.RemoteMessageID) == "" {
		issues = append(issues, ConformanceIssue{
			Code:    "missing_remote_message_id",
			Message: fmt.Sprintf("delivery %q sequence %d did not return remote_message_id", deliveryID, event.Seq),
		})
	}
	if remoteMessageSeen[deliveryID] && eventType != bridgepkg.DeliveryEventTypeResume &&
		!noopAck &&
		strings.TrimSpace(record.Ack.ReplaceRemoteMessageID) == "" {
		issues = append(issues, ConformanceIssue{
			Code: "missing_replace_remote_message_id",
			Message: fmt.Sprintf(
				"delivery %q sequence %d did not return replace_remote_message_id",
				deliveryID,
				event.Seq,
			),
		})
	}
	if eventType != bridgepkg.DeliveryEventTypeProgress && strings.TrimSpace(record.Ack.RemoteMessageID) != "" {
		remoteMessageSeen[deliveryID] = true
	}
	return issues
}

type deliveryAckKey struct {
	deliveryID string
	seq        int64
}

func clearPendingDeliveryAcks(pending map[deliveryAckKey]struct{}, deliveryID string) {
	for key := range pending {
		if key.deliveryID == deliveryID {
			delete(pending, key)
		}
	}
}

func validateIngestConformance(
	ingests []IngestRecord,
	expectedByID map[string]ManagedInstanceExpectation,
) []ConformanceIssue {
	issues := make([]ConformanceIssue, 0)
	for _, record := range ingests {
		instanceID := strings.TrimSpace(record.Envelope.BridgeInstanceID)
		if instanceID == "" {
			issues = append(issues, ConformanceIssue{
				Code:    "missing_ingest_instance_id",
				Message: "adapter ingested an inbound message without bridge_instance_id",
			})
			continue
		}
		if len(expectedByID) > 0 {
			if _, ok := expectedByID[instanceID]; !ok {
				issues = append(issues, ConformanceIssue{
					Code:    "unexpected_ingest_instance",
					Message: fmt.Sprintf("ingest targeted unexpected instance %q", instanceID),
				})
			}
		}
	}
	return issues
}
