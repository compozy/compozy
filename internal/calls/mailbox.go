package calls

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/contracts"
)

const maxDeliveryAttempts = 3

// SendMessage admits one durable peer message and attempts boundary delivery.
func (s *Service) SendMessage(ctx context.Context, input SendMessageInput) (MessageRecord, error) {
	mailbox, err := s.mailboxStore()
	if err != nil {
		return MessageRecord{}, err
	}
	input.ProfileID = strings.TrimSpace(input.ProfileID)
	input.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	input.From.Kind = strings.TrimSpace(input.From.Kind)
	input.From.ID = strings.TrimSpace(input.From.ID)
	input.To = strings.TrimSpace(input.To)
	input.CallID = strings.TrimSpace(input.CallID)
	if input.Scope == "" {
		if input.WorkspaceID == "" {
			input.Scope = ScopeGlobal
		} else {
			input.Scope = ScopeWorkspace
		}
	}
	if err := validateMessageIdentity(input); err != nil {
		return MessageRecord{}, err
	}
	input, err = s.resolveParentMessageTarget(ctx, input)
	if err != nil {
		return MessageRecord{}, err
	}
	clean, _, reject := contracts.SanitizeText(input.Body)
	if reject {
		return MessageRecord{}, newError(CodeValidation, "message contains unsafe secret material", nil)
	}
	if strings.TrimSpace(clean) == "" {
		return MessageRecord{}, newError(CodeValidation, "message body is required", nil)
	}
	if len([]byte(clean)) > s.messageMaxBytes {
		rejectErr := newError(
			CodeMessageTooLarge,
			fmt.Sprintf("message is %d bytes; maximum is %d", len([]byte(clean)), s.messageMaxBytes),
			nil,
		)
		s.emitMessageRejected(ctx, input, rejectErr)
		return MessageRecord{}, rejectErr
	}
	messageID, err := s.newID("msg")
	if err != nil {
		return MessageRecord{}, fmt.Errorf("calls: generate message id: %w", err)
	}
	digest := sha256.Sum256([]byte(clean))
	now := s.now().UTC()
	record, err := mailbox.AcceptMessage(ctx, MessageAdmission{
		Record: MessageRecord{
			MessageID: messageID, ProfileID: input.ProfileID, Scope: input.Scope,
			WorkspaceID: input.WorkspaceID, From: input.From, CallID: input.CallID,
			Body: clean, DedupHash: hex.EncodeToString(digest[:]), CreatedAt: now,
		},
		Target: input.To, DedupWindow: s.messageDedup,
		RateLimit:  s.config.Messages.RateLimitPerMinute,
		PendingCap: s.config.Messages.PendingCap,
	})
	if err != nil {
		s.emitMessageRejected(ctx, input, err)
		return MessageRecord{}, err
	}
	s.emitHook(ctx, HookCallMessageSent, HookPayload{
		ProfileID: record.ProfileID, Scope: record.Scope, WorkspaceID: record.WorkspaceID,
		CallID: record.CallID, MessageID: record.MessageID,
		Actor: Actor{Kind: record.From.Kind, ID: record.From.ID}, Delivery: string(record.Delivery),
	})
	return record, nil
}

func (s *Service) resolveParentMessageTarget(ctx context.Context, input SendMessageInput) (SendMessageInput, error) {
	if input.To != "parent" {
		return input, nil
	}
	if input.From.Kind != "session" {
		return SendMessageInput{}, newError(CodeTargetDenied, "only a child session may address parent", nil)
	}
	call, err := s.store.GetOpenCallForChild(ctx, CallScope{
		ProfileID: input.ProfileID, Scope: input.Scope, WorkspaceID: input.WorkspaceID,
	}, input.From.ID)
	if err != nil {
		return SendMessageInput{}, err
	}
	input.To = call.ParentSessionID
	if input.CallID == "" {
		input.CallID = call.CallID
	}
	return input, nil
}

