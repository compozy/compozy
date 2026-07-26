package globaldb

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

var _ store.NetworkAcceptanceStore = (*NetworkRepo)(nil)

// AcceptNetworkMessage commits conversation evidence, recipient decisions, and wake work atomically.
func (g *NetworkRepo) AcceptNetworkMessage(
	ctx context.Context,
	req store.AcceptNetworkMessageRequest,
) (result store.AcceptNetworkMessageResult, err error) {
	if err := g.checkReady(ctx, "accept network message"); err != nil {
		return store.AcceptNetworkMessageResult{}, err
	}
	normalized, err := g.normalizeNetworkAcceptance(req)
	if err != nil {
		return store.AcceptNetworkMessageResult{}, err
	}
	now := normalized.Message.Timestamp

	err = g.withNetworkImmediateTransaction(ctx, "accept network message", func(exec networkSQLExecutor) error {
		return g.acceptNetworkMessageWithExecutor(ctx, exec, &normalized, now, &result)
	})
	if err != nil {
		return store.AcceptNetworkMessageResult{}, err
	}
	return result, nil
}

func (g *NetworkRepo) acceptNetworkMessageWithExecutor(
	ctx context.Context,
	exec networkSQLExecutor,
	normalized *store.AcceptNetworkMessageRequest,
	now time.Time,
	result *store.AcceptNetworkMessageResult,
) error {
	write, acceptanceSeq, err := persistNetworkConversationMessageWithExecutor(
		ctx,
		exec,
		normalized.Message,
	)
	if err != nil {
		return err
	}
	result.AcceptanceSeq = acceptanceSeq
	result.Duplicate = write.Duplicate
	result.Conversation = write
	if write.Duplicate {
		result.Dispositions, err = listNetworkMessageDispositions(
			ctx,
			exec,
			normalized.Message.WorkspaceID,
			normalized.Message.MessageID,
		)
		return err
	}
	if err := insertNetworkMessageDispositions(
		ctx,
		exec,
		normalized.Message,
		normalized.Dispositions,
		acceptanceSeq,
		now,
	); err != nil {
		return err
	}
	if err := upsertDeliveredNetworkThreadParticipants(
		ctx,
		exec,
		normalized.Message,
		normalized.Dispositions,
	); err != nil {
		return err
	}
	result.Dispositions = append([]store.NetworkMessageDisposition(nil), normalized.Dispositions...)
	availability, err := getNetworkAvailabilityWithExecutor(ctx, exec)
	if err != nil {
		return err
	}
	result.AvailabilityEpoch = availability.Epoch
	return g.admitAcceptedNetworkWakes(
		ctx,
		exec,
		normalized,
		acceptanceSeq,
		now,
		availability.Enabled,
		result,
	)
}

func (g *NetworkRepo) admitAcceptedNetworkWakes(
	ctx context.Context,
	exec networkSQLExecutor,
	normalized *store.AcceptNetworkMessageRequest,
	acceptanceSeq int64,
	now time.Time,
	networkEnabled bool,
	result *store.AcceptNetworkMessageResult,
) error {
	for _, input := range normalized.Admissions {
		decision, err := g.admitNetworkWake(
			ctx,
			exec,
			normalized.Message,
			input,
			acceptanceSeq,
			now,
			networkEnabled,
		)
		if err != nil {
			return err
		}
		if decision.reservation != nil {
			result.Admitted = append(result.Admitted, *decision.reservation)
			result.Notify = append(result.Notify, store.CommittedNetworkNotification{
				WorkspaceID:        normalized.Message.WorkspaceID,
				RecipientSessionID: input.RecipientSessionID,
				TaskRunID:          decision.reservation.TaskRunID,
				AcceptanceSeq:      acceptanceSeq,
				ReadyAt:            decision.reservation.CoalesceUntil,
			})
		}
		if decision.skip != nil {
			result.Skipped = append(result.Skipped, *decision.skip)
		}
	}
	return nil
}

