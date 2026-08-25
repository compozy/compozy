package calls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/contracts"
)

const publicationPreviewBytes = 4096

// Publish posts bounded, sanitized result evidence to one Network conversation.
func (s *Service) Publish(ctx context.Context, input PublishInput) (PublishReceipt, error) {
	input = normalizePublishInput(input)
	if err := validatePublishInput(input); err != nil {
		return PublishReceipt{}, err
	}
	store, err := s.publicationStore()
	if err != nil {
		return PublishReceipt{}, err
	}

	// The daemon is the sole writer. Serializing check/post/record prevents duplicate bridge calls.
	s.publishMu.Lock()
	defer s.publishMu.Unlock()
	if existing, getErr := store.GetPublication(ctx, input.CallID, input.Channel, input.ThreadID); getErr == nil {
		return PublishReceipt{NetworkMessageID: existing.NetworkMessageID, Published: false}, nil
	} else if !errors.Is(getErr, ErrPublicationNotFound) {
		return PublishReceipt{}, getErr
	}

	record, err := s.store.GetCall(ctx, s.scope(input.ProfileID, input.Scope, input.WorkspaceID), input.CallID)
	if err != nil {
		return PublishReceipt{}, err
	}
	if record.State != StateCompleted || strings.TrimSpace(record.ResultRef) == "" {
		return PublishReceipt{}, newError(
			CodePublishNotSettled,
			fmt.Sprintf("call is %s — only completed calls with a result publish", record.State),
			nil,
		)
	}
	payload, err := s.loadPublicationPayload(ctx, record)
	if err != nil {
		return PublishReceipt{}, err
	}
	messageID, err := s.newID("msg")
	if err != nil {
		return PublishReceipt{}, fmt.Errorf("calls: allocate publication message: %w", err)
	}
	if s.publisher == nil {
		return PublishReceipt{}, newError(CodePublishNoParticipation, "active Network participation is required", nil)
	}
	publisherSessionID := record.Caller.ID
	if input.Actor.Kind == "agent_session" {
		publisherSessionID = input.Actor.ID
	}
	evidence := ResultEvidence{
		CallID: record.CallID, WorkspaceID: record.WorkspaceID, SourceSessionID: publisherSessionID,
		Channel: input.Channel, ThreadID: input.ThreadID, MessageID: messageID,
		ResultPreview: payload, ResultBytes: record.ResultBytes,
		FetchPath: "/api/workspaces/" + record.WorkspaceID + "/calls/" + record.CallID + "/result",
	}
	networkMessageID, err := s.publisher.PublishResultEvidence(ctx, evidence)
	if err != nil {
		return PublishReceipt{}, newError(CodePublishNoParticipation, "active Network participation is required", err)
	}
	publication, inserted, err := store.RecordPublication(ctx, Publication{
		CallID: record.CallID, Channel: input.Channel, ThreadID: input.ThreadID,
		NetworkMessageID: networkMessageID, CreatedAt: s.now().UTC(),
	})
	if err != nil {
		return PublishReceipt{}, err
	}
	s.emitHook(ctx, HookCallPublished, HookPayload{
		ProfileID: record.ProfileID, Scope: record.Scope, WorkspaceID: record.WorkspaceID,
		CallID: record.CallID, ParentSessionID: record.ParentSessionID,
		ChildSessionID: record.ChildSessionID, RootSessionID: record.GovernedRootID,
		AgentName: record.AgentName, State: record.State, Verdict: record.Verdict,
		Actor: input.Actor, Channel: input.Channel, ThreadID: input.ThreadID,
		NetworkMessageID: publication.NetworkMessageID,
	})
	return PublishReceipt{NetworkMessageID: publication.NetworkMessageID, Published: inserted}, nil
}

func (s *Service) loadPublicationPayload(ctx context.Context, record CallRecord) (json.RawMessage, error) {
	mailbox, err := s.payloadStore()
	if err != nil {
		return nil, err
	}
	payload, err := mailbox.GetCallPayload(ctx, record.WorkspaceID, record.ResultRef)
	if err != nil {
		return nil, err
	}
	clean := json.RawMessage(payload)
	if record.ExpectDigest != "" {
		contract, resolveErr := s.registry.Resolve(ctx, record.ExpectDigest)
		if resolveErr != nil {
			return nil, resolveErr
		}
		clean, _, err = contracts.RedactPreservingContract(contract, clean)
		if err != nil {
			return nil, err
		}
	} else {
		sanitized, _, reject := contracts.SanitizeText(string(clean))
		if reject {
			clean = json.RawMessage(`"[REDACTED]"`)
		} else {
			clean = json.RawMessage(sanitized)
			if !json.Valid(clean) {
				encoded, marshalErr := json.Marshal(sanitized)
				if marshalErr != nil {
					return nil, fmt.Errorf("calls: encode sanitized publication preview: %w", marshalErr)
				}
				clean = encoded
			}
		}
	}
	if len(clean) > publicationPreviewBytes {
		encoded, marshalErr := json.Marshal(truncateUTF8(string(clean), publicationPreviewBytes))
		if marshalErr != nil {
			return nil, fmt.Errorf("calls: encode bounded publication preview: %w", marshalErr)
		}
		clean = encoded
	}
	return append(json.RawMessage(nil), clean...), nil
}

func normalizePublishInput(input PublishInput) PublishInput {
	input.ProfileID = strings.TrimSpace(input.ProfileID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.CallID = strings.TrimSpace(input.CallID)
	input.Channel = strings.TrimSpace(input.Channel)
	input.ThreadID = strings.TrimSpace(input.ThreadID)
	input.Actor.Kind = strings.TrimSpace(input.Actor.Kind)
	input.Actor.ID = strings.TrimSpace(input.Actor.ID)
	return input
}

func validatePublishInput(input PublishInput) error {
	if input.ProfileID == "" || input.CallID == "" || input.Channel == "" || input.Actor.Kind == "" || input.Actor.ID == "" {
		return newError(CodeValidation, "profile_id, call_id, channel, and actor are required", nil)
	}
	if input.Scope == ScopeWorkspace && input.WorkspaceID == "" {
		return newError(CodeValidation, "workspace scope requires workspace_id", nil)
	}
	if input.Scope != ScopeWorkspace {
		return newError(CodeValidation, "Network publication requires workspace scope", nil)
	}
	return nil
}

func (s *Service) publicationStore() (PublicationStore, error) {
	store, ok := s.store.(PublicationStore)
	if !ok {
		return nil, errors.New("calls: store does not implement publications")
	}
	return store, nil
}
