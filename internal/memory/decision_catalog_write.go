package memory

import (
	"context"

	"encoding/json"

	"fmt"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/controller"
	storepkg "github.com/compozy/agh/internal/store"
)

func (c *catalog) insertDecision(ctx context.Context, decision memcontract.Decision, workspaceID string) error {
	return c.withCatalogWriteTx(ctx, "decision wal insert", func(tx *storepkg.WriteTx) error {
		targets, err := json.Marshal(decision.Targets)
		if err != nil {
			return fmt.Errorf("memory: encode decision targets: %w", err)
		}
		frontmatter, err := json.Marshal(decision.Frontmatter)
		if err != nil {
			return fmt.Errorf("memory: encode decision frontmatter: %w", err)
		}
		ruleTrace, err := json.Marshal(decision.RuleTrace)
		if err != nil {
			return fmt.Errorf("memory: encode decision rule_trace: %w", err)
		}
		llmTrace, err := nullableLLMTrace(decision)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_decisions (
				id, candidate_hash, idempotency_key, frontmatter_hash, workspace_id,
				scope, agent_name, agent_tier, op, targets, target_filename, frontmatter,
				post_content, post_content_hash, prior_content, confidence, source,
				rule_trace, llm_trace, reason, prompt_version, decided_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			decision.ID,
			decision.CandidateHash,
			decision.IdempotencyKey,
			controller.FrontmatterHash(decision.Frontmatter),
			nullStringForEmpty(workspaceID),
			string(decision.Frontmatter.Scope.Normalize()),
			nullStringForEmpty(decision.Frontmatter.AgentName),
			nullStringForEmpty(string(decision.Frontmatter.AgentTier.Normalize())),
			decision.Op.String(),
			string(targets),
			decision.TargetFilename,
			string(frontmatter),
			nullStringForEmptyRaw(decision.PostContent),
			nullStringForEmpty(decision.PostContentHash),
			nullStringForEmptyRaw(decision.PriorContent),
			decision.Confidence,
			string(decision.Source.Normalize()),
			string(ruleTrace),
			llmTrace,
			nullStringForEmpty(decision.Reason),
			decision.PromptVersion,
			timeToUnixMillis(decision.DecidedAt),
		); err != nil {
			return fmt.Errorf("memory: insert decision %q: %w", decision.ID, err)
		}
		return nil
	})
}

func (c *catalog) markDecisionApplied(ctx context.Context, id string) error {
	return c.withCatalogWriteTx(ctx, "decision wal mark applied", func(tx *storepkg.WriteTx) error {
		result, err := tx.ExecContext(
			ctx,
			`UPDATE memory_decisions SET applied_at = ? WHERE id = ? AND applied_at IS NULL`,
			timeToUnixMillis(time.Now().UTC()),
			strings.TrimSpace(id),
		)
		if err != nil {
			return fmt.Errorf("memory: mark decision %q applied: %w", id, err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("memory: inspect decision %q applied update: %w", id, err)
		}
		if affected == 0 {
			return fmt.Errorf("memory: decision %q was already applied", id)
		}
		return nil
	})
}