func (s *Service) emitMessageRejected(ctx context.Context, input SendMessageInput, err error) {
	var callErr *Error
	if !errors.As(err, &callErr) {
		return
	}
	switch callErr.Code {
	case CodeMessageTooLarge, CodeMessageRateLimited, CodeMessageDuplicate, CodeMessagePendingCap:
	default:
		return
	}
	s.emitHook(ctx, HookCallMessageRejected, HookPayload{
		ProfileID:      input.ProfileID,
		Scope:          input.Scope,
		WorkspaceID:    input.WorkspaceID,
		CallID:         input.CallID,
		ChildSessionID: input.To,
		Actor:          Actor{Kind: input.From.Kind, ID: input.From.ID},
		Reason:         string(callErr.Code),
	})
}

func validateMessageIdentity(input SendMessageInput) error {
	switch {
	case input.ProfileID == "":
		return newError(CodeValidation, "profile_id is required", nil)
	case input.From.Kind != "session" && input.From.Kind != "operator":
		return newError(CodeValidation, "message sender must be session or operator", nil)
	case input.From.ID == "":
		return newError(CodeValidation, "message sender id is required", nil)
	case input.To == "":
		return newError(CodeValidation, "message target is required", nil)
	case input.Scope == ScopeWorkspace && input.WorkspaceID == "":
		return newError(CodeValidation, "workspace scope requires workspace_id", nil)
	case input.Scope == ScopeGlobal && input.WorkspaceID != "":
		return newError(CodeValidation, "global scope requires an empty workspace_id", nil)
	case input.Scope != ScopeGlobal && input.Scope != ScopeWorkspace:
		return newError(CodeValidation, fmt.Sprintf("unsupported scope %q", input.Scope), nil)
	default:
		return nil
	}
}

// Message returns one mailbox message from an exact ownership scope.
func (s *Service) Message(ctx context.Context, scope CallScope, messageID string) (MessageRecord, error) {
	mailbox, err := s.mailboxStore()
	if err != nil {
		return MessageRecord{}, err
	}
	scope, err = NormalizeCallScope(scope)
	if err != nil {
		return MessageRecord{}, err
	}
	if scope.ProfileID == "" {
		return MessageRecord{}, newError(CodeValidation, "profile_id is required", nil)
	}
	return mailbox.GetMessage(ctx, scope, strings.TrimSpace(messageID))
}

// RenderPeerMessage frames untrusted peer text within the configured byte limit.
func RenderPeerMessage(message MessageRecord, maxBytes int) string {
	agent := strings.TrimSpace(message.FromAgentName)
	if agent == "" {
		agent = "unknown"
	}
	origin := fmt.Sprintf("from agent %s (%s), not the operator", agent, strings.TrimSpace(message.From.ID))
	body, _, reject := contracts.SanitizeText(message.Body)
	if reject {
		body = "[message removed: unsafe secret material]"
	}
	body = escapeUntrustedFrameText(body)
	prefix := origin + "\n<untrusted-agent-message>\n"
	suffix := "\n</untrusted-agent-message>"
	if maxBytes <= 0 {
		return prefix + body + suffix
	}
	bodyBudget := maxBytes - len([]byte(prefix)) - len([]byte(suffix))
	if bodyBudget < 0 {
		return truncateUTF8(origin, maxBytes)
	}
	return prefix + truncateUTF8(body, bodyBudget) + suffix
}

func escapeUntrustedFrameText(value string) string {
	return strings.ReplaceAll(value, "<", `\u003c`)
}

func truncateUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	raw := []byte(value)
	if len(raw) <= maxBytes {
		return value
	}
	raw = raw[:maxBytes]
	for len(raw) > 0 && !utf8.Valid(raw) {
		raw = raw[:len(raw)-1]
	}
	return string(raw)
}

