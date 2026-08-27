package core

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

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
	projected, err := h.Calls.ProjectPayloads(ctx, records)
	if err != nil {
		return nil, err
	}
	items := make([]contract.CallPayload, 0, len(records))
	for index := range records {
		record := &records[index]
		content, contentErr := h.callPayloadContent(ctx, record, projected[index])
		if contentErr != nil {
			return nil, contentErr
		}
		items = append(items, callPayload(record, owners[record.ProfileID], content))
	}
	return items, nil
}

type callPayloadContent struct {
	PromptPreview     string
	PromptBytes       int
	ResultPreview     json.RawMessage
	SupersededPreview json.RawMessage
	SupersededBytes   int
	IdleExpiresAt     *time.Time
}

func (h *BaseHandlers) callPayloadContent(
	ctx context.Context,
	record *callspkg.CallRecord,
	projected callspkg.ProjectionContent,
) (callPayloadContent, error) {
	content := callPayloadContent{}
	if childID := strings.TrimSpace(record.ChildSessionID); childID != "" && h.Sessions != nil {
		info, err := h.Sessions.Status(ctx, childID)
		if err != nil {
			return callPayloadContent{}, err
		}
		content.IdleExpiresAt = info.IdleExpiresAt
	}
	if len(projected.Prompt) > 0 {
		content.PromptPreview = boundedCallTextPreview(string(projected.Prompt), callPromptPreviewBytes)
		content.PromptBytes = len(projected.Prompt)
	}
	if record.State == callspkg.StateCompleted && len(projected.Result) > 0 {
		content.ResultPreview = boundedCallPreview(projected.Result, record.ResultBudget.MaxBytes)
	}
	if len(projected.Superseded) > 0 {
		content.SupersededPreview = boundedCallPreview(projected.Superseded, record.ResultBudget.MaxBytes)
		content.SupersededBytes = len(projected.Superseded)
	}
	return content, nil
}

func callPayload(
	record *callspkg.CallRecord,
	profile profileOwnerIdentity,
	content callPayloadContent,
) contract.CallPayload {
	payload := contract.CallPayload{
		CallID: record.CallID, ProfileID: record.ProfileID, ProfileName: profile.Name,
		Scope: string(record.Scope), WorkspaceID: record.WorkspaceID,
		Caller: contract.CallOwnerPayload{Kind: string(record.Caller.Kind), ID: record.Caller.ID},
		Actor:  contract.CallOwnerPayload{Kind: record.Actor.Kind, ID: record.Actor.ID},
		Agent:  record.AgentName, ChildSessionID: record.ChildSessionID,
		ParentSessionID: record.ParentSessionID, RootSessionID: record.GovernedRootID,
		Depth: record.Depth, State: string(record.State), Verdict: string(record.Verdict),
		ExpectDigest: record.ExpectDigest, PromptPreview: content.PromptPreview, PromptBytes: content.PromptBytes,
		ResultPreview: cloneCallJSON(content.ResultPreview),
		ResultBytes:   record.ResultBytes, ResultBudget: record.ResultBudget.MaxBytes,
		ResultOverflow: string(record.ResultBudget.Overflow), Strict: record.Strict,
		IdleTTLSeconds: durationSeconds(record.IdleTTL), IdleExpiresAt: content.IdleExpiresAt,
		FailureCode:   record.FailureCode,
		FailureDetail: record.FailureDetail, FirstIssueText: record.FirstIssueText,
		SecondIssueText: record.SecondIssueText, FinalProsePreview: record.FinalProsePreview,
		SupersededPreview: cloneCallJSON(content.SupersededPreview), SupersededBytes: content.SupersededBytes,
		RepairAttempts: record.RepairAttempts,
		Replayed:       record.Replayed, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt,
		StartedAt: timePointer(record.StartedAt), SettledAt: timePointer(record.SettledAt),
		DeadlineAt: timePointer(record.DeadlineAt),
	}
	if record.Verdict != "" {
		payload.Provenance = &contract.CallProvenancePayload{
			ProducedBy: record.AgentName, SessionID: record.ChildSessionID,
			Admitted: string(record.Verdict),
		}
	}
	return payload
}

func boundedCallTextPreview(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	preview := value[:maxBytes]
	for preview != "" && !utf8.ValidString(preview) {
		preview = preview[:len(preview)-1]
	}
	return preview
}

func (h *BaseHandlers) callCreatePayload(
	ctx context.Context,
	record *callspkg.CallRecord,
) (contract.CallCreatePayload, error) {
	payload := contract.CallCreatePayload{
		CallID: record.CallID, ChildSessionID: record.ChildSessionID,
		State: string(record.State), Replayed: record.Replayed,
	}
	if childID := strings.TrimSpace(record.ChildSessionID); childID != "" && h.Sessions != nil {
		info, err := h.Sessions.Status(ctx, childID)
		if err != nil {
			return contract.CallCreatePayload{}, err
		}
		payload.IdleExpiresAt = info.IdleExpiresAt
	}
	return payload, nil
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
		CallID: record.CallID, Text: record.Body, Delivery: publicCallDelivery(string(record.Delivery)),
		Reason: record.DeliveryReason, Attempts: record.DeliveryAttempts,
		CreatedAt: record.CreatedAt, DeliveredAt: timePointer(record.DeliveredAt),
	}
}

func publicCallDelivery(value string) string {
	switch strings.TrimSpace(value) {
	case "pending", queryFilterQueuedValue:
		return queryFilterQueuedValue
	case "injected", "delivered-into-turn":
		return "delivered-into-turn"
	case "woken", "woke":
		return "woke"
	case queryFilterFailedValue:
		return queryFilterFailedValue
	default:
		return queryFilterFailedValue
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
	cloned := value.UTC()
	return &cloned
}

func cloneCallJSON(value json.RawMessage) json.RawMessage {
	if len(value) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), value...)
}
