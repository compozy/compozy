package core

import (
	"context"
	"encoding/base64"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/agentinstance"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

var (
	errSkillReviewQuery = errors.New("invalid skill review query")
	// ErrSkillReviewInvalidCursor refuses a malformed cursor or one reused under different filters.
	ErrSkillReviewInvalidCursor = errors.New("invalid_cursor")
)

// skillReviewCursor binds continuation to the population and every filter it was issued for.
type skillReviewCursor struct {
	Profile, Workspace, Instance, State, Kind, DecidedVia, After string
	At                                                           time.Time
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
	case "", "pending", "approved", "rejected", "reverted":
	default:
		return result, fmt.Errorf("%w: invalid review state", errSkillReviewQuery)
	}
	switch agentinstance.SkillProposalMode(query.Kind) {
	case "", agentinstance.SkillProposalNew, agentinstance.SkillProposalEdit:
	default:
		return result, fmt.Errorf("%w: kind must be new or edit", errSkillReviewQuery)
	}
	switch query.DecidedVia {
	case "", "operator", "auto_approve":
	default:
		return result, fmt.Errorf("%w: decided_via must be operator or auto_approve", errSkillReviewQuery)
	}
	identity := skillReviewCursor{
		Profile:    query.ProfileID,
		Workspace:  query.WorkspaceID,
		Instance:   query.InstanceID,
		State:      query.State,
		Kind:       query.Kind,
		DecidedVia: query.DecidedVia,
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
	result := skillReviewMetadata(review)
	contents, err := agentinstance.ReadSkillReviewContents(review)
	if err != nil && !errors.Is(err, agentinstance.ErrSkillReviewInvalid) {
		return contract.AgentSkillReviewPayload{}, err
	}
	var automatic agentinstance.SkillAutomaticContents
	if err == nil {
		// For an edit with an unverifiable snapshot only the draft can be inventoried.
		automatic, err = contents.Draft.AutomaticContents()
		if contents.Result != nil {
			automatic, err = contents.Result.AutomaticContents()
		}
	}
	if err == nil && contents.Base != nil {
		var base agentinstance.SkillAutomaticContents
		base, err = contents.Base.AutomaticContents()
		result.BaseRunsAutomatically = automaticContentsPayload(base)
	}
	if err != nil {
		// A draft that no longer verifies still reads as a review, so it can be seen and rejected.
		result.BaseRunsAutomatically = nil
		result.Problem = agentinstance.ErrSkillReviewInvalid.Error()
		return result, nil
	}
	result.RunsAutomatically = *automaticContentsPayload(automatic)
	if contents.SnapshotProblem != nil {
		result.Problem = agentinstance.ErrSkillReviewSnapshotInvalid.Error()
	}
	if detail {
		addSkillReviewFiles(&result, contents)
	}
	return result, nil
}

func automaticContentsPayload(automatic agentinstance.SkillAutomaticContents) *contract.AgentSkillAutomaticContents {
	return &contract.AgentSkillAutomaticContents{
		Hooks:      automatic.Hooks,
		MCPServers: automatic.MCPServers,
		Scripts:    automatic.Scripts,
	}
}

// addSkillReviewFiles adds the proposed files and, for an edit, the per-path changes and the base
// snapshot: text redacted like drafts, non-UTF-8 files as name and size.
func addSkillReviewFiles(result *contract.AgentSkillReviewPayload, contents agentinstance.SkillReviewContents) {
	result.Files = make([]contract.AgentSkillDraftFile, 0, len(contents.Draft.Files))
	for _, file := range contents.Draft.Files {
		result.Files = append(result.Files, contract.AgentSkillDraftFile{Path: file.Path, Content: file.Content})
	}
	for _, change := range contents.Changes {
		result.Changes = append(result.Changes, contract.AgentSkillFileChange{Path: change.Path, Change: change.Change})
	}
	for _, file := range contents.Base {
		item := contract.AgentSkillBaseFile{Path: file.Path}
		if utf8.Valid(file.Data) {
			item.Content = redact.String(string(file.Data))
		} else {
			item.Binary, item.Size = true, int64(len(file.Data))
		}
		result.BaseFiles = append(result.BaseFiles, item)
	}
}

func skillReviewMetadata(review agentinstance.SkillReview) contract.AgentSkillReviewPayload {
	return contract.AgentSkillReviewPayload{
		ID:          review.ID,
		InstanceID:  review.InstanceID,
		ProfileID:   review.ProfileID,
		WorkspaceID: review.WorkspaceID,
		Agent:       review.AgentName,
		Name:        review.SkillName,
		State:       review.State,
		Kind:        string(review.Kind),
		Digest:      review.Digest,
		BaseDigest:  review.BaseDigest,
		Lesson:      skillLessonPayload(review.Lesson),
		// Never null: legacy reviews carry an empty list.
		EvidenceSessionIDs: append([]string{}, review.EvidenceSessionIDs...),
		CreatedAt:          review.CreatedAt,
		DecidedAt:          review.DecidedAt,
		DecidedBy:          review.DecidedBy,
		DecidedVia:         review.DecidedVia,
		RevertedBy:         review.RevertedBy,
		RevertedAt:         review.RevertedAt,
	}
}

func skillLessonPayload(lesson agentinstance.SkillLesson) *contract.AgentSkillLesson {
	if !lesson.Recorded() {
		return nil
	}
	return &contract.AgentSkillLesson{
		Cue: lesson.Cue, Lesson: lesson.Lesson, Evidence: lesson.Evidence, Confidence: lesson.Confidence,
	}
}

func (identity skillReviewCursor) apply(cursor string, query *agentinstance.SkillReviewQuery) error {
	if cursor == "" {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return fmt.Errorf("%w: cursor is malformed", ErrSkillReviewInvalidCursor)
	}
	var previous skillReviewCursor
	if err := json.Unmarshal(raw, &previous); err != nil {
		return fmt.Errorf("%w: cursor is malformed", ErrSkillReviewInvalidCursor)
	}
	after, at := previous.After, previous.At
	previous.After, previous.At = "", time.Time{}
	if previous != identity || after == "" || at.IsZero() {
		return fmt.Errorf("%w: cursor does not match this query", ErrSkillReviewInvalidCursor)
	}
	query.AfterID, query.AfterCreatedAt = after, at
	return nil
}