// DrainDeliveries attempts pending messages for one recipient at a runtime boundary.
func (s *Service) DrainDeliveries(ctx context.Context, recipientSessionID string, limit int) error {
	if s.invoker == nil {
		return errors.New("calls: session invoker is required for delivery")
	}
	mailbox, err := s.mailboxStore()
	if err != nil {
		return err
	}
	deliveries, err := mailbox.ListPendingDeliveries(ctx, strings.TrimSpace(recipientSessionID), limit)
	if err != nil {
		return err
	}
	var errs []error
	for _, item := range deliveries {
		content, buildErr := s.deliveryContent(ctx, item)
		if buildErr != nil {
			errs = append(errs, s.failDelivery(ctx, item, "payload_unavailable", buildErr))
			continue
		}
		outcome, deliverErr := s.invoker.DeliverAtBoundary(ctx, Delivery{
			CallID: item.SubjectID, RecipientSessionID: item.RecipientSessionID,
			Body: content.body, Kind: item.Kind, WakeEventID: item.WakeEventID,
			Metadata: content.metadata,
		})
		if deliverErr != nil {
			errs = append(errs, s.failDelivery(ctx, item, "delivery_error", deliverErr))
			continue
		}
		if outcome.State == DeliveryStatePending {
			continue
		}
		if outcome.State != DeliveryStateAttention && outcome.State != DeliveryStateInjected &&
			outcome.State != DeliveryStateWoken {
			errs = append(errs, s.failDelivery(
				ctx,
				item,
				"invalid_delivery_outcome",
				fmt.Errorf("unsupported delivery outcome %q", outcome.State),
			))
			continue
		}
		if outcome.State == DeliveryStateWoken {
			if clearErr := mailbox.ClearCallChildIdleClock(
				ctx,
				item.RecipientSessionID,
				s.now().UTC(),
			); clearErr != nil {
				errs = append(errs, clearErr)
				continue
			}
		}
		updated, updateErr := mailbox.RecordDelivery(ctx, DeliveryUpdate{
			DeliveryID: item.DeliveryID, State: outcome.State, Reason: outcome.Reason,
			At: s.now().UTC(), MaxAttempts: maxDeliveryAttempts,
		})
		if updateErr != nil {
			errs = append(errs, updateErr)
			continue
		}
		if item.Kind == DeliveryKindMessage {
			message, loadErr := mailbox.GetMessage(ctx, item.OwnerScope(), item.SubjectID)
			if loadErr != nil {
				errs = append(errs, loadErr)
				continue
			}
			s.emitHook(ctx, HookCallMessageDelivered, HookPayload{
				ProfileID: message.ProfileID, Scope: message.Scope, WorkspaceID: message.WorkspaceID,
				CallID: message.CallID, MessageID: message.MessageID,
				ChildSessionID: item.RecipientSessionID, Delivery: string(updated.State),
			})
		}
	}
	return errors.Join(errs...)
}

func (s *Service) failDelivery(ctx context.Context, item DeliveryRecord, reason string, cause error) error {
	mailbox, storeErr := s.mailboxStore()
	if storeErr != nil {
		return errors.Join(cause, storeErr)
	}
	_, err := mailbox.RecordDelivery(ctx, DeliveryUpdate{
		DeliveryID: item.DeliveryID, State: DeliveryStatePending, Reason: reason,
		At: s.now().UTC(), MaxAttempts: maxDeliveryAttempts,
	})
	return errors.Join(
		fmt.Errorf("calls: deliver %q: %w", item.DeliveryID, safeDeliveryCause(cause)),
		err,
	)
}

