package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ store.ReplyWatchStore = (*SessionRepo)(nil)

func (g *SessionRepo) InsertReplyWatchTx(
	ctx context.Context,
	exec store.DBTX,
	w store.ReplyWatchRegistration,
) (store.ReplyWatch, error) {
	if w.CreatedAt.IsZero() {
		w.CreatedAt = g.now()
	}
	return insertReplyWatchTx(ctx, exec, w)
}

func insertReplyWatchTx(
	ctx context.Context,
	exec store.DBTX,
	w store.ReplyWatchRegistration,
) (store.ReplyWatch, error) {
	// An empty sender or target workspace is the User scope (USW BR-1); the sessions identify it.
	if w.SenderSessionID == "" || w.TargetSessionID == "" ||
		w.MessageID == "" ||
		w.AdmissionID == "" ||
		w.Hop < 1 ||
		w.Hop > 8 {
		return store.ReplyWatch{}, errors.New(
			"store: reply watch requires sender, target, admission, message, and hop 1..8",
		)
	}
	id := store.ReplyWatchID(w.TargetSessionID, w.MessageID)
	q := sqlcgen.New(exec)
	admission, err := q.ReplyWatchAdmission(ctx, w.AdmissionID)
	if err != nil {
		return store.ReplyWatch{}, err
	}
	if admission.WorkspaceID != w.TargetWorkspaceID || admission.SessionID != w.TargetSessionID ||
		admission.MessageID != w.MessageID {
		return store.ReplyWatch{}, errors.New("store: reply watch admission scope mismatch")
	}
	err = q.InsertReplyWatch(ctx, sqlcgen.InsertReplyWatchParams{
		ID: id, WorkspaceID: w.WorkspaceID, SenderSessionID: w.SenderSessionID,
		TargetWorkspaceID: w.TargetWorkspaceID, TargetSessionID: w.TargetSessionID,
		MessageID: w.MessageID, AdmissionID: w.AdmissionID, TurnID: replyNull(w.TurnID),
		QueueEntryID: replyNull(w.QueueEntryID), Hop: int64(w.Hop), CreatedAt: store.FormatTimestamp(w.CreatedAt),
	})
	if err != nil {
		return store.ReplyWatch{}, err
	}
	row, err := q.GetReplyWatchByMessage(
		ctx,
		sqlcgen.GetReplyWatchByMessageParams{TargetSessionID: w.TargetSessionID, MessageID: w.MessageID},
	)
	if err != nil {
		return store.ReplyWatch{}, err
	}
	if row.SenderSessionID != w.SenderSessionID || row.WorkspaceID != w.WorkspaceID ||
		row.AdmissionID != w.AdmissionID ||
		row.Hop != int64(w.Hop) {
		return store.ReplyWatch{}, store.ErrSessionPromptIdempotencyConflict
	}
	return replyWatchFromSQL(row)
}

func (g *SessionRepo) SupersedeReplyWatchFollowUps(ctx context.Context, id string) error {
	return g.withImmediateTransaction(ctx, "supersede reply follow-ups", func(exec globalSQLExecutor) error {
		superseded, err := sqlcgen.New(exec).SupersedeReplyWatchFollowUps(ctx, id)
		if err != nil {
			return err
		}
		for _, older := range superseded {
			if older.State == store.ReplyWatchAbandoned {
				recordSupersededReplyWatch(ctx, exec, older.ID)
			}
		}
		return nil
	})
}

func recordSupersededReplyWatch(ctx context.Context, exec store.DBTX, id string) {
	if transaction, ok := exec.(*outboxSignalExecutor); ok {
		transaction.supersededReplyWatches = append(transaction.supersededReplyWatches, id)
		return
	}
	logSupersededReplyWatch(ctx, id)
}

func logSupersededReplyWatch(ctx context.Context, id string) {
	slog.InfoContext(ctx, "reply_watch.abandoned", "id", id, "reason", "superseded")
}

// ReplyWatchByMessage reads the watch armed for one message admitted into a target session.
func (g *SessionRepo) ReplyWatchByMessage(
	ctx context.Context,
	targetSessionID, messageID string,
) (store.ReplyWatch, bool, error) {
	row, err := g.readQueries.GetReplyWatchByMessage(
		ctx,
		sqlcgen.GetReplyWatchByMessageParams{TargetSessionID: targetSessionID, MessageID: messageID},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ReplyWatch{}, false, nil
	}
	if err != nil {
		return store.ReplyWatch{}, false, err
	}
	w, err := replyWatchFromSQL(row)
	return w, err == nil, err
}

func (g *SessionRepo) GetReplyWatch(ctx context.Context, id string) (store.ReplyWatch, error) {
	row, err := g.readQueries.GetReplyWatch(ctx, id)
	if err != nil {
		return store.ReplyWatch{}, err
	}
	return replyWatchFromSQL(row)
}