func upsertDeliveredNetworkThreadParticipants(
	ctx context.Context,
	exec networkSQLExecutor,
	message store.NetworkConversationMessage,
	dispositions []store.NetworkMessageDisposition,
) error {
	if message.Surface != store.NetworkSurfaceThread {
		return nil
	}
	for _, disposition := range dispositions {
		if disposition.Decision != store.NetworkDispositionDeliver {
			continue
		}
		recipientMessage := message
		recipientMessage.SessionID = disposition.RecipientSessionID
		if err := upsertNetworkThreadParticipant(ctx, exec, recipientMessage); err != nil {
			return err
		}
	}
	return refreshNetworkThreadSummary(ctx, exec, message)
}

func (g *NetworkRepo) normalizeNetworkAcceptance(
	req store.AcceptNetworkMessageRequest,
) (store.AcceptNetworkMessageRequest, error) {
	message, err := g.normalizeConversationMessage(req.Message)
	if err != nil {
		return store.AcceptNetworkMessageRequest{}, err
	}
	normalized := store.AcceptNetworkMessageRequest{
		Message:      message,
		Dispositions: make([]store.NetworkMessageDisposition, 0, len(req.Dispositions)),
		Admissions:   make([]store.NetworkWakeAdmissionInput, 0, len(req.Admissions)),
	}
	decisions := make(map[string]string, len(req.Dispositions))
	for _, disposition := range req.Dispositions {
		disposition.RecipientSessionID = strings.TrimSpace(disposition.RecipientSessionID)
		disposition.Decision = strings.TrimSpace(disposition.Decision)
		if err := disposition.Validate(); err != nil {
			return store.AcceptNetworkMessageRequest{}, err
		}
		if _, exists := decisions[disposition.RecipientSessionID]; exists {
			return store.AcceptNetworkMessageRequest{}, fmt.Errorf(
				"store: duplicate network message disposition recipient %q",
				disposition.RecipientSessionID,
			)
		}
		decisions[disposition.RecipientSessionID] = disposition.Decision
		normalized.Dispositions = append(normalized.Dispositions, disposition)
	}
	admissionRecipients := make(map[string]struct{}, len(req.Admissions))
	for _, input := range req.Admissions {
		input = normalizeNetworkWakeAdmissionInput(input)
		if err := input.Validate(); err != nil {
			return store.AcceptNetworkMessageRequest{}, err
		}
		if decisions[input.RecipientSessionID] != store.NetworkDispositionDeliver {
			return store.AcceptNetworkMessageRequest{}, fmt.Errorf(
				"store: network wake recipient %q must have a deliver disposition",
				input.RecipientSessionID,
			)
		}
		if _, exists := admissionRecipients[input.RecipientSessionID]; exists {
			return store.AcceptNetworkMessageRequest{}, fmt.Errorf(
				"store: duplicate network wake admission recipient %q",
				input.RecipientSessionID,
			)
		}
		if input.WorkspaceID != message.WorkspaceID {
			return store.AcceptNetworkMessageRequest{}, fmt.Errorf(
				"store: network wake admission workspace does not match accepted message",
			)
		}
		admissionRecipients[input.RecipientSessionID] = struct{}{}
		if input.Spec.Mode == "live" &&
			(input.Spec.WorkspaceID != message.WorkspaceID || input.Spec.ChannelID != message.Channel) {
			return store.AcceptNetworkMessageRequest{}, fmt.Errorf(
				"store: network wake participation scope does not match accepted message",
			)
		}
		normalized.Admissions = append(normalized.Admissions, input)
	}
	return normalized, nil
}

func normalizeNetworkWakeAdmissionInput(input store.NetworkWakeAdmissionInput) store.NetworkWakeAdmissionInput {
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.RecipientSessionID = strings.TrimSpace(input.RecipientSessionID)
	input.OwnerKey = strings.TrimSpace(input.OwnerKey)
	input.Trigger = strings.TrimSpace(input.Trigger)
	input.RootID = strings.TrimSpace(input.RootID)
	input.WakeID = strings.TrimSpace(input.WakeID)
	input.TaskRunID = strings.TrimSpace(input.TaskRunID)
	return input
}
