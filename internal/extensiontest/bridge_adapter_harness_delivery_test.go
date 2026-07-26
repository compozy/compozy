package extensiontest

import (
	"testing"
	"time"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

func TestValidateConformanceDeliveryRequestContract(t *testing.T) {
	t.Run("Should flag delivery records that violate the canonical request contract", func(t *testing.T) {
		t.Parallel()

		report := validConformanceReport()
		request := testDeliveryRequest("delivery-1", 2, bridgepkg.DeliveryEventTypeResume, false)
		request.Event.Resume = &bridgepkg.DeliveryResumeState{LatestEventType: bridgepkg.DeliveryEventTypeStart}
		request.Snapshot = &bridgepkg.DeliverySnapshot{
			DeliveryID:       "different-delivery",
			SessionID:        "sess-1",
			TurnID:           "turn-1",
			BridgeInstanceID: request.Event.BridgeInstanceID,
			RoutingKey:       request.Event.RoutingKey,
			DeliveryTarget:   request.Event.DeliveryTarget,
			LatestSeq:        1,
			LatestEventType:  bridgepkg.DeliveryEventTypeStart,
			CurrentContent:   bridgepkg.MessageContent{Text: "hello"},
			LastSentSeq:      1,
			RemoteMessageID:  "telegram:delivery-1:1",
			UpdatedAt:        time.Date(2026, 4, 11, 5, 1, 0, 0, time.UTC),
		}
		report.Deliveries = []DeliveryRecord{{
			Request: request,
			Ack:     testDeliveryAck("delivery-1", 2, "telegram:delivery-1:1", ""),
		}}

		assertConformanceIssue(t, report, ConformanceExpectation{RequireDelivery: true}, "invalid_delivery_request")
	})
}
