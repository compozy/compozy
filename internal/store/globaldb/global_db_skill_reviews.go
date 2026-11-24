package globaldb

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"

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
) (agentinstance.SkillReview, bool, error) {
	if err := r.checkReady(ctx, "create skill review"); err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	if input.ID == "" || input.InstanceID == "" || input.ProfileID == "" || input.SkillName == "" ||
		input.Digest == "" ||
		input.DraftPath == "" {
		return agentinstance.SkillReview{}, false, errors.New(
			"skill review identity, digest and draft path are required",
		)
	}
	if err := report.Validate(); err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	params, err := skillReviewInsertParams(input)
	if err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	reportID, err := store.NewID("arpt")
	if err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	var result agentinstance.SkillReview
	duplicate := false
	err = store.ExecuteWriteOperation(
		ctx,
		r.db,
		"create skill review",
		func(ctx context.Context, tx *store.WriteTx) error {
			q := sqlcgen.New(tx)
			existing, found, err := skillReviewByIdentity(ctx, q, input)
			if err != nil || found {
				result, duplicate = existing, err == nil
				return err
			}
			createdAt := store.FormatTimestamp(r.now())
			params.CreatedAt = createdAt
			count, err := q.InsertAgentSkillReview(ctx, params)
			if err != nil {
				return err
			}
			if count == 0 {
				return agentinstance.ErrSkillReviewLimit
			}
			row, err := q.GetAgentSkillReview(ctx, input.ID)
			if err != nil {
				return err
			}
			if result, err = skillReview(row); err != nil {
				return err
			}
			return q.InsertAgentInstanceReport(ctx, sqlcgen.InsertAgentInstanceReportParams{
				ID: reportID, InstanceID: input.InstanceID, SourceKey: report.SourceKey, Kind: report.Kind,
				Payload: string(report.Payload), Fingerprint: report.Fingerprint, CreatedAt: createdAt,
			})
		},
	)
	if errors.Is(err, agentinstance.ErrSkillProposePreviouslyRejected) {
		return result, false, err
	}
	if err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	return result, duplicate, nil
}

// skillReviewInsertParams maps a new pending review onto its row; CreatedAt is set inside the write.
func skillReviewInsertParams(input agentinstance.SkillReview) (sqlcgen.InsertAgentSkillReviewParams, error) {
	kind := input.Kind
	if kind == "" {
		kind = agentinstance.SkillProposalNew
	}
	lesson, evidence, err := encodeSkillLesson(input.Lesson, input.EvidenceSessionIDs)
	if err != nil {
		return sqlcgen.InsertAgentSkillReviewParams{}, err
	}
	return sqlcgen.InsertAgentSkillReviewParams{
		ID:                 input.ID,
		InstanceID:         input.InstanceID,
		ProfileID:          input.ProfileID,
		WorkspaceID:        input.WorkspaceID,
		AgentName:          input.AgentName,
		SkillName:          input.SkillName,
		Digest:             input.Digest,
		DraftPath:          input.DraftPath,
		SourceSessionID:    sql.NullString{String: input.SourceSessionID, Valid: input.SourceSessionID != ""},
		Kind:               string(kind),
		BaseDigest:         input.BaseDigest,
		Lesson:             lesson,
		EvidenceSessionIds: evidence,
	}, nil
}

// skillReviewByIdentity applies proposal identity: a pending match is a duplicate, a rejected match
// refuses the proposal and is returned with the error.
func skillReviewByIdentity(
	ctx context.Context,
	q *sqlcgen.Queries,
	input agentinstance.SkillReview,
) (agentinstance.SkillReview, bool, error) {
	row, err := q.GetAgentSkillReviewByIdentity(ctx, sqlcgen.GetAgentSkillReviewByIdentityParams{
		InstanceID: input.InstanceID,
		SkillName:  input.SkillName,
		Digest:     input.Digest,
		BaseDigest: input.BaseDigest,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return agentinstance.SkillReview{}, false, nil
	}
	if err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	existing, err := skillReview(row)
	if err != nil {
		return agentinstance.SkillReview{}, false, err
	}
	if row.State != agentinstance.SkillReviewPending {
		return existing, true, fmt.Errorf("%w: review %s rejected these contents",
			agentinstance.ErrSkillProposePreviouslyRejected, row.ID)
	}
	return existing, true, nil
}

// SessionScope resolves an evidence session's profile and workspace.
func (r *SkillReviewRepo) SessionScope(ctx context.Context, sessionID string) (string, string, error) {
	if err := r.checkReady(ctx, "read evidence session"); err != nil {
		return "", "", err
	}
	row, err := r.readQueries.GetAgentSkillEvidenceSession(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", agentinstance.ErrSkillProposeEvidenceNotFound
	}
	if err != nil {
		return "", "", err
	}
	return row.ProfileID, row.WorkspaceID, nil
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
		ProfileID:        query.ProfileID,
		WorkspaceID:      query.WorkspaceID,
		InstanceID:       query.InstanceID,
		ReviewState:      query.State,
		ReviewKind:       query.Kind,
		ReviewDecidedVia: query.DecidedVia,
		AfterCreatedAt:   after,
		AfterID:          query.AfterID,
		PageLimit:        int64(query.Limit),
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
		Kind:            agentinstance.SkillProposalMode(row.Kind),
		BaseDigest:      row.BaseDigest,
		DecidedVia:      row.DecidedVia,
	}
	if err := json.Unmarshal([]byte(row.Lesson), &result.Lesson); err != nil {
		return agentinstance.SkillReview{}, fmt.Errorf("decode skill review lesson: %w", err)
	}
	if err := json.Unmarshal([]byte(row.EvidenceSessionIds), &result.EvidenceSessionIDs); err != nil {
		return agentinstance.SkillReview{}, fmt.Errorf("decode skill review evidence: %w", err)
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

// encodeSkillLesson stores a recorded lesson as JSON; legacy-shaped reviews keep `{}` and `[]`.
func encodeSkillLesson(lesson agentinstance.SkillLesson, evidence []string) (string, string, error) {
	lessonJSON := "{}"
	if lesson.Recorded() {
		data, err := json.Marshal(lesson)
		if err != nil {
			return "", "", err
		}
		lessonJSON = string(data)
	}
	if evidence == nil {
		evidence = []string{}
	}
	evidenceJSON, err := json.Marshal(evidence)
	if err != nil {
		return "", "", err
	}
	return lessonJSON, string(evidenceJSON), nil
}
