package agentinstance

import (
	"context"
	"errors"
	"time"
)

const SkillReviewPending = "pending"

var (
	ErrSkillReviewLimit = errors.New("skill_review_limit")
	ErrSkillReviewStale = errors.New("skill_review_stale")
	// ErrSkillReviewInvalid means the reviewed digest still matches but the draft cannot be installed:
	// its bytes, name or file paths fail verification. Only rejection can settle such a review.
	ErrSkillReviewInvalid      = errors.New("skill_review_invalid")
	ErrSkillReviewNotFound     = errors.New("skill_review_not_found")
	ErrSkillReviewOperatorOnly = errors.New("skill_review_operator_only")
)

type SkillReview struct {
	ID              string
	InstanceID      string
	ProfileID       string
	WorkspaceID     string
	AgentName       string
	SkillName       string
	Digest          string
	State           string
	DraftPath       string
	SourceSessionID string
	DecidedBy       string
	CreatedAt       time.Time
	DecidedAt       *time.Time
	Kind            SkillProposalMode
	// BaseDigest is the tree digest of the captured base of an edit; "" for new skills and legacy rows.
	BaseDigest         string
	Lesson             SkillLesson
	EvidenceSessionIDs []string
	// DecidedVia is "" while pending, "operator" or "auto_approve" once decided.
	DecidedVia string
}

type SkillReviewQuery struct {
	ProfileID      string
	WorkspaceID    string
	InstanceID     string
	State          string
	Kind           string
	DecidedVia     string
	AfterCreatedAt time.Time
	AfterID        string
	Limit          int
}

type SkillReviewStore interface {
	// CreateReview applies proposal identity (instance, skill, digest, base digest): a pending review
	// with that identity is returned with duplicate=true and no new row or report; a rejected one
	// refuses with ErrSkillProposePreviouslyRejected naming it.
	CreateReview(context.Context, SkillReview, ReportInput) (review SkillReview, duplicate bool, err error)
	GetReview(context.Context, string) (SkillReview, error)
	ListReviews(context.Context, SkillReviewQuery) ([]SkillReview, error)
	DecideReview(context.Context, string, string, string, string) error
}
