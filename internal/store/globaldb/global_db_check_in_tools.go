package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

var _ toolspkg.CheckInRecorder = checkInToolRecorder{}

func (r *InstanceRepo) Begin(
	ctx context.Context,
	a toolspkg.CheckInAttempt,
	p toolspkg.CheckInPolicy,
) (toolspkg.CheckInDecision, error) {
	var decision toolspkg.CheckInDecision
	err := store.ExecuteWriteOperation(
		ctx,
		r.db,
		"begin check-in tool call",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			counts, err := q.CountCheckInToolCalls(ctx, p.OccurrenceID)
			if err != nil {
				return err
			}
			decision = toolspkg.CheckInDecision{
				Seq:     int(counts.Total) + 1,
				Allowed: a.DenyReason == "",
				Reason:  a.DenyReason,
			}
			if counts.Total >= int64(p.MaxToolCalls) ||
				decision.Allowed && a.Access == toolspkg.AccessWrite && counts.Writes >= int64(p.MaxWriteCalls) {
				decision.Allowed = false
				decision.Reason = string(toolspkg.ReasonCheckInToolBudgetExhausted)
			}
			outcome, choice, effect := "running", "allowed", "maybe"
			if !decision.Allowed {
				outcome, choice, effect = "denied", "denied", "none"
			}
			backend := "native"
			if a.Backend == toolspkg.BackendMCP {
				backend = "mcp"
			}
			now := store.FormatTimestamp(r.now())
			return q.InsertCheckInToolCall(
				ctx,
				sqlcgen.InsertCheckInToolCallParams{
					InstanceID:   p.InstanceID,
					OccurrenceID: p.OccurrenceID,
					Seq:          int64(decision.Seq),
					ToolID:       string(a.ToolID),
					Backend:      backend,
					McpServer:    sourceText(a.MCPServer),
					Access:       string(a.Access),
					Decision:     choice,
					DenyReason:   sourceText(decision.Reason),
					Outcome:      outcome,
					Effect:       effect,
					ArgsPreview:  sourceText(a.ArgsPreview),
					CreatedAt:    now,
					UpdatedAt:    now,
				},
			)
		},
	)
	return decision, err
}

func (r *InstanceRepo) FinishToolCall(
	ctx context.Context,
	occurrence string,
	seq int,
	outcome, code string,
	d time.Duration,
) error {
	effect := "maybe"
	if outcome == "ok" {
		effect = "done"
	}
	if outcome == "denied" {
		effect = "none"
	}
	return r.executeWriteQuery(ctx, "finish check-in tool call", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.FinishCheckInToolCall(
			ctx,
			sqlcgen.FinishCheckInToolCallParams{
				Outcome:      outcome,
				Effect:       effect,
				ErrorCode:    sourceText(code),
				DurationMs:   sql.NullInt64{Int64: d.Milliseconds(), Valid: true},
				UpdatedAt:    store.FormatTimestamp(r.now()),
				OccurrenceID: occurrence,
				Seq:          int64(seq),
			},
		)
	})
}
func (r *InstanceRepo) ApprovedAuthority(ctx context.Context, profile, dir string) (string, error) {
	value, err := r.readQueries.GetHeartbeatAuthorityApproval(
		ctx,
		sqlcgen.GetHeartbeatAuthorityApprovalParams{ProfileID: profile, DefinitionDir: dir},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}
func (r *InstanceRepo) ApproveAuthority(ctx context.Context, profile, dir, hash, via string, now time.Time) error {
	return r.executeWriteQuery(ctx, "approve heartbeat authority", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.ApproveHeartbeatAuthority(
			ctx,
			sqlcgen.ApproveHeartbeatAuthorityParams{
				ProfileID:     profile,
				DefinitionDir: dir,
				Sha256:        hash,
				ApprovedAt:    store.FormatTimestamp(now),
				ApprovedVia:   via,
			},
		)
	})
}
func (r *InstanceRepo) RevokeAuthority(ctx context.Context, profile, dir string) error {
	return r.executeWriteQuery(ctx, "revoke heartbeat authority", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.RevokeHeartbeatAuthority(
			ctx,
			sqlcgen.RevokeHeartbeatAuthorityParams{ProfileID: profile, DefinitionDir: dir},
		)
	})
}

