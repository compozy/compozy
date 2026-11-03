package agentinstance

import (
	"context"
	"errors"
	"time"
)

const SkillReviewPending = "pending"

var (
	ErrSkillReviewLimit        = errors.New("skill_review_limit")
	ErrSkillReviewStale        = errors.New("skill_review_stale")
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
}

type SkillReviewQuery struct {
	ProfileID      string
	WorkspaceID    string
	InstanceID     string
	State          string
	AfterCreatedAt time.Time
	AfterID        string
	Limit          int
}

type SkillReviewStore interface {
	CreateReview(context.Context, SkillReview, ReportInput) (SkillReview, error)
	GetReview(context.Context, string) (SkillReview, error)
	ListReviews(context.Context, SkillReviewQuery) ([]SkillReview, error)
	DecideReview(context.Context, string, string, string, string) error
}