func safeDeliveryCause(cause error) error {
	if cause == nil {
		return errors.New("unknown delivery failure")
	}
	if errors.Is(cause, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	clean, _, reject := contracts.SanitizeText(cause.Error())
	if reject || strings.TrimSpace(clean) == "" {
		return errors.New("delivery failed with unsafe diagnostic material")
	}
	return errors.New(clean)
}

type durableDeliveryContent struct {
	body     string
	metadata DeliveryMetadata
}

func (s *Service) deliveryContent(ctx context.Context, delivery DeliveryRecord) (durableDeliveryContent, error) {
	mailbox, err := s.mailboxStore()
	if err != nil {
		return durableDeliveryContent{}, err
	}
	switch delivery.Kind {
	case DeliveryKindMessage:
		message, err := mailbox.GetMessage(ctx, delivery.OwnerScope(), delivery.SubjectID)
		if err != nil {
			return durableDeliveryContent{}, err
		}
		return durableDeliveryContent{
			body: RenderPeerMessage(message, s.messageMaxBytes),
			metadata: DeliveryMetadata{
				MessageID: message.MessageID, CallID: message.CallID,
				DeliveryKind: string(delivery.Kind), Reason: "call_message", WakeEventID: delivery.WakeEventID,
			},
		}, nil
	case DeliveryKindFollowUp:
		call, err := s.store.GetCallForSettlement(ctx, delivery.OwnerScope(), delivery.SubjectID)
		if err != nil {
			return durableDeliveryContent{}, err
		}
		prompt, err := mailbox.GetCallPayload(ctx, call.WorkspaceID, call.PromptRef)
		if err != nil {
			return durableDeliveryContent{}, err
		}
		return durableDeliveryContent{
			body: string(prompt), metadata: deliverySyntheticMetadata(delivery, &call, "call_follow_up"),
		}, nil
	case DeliveryKindCompletion:
		call, err := s.store.GetCallForSettlement(ctx, delivery.OwnerScope(), delivery.SubjectID)
		if err != nil {
			return durableDeliveryContent{}, err
		}
		return durableDeliveryContent{
			body:     RenderCompletionWake(&call),
			metadata: deliverySyntheticMetadata(delivery, &call, "call_completion"),
		}, nil
	case DeliveryKindRepair:
		call, err := s.store.GetCallForSettlement(ctx, delivery.OwnerScope(), delivery.SubjectID)
		if err != nil {
			return durableDeliveryContent{}, err
		}
		body, _, reject := contracts.SanitizeText(call.FirstIssueText)
		if reject || strings.TrimSpace(body) == "" {
			return durableDeliveryContent{}, errors.New("calls: repair delivery has no safe issue text")
		}
		return durableDeliveryContent{
			body: body, metadata: deliverySyntheticMetadata(delivery, &call, "call_repair"),
		}, nil
	default:
		return durableDeliveryContent{}, fmt.Errorf("calls: unsupported delivery kind %q", delivery.Kind)
	}
}

func deliverySyntheticMetadata(
	delivery DeliveryRecord,
	call *CallRecord,
	reason string,
) DeliveryMetadata {
	return DeliveryMetadata{
		CallID: call.CallID, CallState: string(call.State),
		ResultBytes: call.ResultBytes, ContractDigest: call.ExpectDigest,
		DeliveryKind: string(delivery.Kind), Reason: reason, WakeEventID: delivery.WakeEventID,
	}
}

func (s *Service) mailboxStore() (MailboxStore, error) {
	if s.mailbox == nil {
		return nil, errors.New("calls: store does not implement mailbox persistence")
	}
	return s.mailbox, nil
}

// FenceReapSession prevents new work from racing with session reaping.
func (s *Service) FenceReapSession(ctx context.Context, input ReapedSession) (bool, error) {
	mailbox, err := s.mailboxStore()
	if err != nil {
		return false, err
	}
	result, err := mailbox.FenceSessionReap(ctx, SessionReapFence{
		SessionID: strings.TrimSpace(input.SessionID),
		Reason:    strings.TrimSpace(input.Reason),
		At:        s.now().UTC(),
	})
	if err != nil {
		return false, err
	}
	for index := range result.SettledCalls {
		record := &result.SettledCalls[index]
		s.notifyWaiters(record.CallID)
		s.emitTerminalTransition(ctx, StateRunning, record)
	}
	return result.Allowed, nil
}

// FailRecipientDeliveries terminalizes pending deliveries for a stopped recipient.
func (s *Service) FailRecipientDeliveries(ctx context.Context, sessionID, reason string) error {
	mailbox, err := s.mailboxStore()
	if err != nil {
		return err
	}
	return mailbox.FailPendingDeliveriesForRecipient(
		ctx, strings.TrimSpace(sessionID), strings.TrimSpace(reason), s.now().UTC(),
	)
}

// FinalizeReapedSession closes a fenced session after its runtime stops.
func (s *Service) FinalizeReapedSession(ctx context.Context, input ReapedSession) error {
	mailbox, err := s.mailboxStore()
	if err != nil {
		return err
	}
	input.SessionID = strings.TrimSpace(input.SessionID)
	input.Reason = strings.TrimSpace(input.Reason)
	if err := mailbox.FinalizeReapedSession(ctx, input.SessionID, input.Reason, s.now().UTC()); err != nil {
		return err
	}
	s.emitHook(ctx, HookCallReaped, HookPayload{
		ProfileID:       input.ProfileID,
		Scope:           input.Scope,
		WorkspaceID:     input.WorkspaceID,
		ChildSessionID:  input.SessionID,
		ParentSessionID: input.ParentSessionID,
		RootSessionID:   input.RootSessionID,
		AgentName:       input.AgentName,
		Reason:          input.Reason,
	})
	return nil
}
