package bridges

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	redactpkg "github.com/compozy/agh/internal/redact"
)

const (
	sessionStoppedDeliveryMessage       = "session stopped before delivery completed"
	indeterminateDeliveryOutcomeMessage = "delivery outcome indeterminate after restart"
)

// ReconcileDelivery closes one persisted unfinished delivery. An unmatched
// send intent is terminalized locally; otherwise the provider receives one
// write-ahead terminal error post.
func (b *Broker) ReconcileDelivery(
	ctx context.Context,
	record DeliveryLedgerRecord,
	extensionName string,
) error {
	normalized, err := b.validateDeliveryReconciliation(ctx, record)
	if err != nil {
		return err
	}
	if normalized.LastSentSeq > normalized.LastAckedSeq {
		return b.terminalizeIndeterminateReconciliation(ctx, normalized)
	}
	extensionName = strings.TrimSpace(extensionName)
	if extensionName == "" {
		return errors.New("bridges: delivery reconciliation extension name is required")
	}

	request, checkpoint, metrics, err := b.failOpenReconciliation(normalized)
	if err != nil {
		return err
	}
	transport := b.currentTransport()
	if transport == nil {
		return ErrDeliveryTransportUnavailable
	}
	intent := DeliveryLedgerCheckpoint{
		DeliveryID:      normalized.DeliveryID,
		State:           DeliveryLedgerStateActive,
		LastSentSeq:     request.Event.Seq,
		LastAckedSeq:    normalized.LastAckedSeq,
		RemoteMessageID: normalized.RemoteMessageID,
		UpdatedAt:       checkpoint.UpdatedAt,
	}
	if err := b.persistDeliveryCheckpoint(ctx, intent); err != nil {
		return err
	}
	normalizedAck, err := b.requestReconciledDelivery(ctx, transport, extensionName, request)
	if err != nil {
		return err
	}
	return b.persistReconciledDelivery(ctx, normalized, request, normalizedAck, checkpoint, metrics)
}

func (b *Broker) validateDeliveryReconciliation(
	ctx context.Context,
	record DeliveryLedgerRecord,
) (DeliveryLedgerRecord, error) {
	if b == nil {
		return DeliveryLedgerRecord{}, errors.New("bridges: delivery broker is required")
	}
	if ctx == nil {
		return DeliveryLedgerRecord{}, errors.New("bridges: delivery reconciliation context is required")
	}
	if err := ctx.Err(); err != nil {
		return DeliveryLedgerRecord{}, err
	}
	if b.ledgerStore == nil {
		return DeliveryLedgerRecord{}, errors.New("bridges: delivery ledger store is required")
	}
	normalized, err := record.Canonicalize()
	if err != nil {
		return DeliveryLedgerRecord{}, err
	}
	if normalized.State != DeliveryLedgerStateActive {
		return DeliveryLedgerRecord{}, fmt.Errorf(
			"bridges: reconcile delivery %q state %q: %w",
			normalized.DeliveryID,
			normalized.State,
			ErrDeliveryLedgerConflict,
		)
	}
	return normalized, nil
}

func (b *Broker) requestReconciledDelivery(
	ctx context.Context,
	transport DeliveryTransport,
	extensionName string,
	request DeliveryRequest,
) (DeliveryAck, error) {
	callCtx, cancel := context.WithTimeout(ctx, b.requestTimeout)
	ack, err := transport.DeliverBridge(callCtx, extensionName, request)
	cancel()
	if err != nil {
		return DeliveryAck{}, fmt.Errorf("bridges: reconcile delivery %q: %w", request.Event.DeliveryID, err)
	}
	if err := ack.ValidateFor(request.Event); err != nil {
		return DeliveryAck{}, fmt.Errorf(
			"bridges: validate reconciled delivery %q acknowledgement: %w",
			request.Event.DeliveryID,
			err,
		)
	}
	return ack.normalize(), nil
}

func (b *Broker) persistReconciledDelivery(
	ctx context.Context,
	record DeliveryLedgerRecord,
	request DeliveryRequest,
	ack DeliveryAck,
	checkpoint DeliveryLedgerCheckpoint,
	metrics DeliveryMetricRecord,
) error {
	if ack.Outcome.Normalize() == DeliveryAckOutcomeCommittedResultUnavailable {
		message := redactpkg.String(strings.TrimSpace(ack.Error.Message))
		metrics = b.reconciliationMetricRecord(record, checkpoint.UpdatedAt, message)
		checkpoint.LastSentSeq = request.Event.Seq
		checkpoint.LastAckedSeq = record.LastAckedSeq
		checkpoint.RemoteMessageID = record.RemoteMessageID
		checkpoint.TerminalError = message
	} else if ack.RemoteMessageID != "" {
		checkpoint.RemoteMessageID = ack.RemoteMessageID
	}
	return b.persistReconciliationResult(ctx, checkpoint, metrics)
}

func (b *Broker) persistReconciliationResult(
	ctx context.Context,
	checkpoint DeliveryLedgerCheckpoint,
	metrics DeliveryMetricRecord,
) error {
	checkpoint.Metrics = &metrics
	if err := b.persistDeliveryCheckpoint(ctx, checkpoint); err != nil {
		return err
	}

	b.mu.Lock()
	b.applyDeliveryMetricRecordLocked(metrics)
	b.mu.Unlock()
	return nil
}

