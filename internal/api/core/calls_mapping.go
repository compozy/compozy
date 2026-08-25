package core

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
)

func (h *BaseHandlers) callPayloads(
	ctx context.Context,
	records []callspkg.CallRecord,
) ([]contract.CallPayload, error) {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]contract.CallPayload, 0, len(records))
	for _, record := range records {
		items = append(items, callPayload(record, owners[record.ProfileID], nil))
	}
	return items, nil
}

func callPayload(
	record callspkg.CallRecord,
	profile profileOwnerIdentity,
	resultPreview json.RawMessage,
) contract.CallPayload {
	payload := contract.CallPayload{
		CallID: record.CallID, ProfileID: record.ProfileID, ProfileName: profile.Name,
		Scope: string(record.Scope), WorkspaceID: record.WorkspaceID,
		Caller: contract.CallOwnerPayload{Kind: string(record.Caller.Kind), ID: record.Caller.ID},
		Actor:  contract.CallOwnerPayload{Kind: record.Actor.Kind, ID: record.Actor.ID},
		Agent:  record.AgentName, ChildSessionID: record.ChildSessionID,
		ParentSessionID: record.ParentSessionID, RootSessionID: record.GovernedRootID,
		Depth: record.Depth, State: string(record.State), Verdict: string(record.Verdict),
		ExpectDigest: record.ExpectDigest, ResultPreview: cloneCallJSON(resultPreview),
		ResultBytes: record.ResultBytes, ResultBudget: record.ResultBudget.MaxBytes,
		ResultOverflow: string(record.ResultBudget.Overflow), Strict: record.Strict,
		IdleTTLSeconds: durationSeconds(record.IdleTTL), FailureCode: record.FailureCode,
		FailureDetail: record.FailureDetail, FinalProsePreview: record.FinalProsePreview,
		RepairAttempts: record.RepairAttempts,
		Replayed:       record.Replayed, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		StartedAt: timePointer(record.StartedAt), SettledAt: timePointer(record.SettledAt),
		DeadlineAt: timePointer(record.DeadlineAt),
	}
	if record.Verdict != "" || record.ChildSessionID != "" {
		payload.Provenance = &contract.CallProvenancePayload{
			ProducedBy: record.AgentName, SessionID: record.ChildSessionID,
			Admitted: string(record.Verdict),
		}
	}
	return payload
}

func callCreatePayload(record callspkg.CallRecord) contract.CallCreatePayload {
	return contract.CallCreatePayload{
		CallID: record.CallID, ChildSessionID: record.ChildSessionID,
		State: string(record.State), Replayed: record.Replayed,
	}
}

func (h *BaseHandlers) callMessagePayloads(
	ctx context.Context,
	records []callspkg.MessageRecord,
) ([]contract.CallMessagePayload, error) {
	owners, err := h.profileOwnerIdentities(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]contract.CallMessagePayload, 0, len(records))
	for _, record := range records {
		items = append(items, callMessagePayload(record, owners[record.ProfileID]))
	}
	return items, nil
}

func callMessagePayload(record callspkg.MessageRecord, profile profileOwnerIdentity) contract.CallMessagePayload {
	return contract.CallMessagePayload{
		MessageID: record.MessageID, ProfileID: record.ProfileID, ProfileName: profile.Name,
		Scope: string(record.Scope), WorkspaceID: record.WorkspaceID,
		From:          contract.CallOwnerPayload{Kind: record.From.Kind, ID: record.From.ID},
		FromAgentName: record.FromAgentName, ToSessionID: record.ToSessionID,
		CallID: record.CallID, Text: record.Body, Delivery: publicCallDelivery(record.Delivery),
		Reason: record.DeliveryReason, Attempts: record.DeliveryAttempts,
		CreatedAt: record.CreatedAt, DeliveredAt: timePointer(record.DeliveredAt),
	}
}

func publicCallDelivery(value string) string {
	switch strings.TrimSpace(value) {
	case "pending":
		return "queued"
	case "injected":
		return "delivered-into-turn"
	case "woken":
		return "woke"
	default:
		return strings.TrimSpace(value)
	}
}

func durationSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}
	return int64((value + time.Second - 1) / time.Second)
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copy := value.UTC()
	return &copy
}

func cloneCallJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
