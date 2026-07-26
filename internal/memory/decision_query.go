package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

// DecisionRecord is the redaction-safe query model for persisted decisions.
type DecisionRecord struct {
	Decision    memcontract.Decision
	WorkspaceID string
	AgentName   string
	AgentTier   memcontract.AgentTier
	AppliedAt   *time.Time
}

// DecisionListQuery filters persisted controller decisions.
type DecisionListQuery struct {
	Scope          memcontract.Scope
	WorkspaceID    string
	AgentName      string
	AgentTier      memcontract.AgentTier
	Operation      string
	TargetFilename string
	Since          time.Time
	Reason         string
	Limit          int
}

// ListDecisionRecords returns persisted controller decisions ordered newest first.
func (s *Store) ListDecisionRecords(ctx context.Context, query DecisionListQuery) ([]DecisionRecord, error) {
	if ctx == nil {
		return nil, errors.New("memory: list decisions context is required")
	}
	if err := s.ensureDecisionCatalog(ctx); err != nil {
		return nil, err
	}
	stored, err := s.catalog.listDecisions(ctx, query)
	if err != nil {
		return nil, err
	}
	records := make([]DecisionRecord, 0, len(stored))
	for _, decision := range stored {
		records = append(records, decision.record())
	}
	return records, nil
}

func (c *catalog) listDecisions(ctx context.Context, query DecisionListQuery) ([]storedDecision, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return nil, err
	}
	if db == nil {
		return nil, errors.New("memory: decision catalog is disabled")
	}
	sqlText := strings.Join([]string{
		`SELECT id, candidate_hash, idempotency_key, workspace_id, scope, agent_name,`,
		`agent_tier, op, targets, target_filename, frontmatter, post_content,`,
		`post_content_hash, prior_content, confidence, source, rule_trace, llm_trace,`,
		`reason, prompt_version, applied_at, decided_at`,
		`FROM memory_decisions`,
	}, "\n")
	clauses, args, err := decisionListWhere(query)
	if err != nil {
		return nil, err
	}
	if len(clauses) > 0 {
		sqlText += "\nWHERE " + strings.Join(clauses, " AND ")
	}
	sqlText += "\nORDER BY decided_at DESC, id DESC\nLIMIT ?"
	args = append(args, clampMemoryQueryLimit(query.Limit))
	rows, err := db.QueryContext(ctx, sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("memory: list decisions: %w", err)
	}
	defer closeRows(rows, "memory: close decision rows failed")
	return scanStoredDecisionRows(rows)
}

func decisionListWhere(query DecisionListQuery) ([]string, []any, error) {
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)
	if scope := query.Scope.Normalize(); scope != "" {
		if err := scope.Validate(); err != nil {
			return nil, nil, wrapValidationError("list decisions scope", string(query.Scope), err)
		}
		clauses = append(clauses, "scope = ?")
		args = append(args, string(scope))
	}
	if workspaceID := strings.TrimSpace(query.WorkspaceID); workspaceID != "" {
		clauses = append(clauses, "workspace_id = ?")
		args = append(args, workspaceID)
	}
	if agentName := strings.TrimSpace(query.AgentName); agentName != "" {
		clauses = append(clauses, "agent_name = ?")
		args = append(args, agentName)
	}
	if agentTier := query.AgentTier.Normalize(); agentTier != "" {
		if err := agentTier.Validate(); err != nil {
			return nil, nil, wrapValidationError("list decisions agent tier", string(query.AgentTier), err)
		}
		clauses = append(clauses, "agent_tier = ?")
		args = append(args, string(agentTier))
	}
	if op := strings.TrimSpace(query.Operation); op != "" {
		clauses = append(clauses, "op = ?")
		args = append(args, op)
	}
	if targetFilename := strings.TrimSpace(query.TargetFilename); targetFilename != "" {
		cleaned, err := cleanFilename(targetFilename)
		if err != nil {
			return nil, nil, wrapValidationError("list decisions target filename", targetFilename, err)
		}
		clauses = append(clauses, "target_filename = ?")
		args = append(args, cleaned)
	}
	if !query.Since.IsZero() {
		clauses = append(clauses, "decided_at >= ?")
		args = append(args, timeToUnixMillis(query.Since.UTC()))
	}
	if reason := strings.TrimSpace(query.Reason); reason != "" {
		clauses = append(clauses, "reason = ?")
		args = append(args, reason)
	}
	return clauses, args, nil
}

func scanStoredDecisionRows(rows *sql.Rows) ([]storedDecision, error) {
	decisions := make([]storedDecision, 0)
	for rows.Next() {
		decision, scanErr := scanStoredDecision(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		decisions = append(decisions, decision)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory: iterate decisions: %w", err)
	}
	return decisions, nil
}
