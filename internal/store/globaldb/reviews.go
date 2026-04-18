package globaldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
)

// ReviewRound captures one durable review-round projection row.
type ReviewRound struct {
	ID              string
	WorkflowID      string
	RoundNumber     int
	Provider        string
	PRRef           string
	ResolvedCount   int
	UnresolvedCount int
	UpdatedAt       time.Time
}

// ReviewIssue captures one durable review-issue projection row.
type ReviewIssue struct {
	ID          string
	RoundID     string
	IssueNumber int
	Severity    string
	Status      string
	SourcePath  string
	UpdatedAt   time.Time
}

// ErrReviewRoundNotFound reports a missing review round.
var ErrReviewRoundNotFound = errors.New("globaldb: review round not found")

// GetLatestReviewRound loads the newest review round for one workflow.
func (g *GlobalDB) GetLatestReviewRound(ctx context.Context, workflowID string) (ReviewRound, error) {
	if err := g.requireContext(ctx, "get latest review round"); err != nil {
		return ReviewRound{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, round_number, provider, pr_ref, resolved_count, unresolved_count, updated_at
		 FROM review_rounds
		 WHERE workflow_id = ?
		 ORDER BY round_number DESC
		 LIMIT 1`,
		strings.TrimSpace(workflowID),
	)
	reviewRound, err := scanReviewRound(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReviewRound{}, ErrReviewRoundNotFound
		}
		return ReviewRound{}, err
	}
	return reviewRound, nil
}

// GetReviewRound loads one review round by workflow and round number.
func (g *GlobalDB) GetReviewRound(ctx context.Context, workflowID string, round int) (ReviewRound, error) {
	if err := g.requireContext(ctx, "get review round"); err != nil {
		return ReviewRound{}, err
	}

	row := g.db.QueryRowContext(
		ctx,
		`SELECT id, workflow_id, round_number, provider, pr_ref, resolved_count, unresolved_count, updated_at
		 FROM review_rounds
		 WHERE workflow_id = ? AND round_number = ?`,
		strings.TrimSpace(workflowID),
		round,
	)
	reviewRound, err := scanReviewRound(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReviewRound{}, ErrReviewRoundNotFound
		}
		return ReviewRound{}, err
	}
	return reviewRound, nil
}

// ListReviewIssues loads the durable issue rows for one review round.
func (g *GlobalDB) ListReviewIssues(ctx context.Context, roundID string) ([]ReviewIssue, error) {
	if err := g.requireContext(ctx, "list review issues"); err != nil {
		return nil, err
	}

	rows, err := g.db.QueryContext(
		ctx,
		`SELECT id, round_id, issue_number, severity, status, source_path, updated_at
		 FROM review_issues
		 WHERE round_id = ?
		 ORDER BY issue_number ASC, id ASC`,
		strings.TrimSpace(roundID),
	)
	if err != nil {
		return nil, fmt.Errorf("globaldb: query review issues for round %q: %w", strings.TrimSpace(roundID), err)
	}
	defer func() {
		_ = rows.Close()
	}()

	issues := make([]ReviewIssue, 0)
	for rows.Next() {
		issue, scanErr := scanReviewIssue(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("globaldb: iterate review issues for round %q: %w", strings.TrimSpace(roundID), err)
	}
	return issues, nil
}

func scanReviewRound(scanner interface {
	Scan(dest ...any) error
}) (ReviewRound, error) {
	var (
		row       ReviewRound
		provider  sql.NullString
		prRef     sql.NullString
		updatedAt string
	)
	if err := scanner.Scan(
		&row.ID,
		&row.WorkflowID,
		&row.RoundNumber,
		&provider,
		&prRef,
		&row.ResolvedCount,
		&row.UnresolvedCount,
		&updatedAt,
	); err != nil {
		return ReviewRound{}, fmt.Errorf("globaldb: scan review round: %w", err)
	}
	row.Provider = strings.TrimSpace(provider.String)
	row.PRRef = strings.TrimSpace(prRef.String)
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAt)
	if err != nil {
		return ReviewRound{}, fmt.Errorf("globaldb: parse review round updated_at: %w", err)
	}
	row.UpdatedAt = parsedUpdatedAt
	return row, nil
}

func scanReviewIssue(scanner interface {
	Scan(dest ...any) error
}) (ReviewIssue, error) {
	var (
		row       ReviewIssue
		updatedAt string
	)
	if err := scanner.Scan(
		&row.ID,
		&row.RoundID,
		&row.IssueNumber,
		&row.Severity,
		&row.Status,
		&row.SourcePath,
		&updatedAt,
	); err != nil {
		return ReviewIssue{}, fmt.Errorf("globaldb: scan review issue: %w", err)
	}
	parsedUpdatedAt, err := store.ParseTimestamp(updatedAt)
	if err != nil {
		return ReviewIssue{}, fmt.Errorf("globaldb: parse review issue updated_at: %w", err)
	}
	row.UpdatedAt = parsedUpdatedAt
	return row, nil
}