func (g *SessionRepo) ListReplyWatches(ctx context.Context, f store.ReplyWatchFilter) ([]store.ReplyWatch, error) {
	rows, err := g.readQueries.ListReplyWatches(ctx, sqlcgen.ListReplyWatchesParams{
		StateFilter:     f.State,
		WorkspaceFilter: f.WorkspaceID,
		SenderFilter:    f.SenderSessionID,
		TargetFilter:    f.TargetSessionID,
	})
	return mapReplyWatches(rows, err)
}

// ListPendingReplyWatchesBySender reads one sender's armed and fired watches through its index.
func (g *SessionRepo) ListPendingReplyWatchesBySender(ctx context.Context, sender string) ([]store.ReplyWatch, error) {
	rows, err := g.readQueries.ListPendingReplyWatchesBySender(ctx, sender)
	return mapReplyWatches(rows, err)
}

func (g *SessionRepo) ListPendingReplyWatchesByTarget(ctx context.Context, target string) ([]store.ReplyWatch, error) {
	rows, err := g.readQueries.ListPendingReplyWatchesByTarget(ctx, target)
	return mapReplyWatches(rows, err)
}

func mapReplyWatches(rows []sqlcgen.SessionPromptReplyWatch, err error) ([]store.ReplyWatch, error) {
	if err != nil {
		return nil, err
	}
	out := make([]store.ReplyWatch, 0, len(rows))
	for i := range rows {
		watch, err := replyWatchFromSQL(rows[i])
		if err != nil {
			return nil, err
		}
		out = append(out, watch)
	}
	return out, nil
}

func (g *SessionRepo) BindReplyWatch(ctx context.Context, id, turn, queue string) error {
	return g.executeWriteQuery(
		ctx,
		"globaldb.BindReplyWatch",
		func(ctx context.Context, queries *sqlcgen.Queries) error {
			return queries.BindReplyWatch(
				ctx,
				sqlcgen.BindReplyWatchParams{ID: id, TurnID: replyNull(turn), QueueEntryID: replyNull(queue)},
			)
		},
	)
}

func (g *SessionRepo) FireReplyWatch(ctx context.Context, id string, f store.ReplyWatchFire) (bool, error) {
	switch f.Outcome {
	case store.ReplyOutcomeCompleted,
		store.ReplyOutcomeFailed,
		store.ReplyOutcomeCanceled,
		store.ReplyOutcomeDropped,
		store.ReplyOutcomeUnknown,
		store.ReplyOutcomeExpired:
	default:
		return false, errors.New("store: invalid reply outcome")
	}
	if utf8.RuneCountInString(f.Text) > 12000 {
		return false, errors.New("store: reply exceeds 12000 runes")
	}
	var fired bool
	err := g.withImmediateTransaction(ctx, "fire reply watch", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		if f.InterimWait != "" {
			newer, err := q.HasNewerReplyWatch(ctx, id)
			if err != nil {
				return err
			}
			if newer {
				f.FollowUpAfter, f.InterimWait = 0, ""
			}
		}
		n, err := q.FireReplyWatch(ctx, sqlcgen.FireReplyWatchParams{
			ID: id, Outcome: replyNull(f.Outcome), ReplyText: replyNull(f.Text),
			ReplyTruncated: subagentBool(f.Truncated), FiredAt: replyNull(store.FormatTimestamp(g.now())),
			ResultTurnID: replyNull(f.TurnID), ExpectedAfter: f.ExpectedAfter,
			NextAfter: max(f.ExpectedAfter, f.FollowUpAfter), InterimWait: replyNull(f.InterimWait),
		})
		fired = n == 1
		return err
	})
	return fired && err == nil, err
}

func (g *SessionRepo) AbandonReplyWatch(ctx context.Context, id, reason string) (bool, error) {
	n, err := executeWriteQueryResult(
		ctx, g.repoBase, "globaldb.AbandonReplyWatch",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.AbandonReplyWatch(
				ctx,
				sqlcgen.AbandonReplyWatchParams{ID: id, AbandonReason: replyNull(reason)},
			)
		},
	)
	return n == 1, err
}

