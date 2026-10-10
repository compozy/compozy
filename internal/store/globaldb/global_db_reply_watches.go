package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
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
	if w.WorkspaceID == "" || w.SenderSessionID == "" || w.TargetWorkspaceID == "" || w.TargetSessionID == "" ||
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

func (g *SessionRepo) GetReplyWatch(ctx context.Context, id string) (store.ReplyWatch, error) {
	row, err := g.queries.GetReplyWatch(ctx, id)
	if err != nil {
		return store.ReplyWatch{}, err
	}
	return replyWatchFromSQL(row)
}

func (g *SessionRepo) ListReplyWatches(ctx context.Context, f store.ReplyWatchFilter) ([]store.ReplyWatch, error) {
	rows, err := g.queries.ListReplyWatches(
		ctx,
		sqlcgen.ListReplyWatchesParams{
			StateFilter:     f.State,
			WorkspaceFilter: f.WorkspaceID,
			SenderFilter:    f.SenderSessionID,
			TargetFilter:    f.TargetSessionID,
		},
	)
	if err != nil {
		return nil, err
	}
	out := make([]store.ReplyWatch, 0, len(rows))
	for _, row := range rows {
		w, err := replyWatchFromSQL(row)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

func (g *SessionRepo) BindReplyWatch(ctx context.Context, id, turn, queue string) error {
	return g.queries.BindReplyWatch(
		ctx,
		sqlcgen.BindReplyWatchParams{ID: id, TurnID: replyNull(turn), QueueEntryID: replyNull(queue)},
	)
}

func (g *SessionRepo) FireReplyWatch(ctx context.Context, id string, f store.ReplyWatchFire) (bool, error) {
	switch f.Outcome {
	case store.ReplyOutcomeCompleted,
		store.ReplyOutcomeFailed,
		store.ReplyOutcomeCanceled,
		store.ReplyOutcomeDropped,
		store.ReplyOutcomeUnknown:
	default:
		return false, errors.New("store: invalid reply outcome")
	}
	if utf8.RuneCountInString(f.Text) > 12000 {
		return false, errors.New("store: reply exceeds 12000 runes")
	}
	n, err := g.queries.FireReplyWatch(
		ctx,
		sqlcgen.FireReplyWatchParams{
			ID:             id,
			Outcome:        replyNull(f.Outcome),
			ReplyText:      replyNull(f.Text),
			ReplyTruncated: subagentBool(f.Truncated),
			FiredAt:        replyNull(store.FormatTimestamp(g.now())),
		},
	)
	return n == 1, err
}

func (g *SessionRepo) AbandonReplyWatch(ctx context.Context, id, reason string) error {
	return g.queries.AbandonReplyWatch(ctx, sqlcgen.AbandonReplyWatchParams{ID: id, AbandonReason: replyNull(reason)})
}

func (g *SessionRepo) DeliverReplyWatch(
	ctx context.Context,
	id string,
	entry store.SessionInputQueueInsert,
) (delivered bool, err error) {
	entry = entry.Normalize()
	if err := entry.Validate(); err != nil {
		return false, err
	}
	if entry.OwnerKind != store.SessionInputOwnerSynthetic || entry.MessageID != "prompt-reply:"+id ||
		entry.Priority != 1 {
		return false, errors.New("store: invalid reply wake input identity")
	}
	var meta struct {
		Kind string `json:"kind"`
	}
	if entry.SyntheticPrompt == nil {
		return false, errors.New("store: reply wake requires synthetic metadata")
	}
	if err := json.Unmarshal(entry.SyntheticPrompt.Metadata, &meta); err != nil {
		return false, err
	}
	if meta.Kind != "session_reply" {
		return false, errors.New("store: reply wake requires session_reply kind")
	}
	err = g.withImmediateTransaction(ctx, "deliver reply watch", func(exec globalSQLExecutor) error {
		q := sqlcgen.New(exec)
		w, err := q.GetReplyWatch(ctx, id)
		if err != nil {
			return err
		}
		if w.State != store.ReplyWatchFired {
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
			return q.AbandonReplyWatch(
				ctx,
				sqlcgen.AbandonReplyWatchParams{ID: id, AbandonReason: replyNull("sender_gone")},
			)
		}
		count, err := countPendingSessionInputs(ctx, exec, entry.SessionID)
		if err != nil {
			return err
		}
		if count >= entry.QueueCap {
			return &store.SessionInputQueueFullError{SessionID: entry.SessionID, Cap: entry.QueueCap, Count: count}
		}
		n, err := q.DeliverReplyWatch(
			ctx,
			sqlcgen.DeliverReplyWatchParams{
				ID:               id,
				DeliveredInputID: replyNull(entry.ID),
				DeliveredAt:      replyNull(store.FormatTimestamp(g.now())),
			},
		)
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

func (g *SessionRepo) ReplyWatchEvidence(
	ctx context.Context,
	w store.ReplyWatch,
) (store.SessionPromptAdmission, []store.SessionInputQueueEntry, error) {
	row, err := g.queries.ReplyWatchAdmission(ctx, w.AdmissionID)
	if err != nil {
		return store.SessionPromptAdmission{}, nil, err
	}
	admission, err := sessionPromptAdmissionFromGenerated(&row)
	if err != nil {
		return store.SessionPromptAdmission{}, nil, err
	}
	rows, err := g.queries.ReplyWatchMessageInputs(
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
