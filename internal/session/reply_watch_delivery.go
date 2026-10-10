package session

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
)

func (s *replyWatchService) Deliver(ctx context.Context, id string) error {
	w, err := s.store.GetReplyWatch(ctx, id)
	if err != nil || w.State != store.ReplyWatchFired {
		return err
	}
	sender, err := s.manager.Status(ctx, w.SenderSessionID)
	if errors.Is(err, ErrSessionNotFound) || (err == nil && sender.ArchivedAt != nil) {
		return s.abandon(ctx, id, "sender_gone")
	}
	if err != nil {
		return err
	}
	if _, live := s.manager.Get(w.SenderSessionID); !live || sender.State != StateActive {
		s.manager.logger.InfoContext(ctx, "reply_watch.delivery_deferred", "id", id, "reason", "sender_not_live")
		return nil
	}
	target, err := s.manager.Status(ctx, w.TargetSessionID)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}
	title, agent := "a deleted session", ""
	if target != nil {
		title, agent = target.Name, target.AgentName
	}
	generation, err := s.manager.currentInputGeneration(ctx, sender.ID)
	if err != nil {
		return err
	}
	inputID, err := store.NewID("inq")
	if err != nil {
		return err
	}
	turn, err := s.manager.newPromptTurnID()
	if err != nil {
		return err
	}
	meta, err := json.Marshal(acp.PromptSyntheticMeta{
		Kind:             acp.PromptSyntheticKindSessionReply,
		WakeEventID:      id,
		ChildSessionID:   w.TargetSessionID,
		Reason:           w.Outcome,
		Summary:          w.ReplyText,
		Hop:              w.Hop,
		ChildWorkspaceID: w.TargetWorkspaceID,
		ChildAgentName:   agent,
		ReplyTruncated:   w.ReplyTruncated,
	})
	if err != nil {
		return err
	}
	text := replyWakeText(w, title)
	message := "prompt-reply:" + id
	delivered, err := s.store.DeliverReplyWatch(ctx, id, store.SessionInputQueueInsert{
		ID:                inputID,
		SessionID:         sender.ID,
		OwnerKind:         store.SessionInputOwnerSynthetic,
		MessageID:         message,
		Priority:          1,
		TurnID:            turn,
		Mode:              store.SessionInputQueueModeQueue,
		Delivery:          store.SessionInputDeliveryAfterTurn,
		Text:              text,
		SessionGeneration: generation,
		QueueCap:          s.manager.busyInput.QueueCap,
		Now:               s.manager.now(),
		SyntheticPrompt: &store.SessionInputSyntheticPrompt{
			RunID:    message,
			Delivery: store.SessionInputDeliveryAfterTurn,
			Metadata: meta,
		},
	})
	if errors.Is(err, store.ErrSessionInputQueueFull) {
		s.manager.logger.InfoContext(ctx, "reply_watch.delivery_deferred", "id", id, "reason", "queue_full")
		return nil
	}
	if err != nil || !delivered {
		return err
	}
	s.manager.logger.InfoContext(ctx, "reply_watch.delivered", "id", id, "route", "queued")
	s.manager.startTrackedPromptTask(func() { s.manager.startNextQueuedInputPrompt(sender.ID) })
	return nil
}

func replyWakeText(w store.ReplyWatch, title string) string {
	text := fmt.Sprintf(
		"Session %q (%s) replied to your message %s: %s.\n---\n%s",
		title,
		w.TargetSessionID,
		w.MessageID,
		w.Outcome,
		w.ReplyText,
	)
	if w.ReplyTruncated {
		text += "\n[Reply truncated at 12000 characters. Read the full turn with compozy__session_history.]"
	}
	return text
}
