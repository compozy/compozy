package core

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var errSkillReviewQuery = errors.New("invalid skill review query")

type skillReviewCursor struct {
	Profile, Workspace, Instance, State, After string
	At                                         time.Time
}

// SkillReviewsPage binds continuation to the authorized population, then hydrates only the page.
func (api *AgentInstanceAPI) SkillReviewsPage(ctx context.Context, query agentinstance.SkillReviewQuery,
	actor workspaceaccess.ActorRef, cursor string) (contract.AgentSkillReviewsResponse, error) {
	result := contract.AgentSkillReviewsResponse{Reviews: []contract.AgentSkillReviewPayload{}}
	if api.SkillProposals == nil {
		return result, errors.New("skill reviews unavailable")
	}
	if query.Limit < 1 || query.Limit > 100 {
		return result, fmt.Errorf("%w: limit must be from 1 to 100", errSkillReviewQuery)
	}
	switch query.State {
	case "", "pending", "approved", "rejected":
	default:
		return result, fmt.Errorf("%w: invalid review state", errSkillReviewQuery)
	}
	identity := skillReviewCursor{
		Profile:   query.ProfileID,
		Workspace: query.WorkspaceID,
		Instance:  query.InstanceID,
		State:     query.State,
	}
	if err := identity.apply(cursor, &query); err != nil {
		return result, err
	}

	// Agent readers can see only their own instance, selected before the page cut.
	if !actor.Operator {
		instance, err := api.Store.Get(ctx, query.InstanceID)
		if err != nil {
			return result, err
		}
		if instance.ProfileID != query.ProfileID || instance.WorkspaceID != query.WorkspaceID {
			return result, agentinstance.ErrForbidden
		}
		if err := api.Authority.Authorize(ctx, actor, instance, "skill_reviews"); err != nil {
			return result, err
		}
	}
	rows, err := api.SkillProposals.Store.ListReviews(ctx, query)
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		payload, err := skillReviewPayload(row, false)
		if err != nil {
			return result, err
		}
		result.Reviews = append(result.Reviews, payload)
	}
	if len(rows) == query.Limit {
		last := rows[len(rows)-1]
		query.AfterID, query.AfterCreatedAt, query.Limit = last.ID, last.CreatedAt, 1
		more, err := api.SkillProposals.Store.ListReviews(ctx, query)
		if err != nil {
			return result, err
		}
		if len(more) > 0 {
			identity.After, identity.At = last.ID, last.CreatedAt
			raw, err := json.Marshal(identity)
			if err != nil {
				return result, err
			}
			result.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
		}
	}
	return result, nil
}

func skillReviewPayload(review agentinstance.SkillReview, detail bool) (contract.AgentSkillReviewPayload, error) {
	draft, err := agentinstance.ReadSkillDraft(review.DraftPath, review.Digest)
	if err != nil {
		return contract.AgentSkillReviewPayload{}, err
	}
	automatic, err := draft.AutomaticContents()
	if err != nil {
		return contract.AgentSkillReviewPayload{}, err
	}
	result := contract.AgentSkillReviewPayload{
		ID:          review.ID,
		InstanceID:  review.InstanceID,
		ProfileID:   review.ProfileID,
		WorkspaceID: review.WorkspaceID,
		Agent:       review.AgentName,
		Name:        review.SkillName,
		State:       review.State,
		Digest:      review.Digest,
		CreatedAt:   review.CreatedAt,
		DecidedAt:   review.DecidedAt,
		DecidedBy:   review.DecidedBy,
		RunsAutomatically: contract.AgentSkillAutomaticContents{
			Hooks:      automatic.Hooks,
			MCPServers: automatic.MCPServers,
			Scripts:    automatic.Scripts,
		},
	}
	if detail {
		result.Files = make([]contract.AgentSkillDraftFile, 0, len(draft.Files))
		for _, file := range draft.Files {
			result.Files = append(result.Files, contract.AgentSkillDraftFile{Path: file.Path, Content: file.Content})
		}
	}
	return result, nil
}

func (identity skillReviewCursor) apply(cursor string, query *agentinstance.SkillReviewQuery) error {
	if cursor == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("%w: invalid cursor", errSkillReviewQuery)
	}
	var previous skillReviewCursor
	if err := json.Unmarshal(raw, &previous); err != nil {
		return fmt.Errorf("%w: invalid cursor", errSkillReviewQuery)
	}
	if previous.Profile != identity.Profile || previous.Workspace != identity.Workspace ||
		previous.Instance != identity.Instance || previous.State != identity.State ||
		previous.After == "" || previous.At.IsZero() {
		return fmt.Errorf("%w: cursor does not match this query", errSkillReviewQuery)
	}
	query.AfterID, query.AfterCreatedAt = previous.After, previous.At
	return nil
}
