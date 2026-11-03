package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"slices"
	"time"

	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

var _ source.ApprovalReader = (*InstanceRepo)(nil)
var errSourceCAS = errors.New("heartbeat source changed during collection")

func sourceText(s string) sql.NullString { return sql.NullString{String: s, Valid: true} }
func (r *InstanceRepo) ApprovedHash(ctx context.Context, profile, dir, name string) (string, error) {
	hash, err := r.queries.GetHeartbeatScriptApproval(
		ctx,
		sqlcgen.GetHeartbeatScriptApprovalParams{ProfileID: profile, DefinitionDir: dir, ScriptName: name},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return hash, err
}
func (r *InstanceRepo) ApproveScript(ctx context.Context, profile, dir, name, hash, via string, now time.Time) error {
	return r.queries.ApproveHeartbeatScript(
		ctx,
		sqlcgen.ApproveHeartbeatScriptParams{
			ProfileID:     profile,
			DefinitionDir: dir,
			ScriptName:    name,
			Sha256:        hash,
			ApprovedAt:    store.FormatTimestamp(now),
			ApprovedVia:   via,
		},
	)
}
func (r *InstanceRepo) RevokeScript(ctx context.Context, profile, dir, name string) error {
	return r.queries.RevokeHeartbeatScript(
		ctx,
		sqlcgen.RevokeHeartbeatScriptParams{ProfileID: profile, DefinitionDir: dir, ScriptName: name},
	)
}

func (r *InstanceRepo) SourceStates(
	ctx context.Context,
	id string,
	refs []source.Ref,
	dry bool,
	now time.Time,
) ([]heartbeat.SourceState, error) {
	out := make([]heartbeat.SourceState, 0, len(refs))
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		digest, err := ref.Digest()
		if err != nil {
			return nil, err
		}
		ids = append(ids, ref.ID)
		if !dry {
			if err := r.queries.ResolveHeartbeatSource(
				ctx,
				sqlcgen.ResolveHeartbeatSourceParams{
					InstanceID: id,
					SourceID:   ref.ID,
					RefDigest:  digest,
					UpdatedAt:  store.FormatTimestamp(now),
				},
			); err != nil {
				return nil, err
			}
		}
		row, err := r.queries.GetHeartbeatSource(
			ctx,
			sqlcgen.GetHeartbeatSourceParams{InstanceID: id, SourceID: ref.ID},
		)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		state := heartbeat.SourceState{ID: ref.ID, RefDigest: digest}
		if err == nil && row.RefDigest == digest {
			state.LastPolledAt = row.LastPolledAt.String
			state.LastSuccessAt = row.LastSuccessAt.String
			state.LastItemCount = row.LastItemCount
			state.Generation = row.StateGeneration
			if row.CommittedState.Valid {
				state.State = jsontext.Value(row.CommittedState.String)
			}
			state.FailureStreak = row.FailureStreak
			if row.NextPollAt.Valid && row.NextPollAt.String != "" {
				state.NextPollAt, err = store.ParseTimestamp(row.NextPollAt.String)
				if err != nil {
					return nil, err
				}
			}
			if row.LastErrorCode.String != "" {
				state.Error = source.Failure(row.LastErrorCode.String, row.LastErrorDetail.String)
			}
		}
		out = append(out, state)
	}
	if !dry {
		data, err := json.Marshal(ids)
		if err != nil {
			return nil, err
		}
		if err := r.queries.DeleteRemovedHeartbeatSources(
			ctx,
			sqlcgen.DeleteRemovedHeartbeatSourcesParams{InstanceID: id, SourceIds: string(data)},
		); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *InstanceRepo) SourcePollStatus(
	ctx context.Context,
	id string,
	p heartbeat.PolledSource,
	now, next time.Time,
) error {
	code, detail := "", ""
	if p.Err != nil {
		code, detail = string(p.Err.Code), p.Err.Detail
	}
	return r.queries.UpdateHeartbeatSourceStatus(
		ctx,
		sqlcgen.UpdateHeartbeatSourceStatusParams{
			InstanceID:  id,
			SourceID:    p.Ref.ID,
			RefDigest:   p.RefDigest,
			Now:         sourceText(store.FormatTimestamp(now)),
			ErrorCode:   sourceText(code),
			ErrorDetail: sourceText(detail),
			ItemCount:   int64(len(p.Result.Items)),
			Streak:      p.FailureStreak,
			NextPoll:    sourceText(store.FormatTimestamp(next)),
		},
	)
}
func (r *InstanceRepo) DeferWake(ctx context.Context, id, expected string, until time.Time) (bool, error) {
	n, err := r.queries.DeferInstanceSourceWake(
		ctx,
		sqlcgen.DeferInstanceSourceWakeParams{
			InstanceID:  id,
			ExpectedDue: expected,
			NextWake:    store.FormatTimestamp(until),
		},
	)
	return n == 1, err
}

func bindHeartbeatSources(
	ctx context.Context,
	q *sqlcgen.Queries,
	id, occurrence string,
	items []heartbeat.PolledSource,
	now time.Time,
	empty bool,
) error {
	if empty &&
		slices.ContainsFunc(
			items,
			func(p heartbeat.PolledSource) bool { return len(p.Result.Items) > 0 || p.Result.More },
		) {
		return nil
	}
	for _, p := range items {
		if p.Err != nil {
			continue
		}
		state := sql.NullString{String: string(p.Result.State), Valid: len(p.Result.State) > 0}
		var n int64
		var err error
		if empty {
			if len(p.Result.Items) > 0 || p.Result.More {
				continue
			}
			n, err = q.CommitEmptyHeartbeatSource(
				ctx,
				sqlcgen.CommitEmptyHeartbeatSourceParams{
					InstanceID: id,
					SourceID:   p.Ref.ID,
					RefDigest:  p.RefDigest,
					Generation: p.Generation,
					State:      state,
					Now:        sourceText(store.FormatTimestamp(now)),
				},
			)
		} else {
			n, err = q.BindHeartbeatSource(
				ctx,
				sqlcgen.BindHeartbeatSourceParams{
					InstanceID: id,
					SourceID:   p.Ref.ID,
					RefDigest:  p.RefDigest,
					Generation: p.Generation,
					State:      state,
					Occurrence: sourceText(occurrence),
					Now:        store.FormatTimestamp(now),
				},
			)
		}
		if err != nil {
			return err
		}
		if n != 1 {
			return errSourceCAS
		}
	}
	return nil
}

func finishHeartbeatSources(ctx context.Context, q *sqlcgen.Queries, req heartbeat.FinishRequest, completed int) error {
	return q.FinishHeartbeatSources(
		ctx,
		sqlcgen.FinishHeartbeatSourcesParams{
			InstanceID: req.InstanceID,
			Occurrence: sourceText(req.OccurrenceID),
			Completed:  completed,
			Now:        store.FormatTimestamp(req.Now),
		},
	)
}
