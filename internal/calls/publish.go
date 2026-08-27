package calls

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
)

const publicationPreviewBytes = 4096

// Publish posts bounded, sanitized result evidence to one Network conversation.
func (s *Service) Publish(ctx context.Context, input PublishInput) (PublishReceipt, error) {
	input = normalizePublishInput(input)
	if err := validatePublishInput(input); err != nil {
		return PublishReceipt{}, err
	}
	unlock := s.lockPublication(publicationKey(input))
	defer unlock()
	return s.publishOnce(ctx, input)
}

type publicationLock struct {
	mu   sync.Mutex
	refs int
}

func (s *Service) lockPublication(key string) func() {
	s.publicationMu.Lock()
	lock := s.publicationLocks[key]
	if lock == nil {
		lock = &publicationLock{}
		s.publicationLocks[key] = lock
	}
	lock.refs++
	s.publicationMu.Unlock()
	lock.mu.Lock()
	return func() {
		lock.mu.Unlock()
		s.publicationMu.Lock()
		defer s.publicationMu.Unlock()
		lock.refs--
		if lock.refs == 0 {
			delete(s.publicationLocks, key)
		}
	}
}

func (s *Service) publishOnce(ctx context.Context, input PublishInput) (PublishReceipt, error) {
	publicationStore, err := s.publicationStore()
	if err != nil {
		return PublishReceipt{}, err
	}
	if existing, getErr := publicationStore.GetPublication(
		ctx,
		input.CallID,
		input.Channel,
		input.ThreadID,
	); getErr == nil {
		return PublishReceipt{NetworkMessageID: existing.NetworkMessageID, Published: false}, nil
	} else if !errors.Is(
		getErr,
		ErrPublicationNotFound,
	) {
		return PublishReceipt{}, getErr
	}

	scope, err := NormalizeCallScope(CallScope{
		ProfileID: input.ProfileID, Scope: input.Scope, WorkspaceID: input.WorkspaceID,
	})
	if err != nil {
		return PublishReceipt{}, err
	}
	record, err := s.store.GetCall(ctx, scope, input.CallID)
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
	payload, err := s.loadPublicationPayload(ctx, &record)
	if err != nil {
		return PublishReceipt{}, err
	}
	return s.sendPublication(ctx, input, &record, payload, publicationStore)
}

func (s *Service) sendPublication(
	ctx context.Context,
	input PublishInput,
	record *CallRecord,
	payload json.RawMessage,
	publicationStore PublicationStore,
) (PublishReceipt, error) {
	messageID := publicationMessageID(input)
	if s.publisher == nil {
		return PublishReceipt{}, newError(CodePublishNoParticipation, "active Network participation is required", nil)
	}
	publisherSessionID := record.Caller.ID
	if input.Actor.Kind == actorKindAgentSession {
		publisherSessionID = input.Actor.ID
	}
	evidence := ResultEvidence{
		CallID: record.CallID, WorkspaceID: record.WorkspaceID, SourceSessionID: publisherSessionID,
		Channel: input.Channel, ThreadID: input.ThreadID, MessageID: messageID,
		ResultPreview: payload, ResultBytes: record.ResultBytes,
	}
	networkMessageID, err := s.publisher.PublishResultEvidence(ctx, evidence)
	if err != nil {
		if errors.Is(err, participation.ErrNotParticipating) || errors.Is(err, participation.ErrUnavailable) {
			return PublishReceipt{}, newError(
				CodePublishNoParticipation,
				"active Network participation is required",
				err,
			)
		}
		return PublishReceipt{}, newError(CodePublishFailed, "Network publication failed", err)
	}
	publication, inserted, err := publicationStore.RecordPublication(ctx, Publication{
		CallID: record.CallID, Channel: input.Channel, ThreadID: input.ThreadID,
		NetworkMessageID: networkMessageID, CreatedAt: s.now().UTC(),
	})
	if err != nil {
		s.logger.WarnContext(
			ctx,
			"calls: Network publication committed before receipt persistence failed",
			"call_id", record.CallID,
			"channel", input.Channel,
			"thread_id", input.ThreadID,
			"network_message_id", networkMessageID,
			"error", sanitizeDiagnostic(err.Error(), "publication receipt persistence failed"),
		)
		return PublishReceipt{NetworkMessageID: networkMessageID, Published: true}, nil
	}
	s.emitHook(ctx, HookCallPublished, publicationHookPayload(record, input, publication.NetworkMessageID))
	return PublishReceipt{NetworkMessageID: publication.NetworkMessageID, Published: inserted}, nil
}

func publicationMessageID(input PublishInput) string {
	digest := sha256.Sum256([]byte(publicationKey(input)))
	return fmt.Sprintf("msg-%x", digest[:16])
}

func publicationKey(input PublishInput) string {
	return strings.Join([]string{
		input.ProfileID,
		string(input.Scope),
		input.WorkspaceID,
		input.CallID,
		input.Channel,
		input.ThreadID,
	}, "\x00")
}

func publicationHookPayload(record *CallRecord, input PublishInput, networkMessageID string) HookPayload {
	return HookPayload{
		ProfileID: record.ProfileID, Scope: record.Scope, WorkspaceID: record.WorkspaceID,
		CallID: record.CallID, ParentSessionID: record.ParentSessionID,
		ChildSessionID: record.ChildSessionID, RootSessionID: record.GovernedRootID,
		AgentName: record.AgentName, State: record.State, Verdict: record.Verdict,
		Actor: input.Actor, Channel: input.Channel, ThreadID: input.ThreadID,
		NetworkMessageID: networkMessageID,
	}
}

func (s *Service) loadPublicationPayload(ctx context.Context, record *CallRecord) (json.RawMessage, error) {
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
	if input.ProfileID == "" || input.CallID == "" || input.Channel == "" || input.Actor.Kind == "" ||
		input.Actor.ID == "" {
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