func (b *Broker) terminalizeIndeterminateReconciliation(
	ctx context.Context,
	record DeliveryLedgerRecord,
) error {
	updatedAt := b.now()
	metrics := b.reconciliationMetricRecord(record, updatedAt, indeterminateDeliveryOutcomeMessage)
	checkpoint := DeliveryLedgerCheckpoint{
		DeliveryID:      record.DeliveryID,
		State:           DeliveryLedgerStateTerminalError,
		LastSentSeq:     record.LastSentSeq,
		LastAckedSeq:    record.LastAckedSeq,
		RemoteMessageID: record.RemoteMessageID,
		TerminalError:   indeterminateDeliveryOutcomeMessage,
		UpdatedAt:       updatedAt,
	}
	return b.persistReconciliationResult(ctx, checkpoint, metrics)
}

func (b *Broker) failOpenReconciliation(
	record DeliveryLedgerRecord,
) (DeliveryRequest, DeliveryLedgerCheckpoint, DeliveryMetricRecord, error) {
	target := DeliveryTarget{
		BridgeInstanceID: record.BridgeInstanceID,
		PeerID:           record.RoutingKey.PeerID,
		ThreadID:         record.RoutingKey.ThreadID,
		GroupID:          record.RoutingKey.GroupID,
		Mode:             DeliveryModeReply,
	}
	if err := target.Validate(); err != nil {
		return DeliveryRequest{}, DeliveryLedgerCheckpoint{}, DeliveryMetricRecord{}, err
	}
	seq := max(record.LastSentSeq, record.LastAckedSeq) + 1
	updatedAt := b.now()
	content := MessageContent{Text: sessionStoppedDeliveryMessage}
	event := DeliveryEvent{
		DeliveryID:       record.DeliveryID,
		BridgeInstanceID: record.BridgeInstanceID,
		RoutingKey:       record.RoutingKey,
		DeliveryTarget:   target,
		Seq:              seq,
		EventType:        DeliveryEventTypeError,
		Content:          content,
		Final:            true,
		Operation:        DeliveryOperationPost,
		Error:            &DeliveryErrorDetail{Message: sessionStoppedDeliveryMessage},
	}
	request := DeliveryRequest{Event: event}
	if err := request.Validate(); err != nil {
		return DeliveryRequest{}, DeliveryLedgerCheckpoint{}, DeliveryMetricRecord{}, err
	}

	metrics := b.reconciliationMetricRecord(record, updatedAt, sessionStoppedDeliveryMessage)
	checkpoint := DeliveryLedgerCheckpoint{
		DeliveryID:      record.DeliveryID,
		State:           DeliveryLedgerStateTerminalError,
		LastSentSeq:     seq,
		LastAckedSeq:    seq,
		RemoteMessageID: record.RemoteMessageID,
		TerminalError:   sessionStoppedDeliveryMessage,
		UpdatedAt:       updatedAt,
	}
	return request, checkpoint, metrics, nil
}

func (b *Broker) reconciliationMetricRecord(
	record DeliveryLedgerRecord,
	updatedAt time.Time,
	reason string,
) DeliveryMetricRecord {
	b.mu.Lock()
	defer b.mu.Unlock()
	metrics := b.metricsLocked(record.BridgeInstanceID)
	droppedReasons := cloneDeliveryDropReasons(metrics.droppedByReason)
	droppedTotal := 0
	for _, count := range droppedReasons {
		droppedTotal += count
	}
	return DeliveryMetricRecord{
		BridgeDeliveryMetrics: BridgeDeliveryMetrics{
			BridgeInstanceID:        record.BridgeInstanceID,
			DeliveryDroppedTotal:    droppedTotal,
			DeliveryDroppedByReason: droppedReasons,
			DeliveryFailuresTotal:   metrics.deliveryFailuresTotal + 1,
			LastError:               reason,
			LastErrorAt:             updatedAt,
			LastSuccessAt:           metrics.lastSuccessAt,
		},
		Scope:       record.Scope,
		WorkspaceID: record.WorkspaceID,
		UpdatedAt:   updatedAt,
	}
}

func (b *Broker) applyDeliveryMetricRecordLocked(record DeliveryMetricRecord) {
	metrics := b.metricsLocked(record.BridgeInstanceID)
	if metrics == nil {
		return
	}
	metrics.scope = record.Scope
	metrics.workspaceID = record.WorkspaceID
	metrics.droppedByReason = cloneDeliveryDropReasons(record.DeliveryDroppedByReason)
	metrics.deliveryFailuresTotal = record.DeliveryFailuresTotal
	metrics.lastError = record.LastError
	metrics.lastErrorAt = record.LastErrorAt
	metrics.lastSuccessAt = record.LastSuccessAt
	metrics.updatedAt = record.UpdatedAt
	metrics.persistedRevision = metrics.revision
}

func terminalFailureContent(current MessageContent, reason string) MessageContent {
	safeReason := redactpkg.String(strings.TrimSpace(reason))
	if safeReason == "" {
		safeReason = sessionStoppedDeliveryMessage
	}
	currentText := strings.TrimSpace(current.Text)
	if currentText == "" {
		return MessageContent{Text: safeReason}
	}
	return MessageContent{Text: currentText + "\n\n" + safeReason}
}
