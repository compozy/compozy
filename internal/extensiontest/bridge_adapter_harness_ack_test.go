package extensiontest

import (
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestValidateConformanceDeliveryAckTrackingContract(t *testing.T) {
	t.Parallel()

	t.Run("Should keep missing ack when a later normal ack uses the same delivery id", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{
			{
				Request: testDeliveryRequest("delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
			},
			{
				Request: testDeliveryRequest("delivery-1", 2, bridgepkg.DeliveryEventTypeDelta, false),
				Ack:     testDeliveryAck("delivery-1", 2, "telegram:delivery-1:2", "telegram:delivery-1:1"),
			},
		}

		assertConformanceIssue(t, report, ConformanceExpectation{RequireDelivery: true}, "missing_ack")
	})

	t.Run("Should clear missing ack after an explicit resume delivery", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{
			{
				Request: testDeliveryRequest("delivery-1", 1, bridgepkg.DeliveryEventTypeStart, false),
			},
			testResumeDeliveryRecordContract("delivery-1", 2),
		}

		if err := ValidateConformance(report, ConformanceExpectation{
			RequireDelivery: true,
			RequireResume:   true,
		}); err != nil {
			t.Fatalf("ValidateConformance() error = %v, want nil", err)
		}
	})

	t.Run("Should accept a noop acknowledgement for progress delivery", func(t *testing.T) {
		t.Parallel()

		request := testProgressDeliveryRequestContract("delivery-progress", 2)
		record := DeliveryRecord{
			Request: request,
			Ack:     testDeliveryAck("delivery-progress", 2, "", ""),
		}
		issues := validateDeliveryAck(
			record,
			"delivery-progress",
			bridgepkg.DeliveryEventTypeProgress,
			make(map[deliveryAckKey]struct{}),
			make(map[string]bool),
		)
		if len(issues) != 0 {
			t.Fatalf("validateDeliveryAck(progress) issues = %#v, want none", issues)
		}
	})

	t.Run("Should accept a noop acknowledgement for a lifecycle-only final", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest(
			"delivery-progress-final",
			2,
			bridgepkg.DeliveryEventTypeFinal,
			true,
		)
		request.Event.Content = bridgepkg.MessageContent{}
		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{{
			Request: request,
			Ack:     testDeliveryAck("delivery-progress-final", 2, "", ""),
		}}

		if err := ValidateConformance(report, ConformanceExpectation{RequireDelivery: true}); err != nil {
			t.Fatalf("ValidateConformance() error = %v, want nil", err)
		}
	})

	t.Run("Should require a remote message id for a materialized final", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{{
			Request: testDeliveryRequest("delivery-text-final", 2, bridgepkg.DeliveryEventTypeFinal, true),
			Ack:     testDeliveryAck("delivery-text-final", 2, "", ""),
		}}

		assertConformanceIssue(
			t,
			report,
			ConformanceExpectation{RequireDelivery: true},
			"missing_remote_message_id",
		)
	})

	t.Run("Should require a replacement id for a materialized final after text", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{
			{
				Request: testDeliveryRequest("delivery-text-replacement", 1, bridgepkg.DeliveryEventTypeStart, false),
				Ack:     testDeliveryAck("delivery-text-replacement", 1, "telegram:answer", ""),
			},
			{
				Request: testDeliveryRequest("delivery-text-replacement", 2, bridgepkg.DeliveryEventTypeFinal, true),
				Ack:     testDeliveryAck("delivery-text-replacement", 2, "telegram:answer", ""),
			},
		}

		assertConformanceIssue(
			t,
			report,
			ConformanceExpectation{RequireDelivery: true},
			"missing_replace_remote_message_id",
		)
	})

	t.Run("Should require a remote message id for an empty edit final", func(t *testing.T) {
		t.Parallel()

		request := testDeliveryRequest("delivery-edit-final", 2, bridgepkg.DeliveryEventTypeFinal, true)
		request.Event.Content = bridgepkg.MessageContent{}
		request.Event.Operation = bridgepkg.DeliveryOperationEdit
		request.Event.Reference = &bridgepkg.DeliveryMessageReference{RemoteMessageID: "telegram:original"}
		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{{
			Request: request,
			Ack:     testDeliveryAck("delivery-edit-final", 2, "", ""),
		}}

		assertConformanceIssue(
			t,
			report,
			ConformanceExpectation{RequireDelivery: true},
			"missing_remote_message_id",
		)
	})

	t.Run("Should not require replacement after a noop progress acknowledgement", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{
			{
				Request: testProgressDeliveryRequestContract("delivery-progress", 1),
				Ack:     testDeliveryAck("delivery-progress", 1, "", ""),
			},
			{
				Request: testDeliveryRequest("delivery-progress", 2, bridgepkg.DeliveryEventTypeStart, false),
				Ack:     testDeliveryAck("delivery-progress", 2, "telegram:delivery-progress:2", ""),
			},
		}

		if err := ValidateConformance(report, ConformanceExpectation{RequireDelivery: true}); err != nil {
			t.Fatalf("ValidateConformance() error = %v, want nil", err)
		}
	})

	t.Run("Should not treat a progress remote id as a textual replacement anchor", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		report.Deliveries = []DeliveryRecord{
			{
				Request: testProgressDeliveryRequestContract("delivery-progress-anchor", 1),
				Ack:     testDeliveryAck("delivery-progress-anchor", 1, "progress-bubble", ""),
			},
			{
				Request: testDeliveryRequest("delivery-progress-anchor", 2, bridgepkg.DeliveryEventTypeStart, false),
				Ack:     testDeliveryAck("delivery-progress-anchor", 2, "answer-message", ""),
			},
		}

		if err := ValidateConformance(report, ConformanceExpectation{RequireDelivery: true}); err != nil {
			t.Fatalf("ValidateConformance() error = %v, want nil", err)
		}
	})
}

func testProgressDeliveryRequestContract(deliveryID string, seq int64) bridgepkg.DeliveryRequest {
	request := testDeliveryRequest(deliveryID, seq, bridgepkg.DeliveryEventTypeProgress, false)
	request.Event.Content = bridgepkg.MessageContent{}
	request.Event.Progress = &bridgepkg.ToolProgress{
		ToolCallID: "call-progress",
		ToolID:     "agh__terminal",
		Phase:      bridgepkg.ToolProgressPhaseStarted,
		Label:      "Running",
		Emoji:      "⚙️",
		Index:      1,
	}
	return request
}

func testResumeDeliveryRecordContract(deliveryID string, seq int64) DeliveryRecord {
	request := testDeliveryRequest(deliveryID, seq, bridgepkg.DeliveryEventTypeResume, false)
	request.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeStart}
	request.Snapshot = &bridgepkg.DeliverySnapshot{
		DeliveryID:       deliveryID,
		SessionID:        "sess-1",
		TurnID:           "turn-1",
		BridgeInstanceID: "brg-telegram-reference",
		RoutingKey:       request.Event.RoutingKey,
		DeliveryTarget:   request.Event.DeliveryTarget,
		LatestSeq:        seq - 1,
		LatestEventType:  bridgepkg.DeliveryEventTypeStart,
		CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
		LastSentSeq:      seq - 1,
		RemoteMessageID:  "telegram:delivery-1:1",
		UpdatedAt:        time.Date(2026, 4, 11, 5, 1, 0, 0, time.UTC),
	}
	return DeliveryRecord{
		Request: request,
		Ack:     testDeliveryAck(deliveryID, seq, "telegram:delivery-1:1", ""),
	}
}
