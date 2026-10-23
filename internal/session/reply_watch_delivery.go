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
	sender, ready, err := s.replySenderReady(ctx, w)
	if err != nil || !ready {
		return err
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
		Summary:          replySummary(w.ReplyText),
		Hop:              replyWakeHop(w),
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

// replyWakeHop is the hop a reply wake carries into the sender: the sender's own hop when it sent the
// message (the message hop minus one). A completion wake hands control back to the sender, so a sequential
// orchestration (stage done → dispatch next child) restarts from the sender's depth instead of adding one
// per stage; forwarding A→B→C still adds one per send and stays bounded by MaxSessionMessageHops.
func replyWakeHop(w store.ReplyWatch) int {
	return max(w.Hop-1, 0)
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
		text += fmt.Sprintf(
			"\n[Reply truncated at 12000 characters. Read compozy__session_history with session_id=%s, "+
				"type=agent_message, turn_id=%s for the full reply.]",
			w.TargetSessionID, w.TurnID,
		)
	}
	return text
}

// replySenderReady reports whether the sender can take the wake now. A gone sender abandons the watch;
// a sender CompozyOS stopped is resumed and redelivers on resume; any other stopped sender waits.
func (s *replyWatchService) replySenderReady(ctx context.Context, w store.ReplyWatch) (*Info, bool, error) {
	sender, err := s.manager.Status(ctx, w.SenderSessionID)
	if errors.Is(err, ErrSessionNotFound) || (err == nil && sender.ArchivedAt != nil) {
		return nil, false, s.abandon(ctx, w.ID, "sender_gone")
	}
	if err != nil {
		return nil, false, err
	}
	if _, live := s.manager.Get(w.SenderSessionID); live && sender.State == StateActive {
		return sender, true, nil
	}
	if replySenderWakeable(sender) && s.manager.replySenderResumable(ctx, w.SenderSessionID) {
		s.resumeSender(ctx, w.ID, w.SenderSessionID)
		return nil, false, nil
	}
	s.manager.logger.InfoContext(ctx, "reply_watch.delivery_deferred", "id", w.ID, "reason", "sender_not_live")
	return nil, false, nil
}

// replySummary is the reply body the Web card renders: empty for an empty answer, so the card shows its
// own empty state instead of the provider-facing placeholder.
func replySummary(text string) string {
	if text == replyNoText {
		return ""
	}
	return text
}