type checkInToolRecorder struct{ repo *InstanceRepo }

func (r checkInToolRecorder) Begin(
	ctx context.Context,
	a toolspkg.CheckInAttempt,
	p toolspkg.CheckInPolicy,
) (toolspkg.CheckInDecision, error) {
	return r.repo.Begin(ctx, a, p)
}

func (r checkInToolRecorder) Finish(
	ctx context.Context,
	occurrence string,
	seq int,
	outcome, code string,
	d time.Duration,
) error {
	return r.repo.FinishToolCall(ctx, occurrence, seq, outcome, code, d)
}
func (r *InstanceRepo) CheckInRecorder() toolspkg.CheckInRecorder { return checkInToolRecorder{r} }

func (r *InstanceRepo) CheckInToolCalls(
	ctx context.Context,
	instance, occurrence string,
) ([]agentinstance.CheckInToolCall, error) {
	rows, err := r.readQueries.ListCheckInToolCalls(
		ctx,
		sqlcgen.ListCheckInToolCallsParams{InstanceID: instance, OccurrenceID: occurrence},
	)
	if err != nil {
		return nil, err
	}
	result := make([]agentinstance.CheckInToolCall, 0, len(rows))
	for _, row := range rows {
		result = append(
			result,
			agentinstance.CheckInToolCall{
				Seq:         row.Seq,
				ToolID:      row.ToolID,
				Backend:     row.Backend,
				MCPServer:   row.McpServer.String,
				Access:      row.Access,
				Decision:    row.Decision,
				DenyReason:  row.DenyReason.String,
				Outcome:     row.Outcome,
				Effect:      row.Effect,
				ErrorCode:   row.ErrorCode.String,
				ArgsPreview: row.ArgsPreview.String,
				DurationMS:  row.DurationMs.Int64,
			},
		)
	}
	return result, nil
}

func (r *InstanceRepo) SweepCheckInToolCalls(ctx context.Context, cutoff time.Time) error {
	return r.executeWriteQuery(ctx, "sweep check-in tool calls", func(ctx context.Context, q *sqlcgen.Queries) error {
		return q.SweepCheckInToolCalls(ctx, store.FormatTimestamp(cutoff))
	})
}

func (r *InstanceRepo) CheckInOccurrence(
	ctx context.Context,
	instance, occurrence string,
) (agentinstance.CheckIn, error) {
	row, err := r.readQueries.GetInstanceCheckInOccurrence(
		ctx,
		sqlcgen.GetInstanceCheckInOccurrenceParams{InstanceID: instance, OccurrenceID: occurrence},
	)
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.CheckIn{}, agentinstance.ErrNotFound
	}
	if err != nil {
		return agentinstance.CheckIn{}, err
	}
	at, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return agentinstance.CheckIn{}, err
	}
	calls, err := r.CheckInToolCalls(ctx, instance, occurrence)
	return agentinstance.CheckIn{
		ID:           row.ID,
		OccurrenceID: row.OccurrenceID,
		Result:       row.Result,
		Reason:       row.Reason,
		Detail:       row.Detail,
		CreatedAt:    at,
		ToolsStatus:  row.ToolsStatus,
		ToolCalls:    calls,
	}, err
}

func (r *InstanceRepo) MarkAuthorityNotice(ctx context.Context, profile, dir, digest string) (bool, error) {
	var rows int64
	err := r.executeWriteQuery(
		ctx,
		"mark check-in authority notice",
		func(ctx context.Context, q *sqlcgen.Queries) error {
			var err error
			rows, err = q.MarkHeartbeatAuthorityNotice(
				ctx,
				sqlcgen.MarkHeartbeatAuthorityNoticeParams{
					ProfileID:      profile,
					DefinitionDir:  dir,
					NotifiedSha256: digest,
				},
			)
			return err
		},
	)
	return rows == 1, err
}

func (r *InstanceRepo) CheckInSessionBinding(ctx context.Context, id string) (string, string, error) {
	row, err := r.readQueries.GetCheckInSessionBinding(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	return row.InstanceID, row.OccurrenceID, err
}
