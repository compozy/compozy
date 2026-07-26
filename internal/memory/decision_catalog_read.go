package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"
)

func (c *catalog) loadDecision(ctx context.Context, id string) (storedDecision, error) {
	db, err := c.ensureDB(ctx)
	if err != nil {
		return storedDecision{}, err
	}
	if db == nil {
		return storedDecision{}, errors.New("memory: decision catalog is disabled")
	}
	row := db.QueryRowContext(
		ctx,
		`SELECT id, candidate_hash, idempotency_key, workspace_id, scope, agent_name,
			agent_tier, op, targets, target_filename, frontmatter, post_content,
			post_content_hash, prior_content, confidence, source, rule_trace, llm_trace,
			reason, prompt_version, applied_at, decided_at
		 FROM memory_decisions
		 WHERE id = ?`,
		strings.TrimSpace(id),
	)
	decision, err := scanStoredDecision(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedDecision{}, fmt.Errorf("memory: decision %q: %w", strings.TrimSpace(id), os.ErrNotExist)
		}
		return storedDecision{}, err
	}
	return decision, nil
}

func (c *catalog) loadDecisionByIdempotencyKey(
	ctx context.Context,
	idempotencyKey string,
) (storedDecision, bool, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return storedDecision{}, false, nil
	}
	db, err := c.ensureDB(ctx)
	if err != nil {
		return storedDecision{}, false, err
	}
	if db == nil {
		return storedDecision{}, false, errors.New("memory: decision catalog is disabled")
	}
	row := db.QueryRowContext(
		ctx,
		`SELECT id, candidate_hash, idempotency_key, workspace_id, scope, agent_name,
			agent_tier, op, targets, target_filename, frontmatter, post_content,
			post_content_hash, prior_content, confidence, source, rule_trace, llm_trace,
			reason, prompt_version, applied_at, decided_at
		 FROM memory_decisions
		 WHERE idempotency_key = ?`,
		key,
	)
	decision, err := scanStoredDecision(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedDecision{}, false, nil
		}
		return storedDecision{}, false, err
	}
	return decision, true, nil
}

func scanStoredDecision(scanner interface{ Scan(dest ...any) error }) (storedDecision, error) {
	var (
		decision        storedDecision
		workspaceID     sql.NullString
		scopeRaw        string
		agentName       sql.NullString
		agentTierRaw    sql.NullString
		opRaw           string
		targetsRaw      string
		frontmatterRaw  string
		postContent     sql.NullString
		postContentHash sql.NullString
		priorContent    sql.NullString
		sourceRaw       string
		ruleTraceRaw    string
		llmTraceRaw     sql.NullString
		reason          sql.NullString
		appliedAt       sql.NullInt64
		decidedAt       int64
	)
	if err := scanner.Scan(
		&decision.ID,
		&decision.CandidateHash,
		&decision.IdempotencyKey,
		&workspaceID,
		&scopeRaw,
		&agentName,
		&agentTierRaw,
		&opRaw,
		&targetsRaw,
		&decision.TargetFilename,
		&frontmatterRaw,
		&postContent,
		&postContentHash,
		&priorContent,
		&decision.Confidence,
		&sourceRaw,
		&ruleTraceRaw,
		&llmTraceRaw,
		&reason,
		&decision.PromptVersion,
		&appliedAt,
		&decidedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return storedDecision{}, err
		}
		return storedDecision{}, fmt.Errorf("memory: scan decision: %w", err)
	}
	if err := decodeStoredDecisionJSON(&decision, targetsRaw, frontmatterRaw, ruleTraceRaw, llmTraceRaw); err != nil {
		return storedDecision{}, err
	}
	op, err := replayOp(opRaw)
	if err != nil {
		return storedDecision{}, fmt.Errorf("memory: decode decision op: %w", err)
	}
	decision.Op = op
	decision.Source = memcontract.DecisionSource(sourceRaw).Normalize()
	decision.WorkspaceID = nullableSQLString(workspaceID)
	decision.AgentName = nullableSQLString(agentName)
	decision.AgentTier = memcontract.AgentTier(nullableSQLString(agentTierRaw)).Normalize()
	decision.PostContent = nullableSQLStringRaw(postContent)
	decision.PostContentHash = nullableSQLString(postContentHash)
	decision.PriorContent = nullableSQLStringRaw(priorContent)
	decision.Reason = nullableSQLString(reason)
	decision.DecidedAt = timeFromUnixMillis(decidedAt)
	if appliedAt.Valid {
		parsed := timeFromUnixMillis(appliedAt.Int64)
		decision.AppliedAt = &parsed
	}
	return decision, nil
}

func decodeStoredDecisionJSON(
	decision *storedDecision,
	targetsRaw string,
	frontmatterRaw string,
	ruleTraceRaw string,
	llmTraceRaw sql.NullString,
) error {
	if err := json.Unmarshal([]byte(targetsRaw), &decision.Targets); err != nil {
		return fmt.Errorf("memory: decode decision targets: %w", err)
	}
	if err := json.Unmarshal([]byte(frontmatterRaw), &decision.Frontmatter); err != nil {
		return fmt.Errorf("memory: decode decision frontmatter: %w", err)
	}
	if err := json.Unmarshal([]byte(ruleTraceRaw), &decision.RuleTrace); err != nil {
		return fmt.Errorf("memory: decode decision rule_trace: %w", err)
	}
	if llmTraceRaw.Valid && strings.TrimSpace(llmTraceRaw.String) != "" {
		var trace memcontract.LLMCall
		if err := json.Unmarshal([]byte(llmTraceRaw.String), &trace); err != nil {
			return fmt.Errorf("memory: decode decision llm_trace: %w", err)
		}
		decision.LLMTrace = &trace
	}
	return nil
}
