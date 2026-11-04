package globaldb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

type SkillReviewRepo struct{ *repoBase }

var _ agentinstance.SkillReviewStore = (*SkillReviewRepo)(nil)

func (g *GlobalDB) SkillReviews() *SkillReviewRepo {
	return &SkillReviewRepo{repoBase: newRepoBase(g.db, g.readDB, g.now, &g.closed)}
}

func (r *SkillReviewRepo) CreateReview(
	ctx context.Context,
	input agentinstance.SkillReview,
	report agentinstance.ReportInput,
) (agentinstance.SkillReview, error) {
	if err := r.checkReady(ctx, "create skill review"); err != nil {
		return agentinstance.SkillReview{}, err
	}
	if input.ID == "" || input.InstanceID == "" || input.ProfileID == "" || input.SkillName == "" ||
		input.Digest == "" ||
		input.DraftPath == "" {
		return agentinstance.SkillReview{}, errors.New("skill review identity, digest and draft path are required")
	}
	if err := report.Validate(); err != nil {
		return agentinstance.SkillReview{}, err
	}
	reportID, err := store.NewID("arpt")
	if err != nil {
		return agentinstance.SkillReview{}, err
	}
	var result agentinstance.SkillReview
	err = store.ExecuteWriteOperation(
		ctx,
		r.db,
		"create skill review",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			count, err := q.InsertAgentSkillReview(ctx, sqlcgen.InsertAgentSkillReviewParams{
				ID:          input.ID,
				InstanceID:  input.InstanceID,
				ProfileID:   input.ProfileID,
				WorkspaceID: input.WorkspaceID,
				AgentName:   input.AgentName,
				SkillName:   input.SkillName,
				Digest:      input.Digest,
				DraftPath:   input.DraftPath,
				SourceSessionID: sql.NullString{
					String: input.SourceSessionID,
					Valid:  input.SourceSessionID != "",
				},
				CreatedAt: store.FormatTimestamp(r.now()),
			})
			if err != nil {
				return err
			}
			row, err := q.GetAgentSkillReviewByDigest(
				ctx,
				sqlcgen.GetAgentSkillReviewByDigestParams{
					InstanceID: input.InstanceID,
					SkillName:  input.SkillName,
					Digest:     input.Digest,
				},
			)
			if errors.Is(err, sql.ErrNoRows) {
				return agentinstance.ErrSkillReviewLimit
			}
			if err != nil {
				return err
			}
			result, err = skillReview(row)
			if err != nil || count == 0 {
				return err
			}
			return q.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{
				ID: reportID, InstanceID: input.InstanceID, SourceKey: report.SourceKey, Kind: report.Kind,
				Payload: string(report.Payload), Fingerprint: report.Fingerprint, CreatedAt: row.CreatedAt,
			})
		},
	)
	return result, err
}

func (r *SkillReviewRepo) GetReview(ctx context.Context, id string) (agentinstance.SkillReview, error) {
	if err := r.checkReady(ctx, "get skill review"); err != nil {
		return agentinstance.SkillReview{}, err
	}
	row, err := r.readQueries.GetAgentSkillReview(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.SkillReview{}, agentinstance.ErrSkillReviewNotFound
	}
	if err != nil {
		return agentinstance.SkillReview{}, err
	}
	return skillReview(row)
}

func (r *SkillReviewRepo) ListReviews(
	ctx context.Context,
	query agentinstance.SkillReviewQuery,
) ([]agentinstance.SkillReview, error) {
	if err := r.checkReady(ctx, "list skill reviews"); err != nil {
		return nil, err
	}
	if query.ProfileID == "" || query.Limit < 1 || query.Limit > 100 {
		return nil, errors.New("skill review query requires profile and limit from 1 to 100")
	}
	after := ""
	if !query.AfterCreatedAt.IsZero() {
		after = store.FormatTimestamp(query.AfterCreatedAt)
	}
	rows, err := r.readQueries.ListAgentSkillReviews(ctx, sqlcgen.ListAgentSkillReviewsParams{
		ProfileID:      query.ProfileID,
		WorkspaceID:    query.WorkspaceID,
		InstanceID:     query.InstanceID,
		ReviewState:    query.State,
		AfterCreatedAt: after,
		AfterID:        query.AfterID,
		PageLimit:      int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]agentinstance.SkillReview, 0, len(rows))
	for _, row := range rows {
		review, err := skillReview(row)
		if err != nil {
			return nil, err
		}
		result = append(result, review)
	}
	return result, nil
}

func (r *SkillReviewRepo) DecideReview(ctx context.Context, id, digest, state, operator string) error {
	if err := r.checkReady(ctx, "decide skill review"); err != nil {
		return err
	}
	if id == "" || digest == "" || operator == "" || (state != "approved" && state != "rejected") {
		return errors.New("skill review decision requires identity, digest, operator and terminal state")
	}
	count, err := executeWriteQueryResult(
		ctx,
		r.repoBase,
		"globaldb.DecideAgentSkillReview",
		func(ctx context.Context, queries *sqlcgen.Queries) (int64, error) {
			return queries.DecideAgentSkillReview(ctx, sqlcgen.DecideAgentSkillReviewParams{
				ID: id, Digest: digest, State: state, DecidedBy: operator,
				DecidedAt: sql.NullString{String: store.FormatTimestamp(r.now()), Valid: true},
			})
		},
	)
	if err != nil {
		return err
	}
	if count != 1 {
		return agentinstance.ErrSkillReviewStale
	}
	return nil
}

func skillReview(row sqlcgen.AgentSkillReview) (agentinstance.SkillReview, error) {
	created, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return agentinstance.SkillReview{}, err
	}
	result := agentinstance.SkillReview{
		ID:              row.ID,
		InstanceID:      row.InstanceID,
		ProfileID:       row.ProfileID,
		WorkspaceID:     row.WorkspaceID,
		AgentName:       row.AgentName,
		SkillName:       row.SkillName,
		Digest:          row.Digest,
		State:           row.State,
		DraftPath:       row.DraftPath,
		SourceSessionID: row.SourceSessionID.String,
		DecidedBy:       row.DecidedBy,
		CreatedAt:       created,
	}
	if row.DecidedAt.Valid {
		decided, err := store.ParseTimestamp(row.DecidedAt.String)
		if err != nil {
			return agentinstance.SkillReview{}, err
		}
		result.DecidedAt = &decided
	}
	return result, nil
}