func (g *SessionRepo) DeliverReplyWatch(
	ctx context.Context,
	id string,
	entry store.SessionInputQueueInsert,
	interim bool,
) (delivered bool, err error) {
	entry = entry.Normalize()
	if err := validateReplyWakeInput(id, entry); err != nil {
		return false, err
	}
	err = g.withImmediateTransaction(ctx, "deliver reply watch", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		w, err := q.GetReplyWatch(ctx, id)
		if err != nil {
			return err
		}
		watch, err := replyWatchFromSQL(w)
		if err != nil {
			return err
		}
		if w.State != store.ReplyWatchFired || entry.MessageID != watch.WakeMessageID() ||
			(watch.InterimWait != "") != interim {
			return nil
		}
		if w.SenderSessionID != entry.SessionID {
			return errors.New("store: reply wake sender mismatch")
		}
		exists, err := q.ReplyWatchSenderExists(
			ctx,
			sqlcgen.ReplyWatchSenderExistsParams{ID: w.SenderSessionID, WorkspaceID: w.WorkspaceID},
		)
		if err != nil {
			return err
		}
		if !exists {
			_, err := q.AbandonReplyWatch(
				ctx,
				sqlcgen.AbandonReplyWatchParams{ID: id, AbandonReason: replyNull("sender_gone")},
			)
			return err
		}
		// The wake joins the sender's current queue generation, read inside this transaction: a clear that
		// advanced it after the caller prepared the entry must not strand a delivered wake in a dead generation.
		generation, err := q.GetSessionInputGeneration(ctx, entry.SessionID)
		if err != nil {
			return fmt.Errorf("store: read reply wake generation: %w", err)
		}
		entry.SessionGeneration = generation
		if err := checkSessionInputQueueCap(ctx, exec, entry); err != nil {
			return err
		}
		n, err := g.finishReplyWatchDelivery(ctx, q, watch, entry.ID)
		if err != nil || n == 0 {
			return err
		}
		if _, err := insertSessionInputQueueEntry(ctx, exec, entry); err != nil {
			return err
		}
		delivered = true
		return nil
	})
	return delivered && err == nil, err
}

func validateReplyWakeInput(id string, entry store.SessionInputQueueInsert) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	base := "prompt-reply:" + id
	if entry.OwnerKind != store.SessionInputOwnerSynthetic || entry.Priority != 1 ||
		(entry.MessageID != base && entry.MessageID != base+":follow-up") {
		return errors.New("store: invalid reply wake input identity")
	}
	if entry.SyntheticPrompt == nil {
		return errors.New("store: reply wake requires synthetic metadata")
	}
	var meta struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta); err != nil {
		return err
	}
	if meta.Kind != "session_reply" {
		return errors.New("store: reply wake requires session_reply kind")
	}
	return nil
}

func (g *SessionRepo) finishReplyWatchDelivery(
	ctx context.Context, q *sqlcgen.Queries, watch store.ReplyWatch, inputID string,
) (int64, error) {
	deliveredAt := replyNull(store.FormatTimestamp(g.now()))
	if watch.InterimWait != "" {
		return q.DeliverInterimReplyWatch(ctx, sqlcgen.DeliverInterimReplyWatchParams{
			ID: watch.ID, DeliveredInputID: replyNull(inputID), DeliveredAt: deliveredAt,
		})
	}
	return q.DeliverReplyWatch(ctx, sqlcgen.DeliverReplyWatchParams{
		ID: watch.ID, DeliveredInputID: replyNull(inputID), DeliveredAt: deliveredAt,
	})
}

func (g *SessionRepo) ReplyWatchEvidence(
	ctx context.Context,
	w store.ReplyWatch,
) (store.SessionPromptAdmission, []store.SessionInputQueueEntry, error) {
	row, err := g.readQueries.ReplyWatchAdmission(ctx, w.AdmissionID)
	if err != nil {
		return store.SessionPromptAdmission{}, nil, err
	}
	admission, err := sessionPromptAdmissionFromGenerated(&row)
	if err != nil {
		return store.SessionPromptAdmission{}, nil, err
	}
	rows, err := g.readQueries.ReplyWatchMessageInputs(
		ctx,
		sqlcgen.ReplyWatchMessageInputsParams{SessionID: w.TargetSessionID, MessageID: w.MessageID},
	)
	if err != nil {
		return admission, nil, err
	}
	inputs := make([]store.SessionInputQueueEntry, 0, len(rows))
	for i := range rows {
		input, err := sessionInputQueueFromGenerated(&rows[i])
		if err != nil {
			return admission, nil, err
		}
		inputs = append(inputs, input)
	}
	return admission, inputs, nil
}

func replyNull(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func replyWatchFromSQL(row sqlcgen.SessionPromptReplyWatch) (store.ReplyWatch, error) {
	w := store.ReplyWatch{
		ID: row.ID, WorkspaceID: row.WorkspaceID, SenderSessionID: row.SenderSessionID,
		TargetWorkspaceID: row.TargetWorkspaceID, TargetSessionID: row.TargetSessionID,
		MessageID: row.MessageID, AdmissionID: row.AdmissionID, TurnID: row.TurnID.String,
		QueueEntryID: row.QueueEntryID.String, DeliveredInputID: row.DeliveredInputID.String,
		AbandonReason: row.AbandonReason.String, Hop: int(row.Hop), State: row.State,
		Outcome: row.Outcome.String, ReplyText: row.ReplyText.String, ReplyTruncated: row.ReplyTruncated != 0,
		FollowUpAfter: row.FollowUpAfter, InterimWait: row.InterimWait.String,
	}
	var err error
	w.CreatedAt, err = store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return w, err
	}
	w.FiredAt, err = parseOptionalSessionInputTimestamp(row.FiredAt)
	if err != nil {
		return w, err
	}
	w.DeliveredAt, err = parseOptionalSessionInputTimestamp(row.DeliveredAt)
	return w, err
}
