package memory

import (
	"context"

	"encoding/json"

	"fmt"

	"path/filepath"
	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	storepkg "github.com/compozy/agh/internal/store"
)

func (c *catalog) logDecisionEvent(
	ctx context.Context,
	decision memcontract.Decision,
	workspaceID string,
	applied bool,
) error {
	eventOp := memoryEventWriteCommitted
	switch decision.Op {
	case memcontract.OpReject:
		eventOp = memoryEventWriteRejected
	case memcontract.OpNoop:
		eventOp = memoryEventWriteShadowed
	}
	return c.insertDecisionEvent(ctx, eventOp, decision, workspaceID, map[string]string{
		decisionMetadataOperationKey:      decision.Op.String(),
		decisionMetadataTargetFilenameKey: decision.TargetFilename,
		decisionMetadataReasonKey:         decision.Reason,
		"applied":                         fmt.Sprintf("%t", applied),
		decisionMetadataRuleIDsKey:        decisionRuleIDs(decision),
	})
}

func (c *catalog) logRevertEvent(ctx context.Context, decision storedDecision) error {
	return c.insertDecisionEvent(
		ctx,
		memoryEventWriteReverted,
		decision.Decision,
		decision.WorkspaceID,
		map[string]string{
			decisionMetadataOperationKey:      "revert",
			decisionMetadataTargetFilenameKey: decision.TargetFilename,
			decisionMetadataReasonKey:         "decision reverted",
		},
	)
}

func (c *catalog) insertDecisionEvent(
	ctx context.Context,
	eventOp string,
	decision memcontract.Decision,
	workspaceID string,
	metadata map[string]string,
) error {
	payload, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("memory: encode decision event metadata: %w", err)
	}
	return c.withCatalogWriteTx(ctx, "decision event insert", func(tx *storepkg.WriteTx) error {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO memory_events (
				op, scope, agent_name, agent_tier, workspace_id, session_id,
				actor_kind, decision_id, target_id, metadata, ts_ms
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			eventOp,
			nullStringForEmpty(decision.Frontmatter.Scope.Normalize()),
			nullStringForEmpty(decision.Frontmatter.AgentName),
			nullStringForEmpty(string(decision.Frontmatter.AgentTier.Normalize())),
			nullStringForEmpty(workspaceID),
			nil,
			"system",
			decision.ID,
			nullStringForEmpty(decision.TargetFilename),
			string(payload),
			timeToUnixMillis(time.Now().UTC()),
		); err != nil {
			return fmt.Errorf("memory: insert decision event: %w", err)
		}
		return nil
	})
}

func nullableLLMTrace(decision memcontract.Decision) (any, error) {
	if decision.LLMTrace == nil {
		return nil, nil
	}
	payload, err := json.Marshal(decision.LLMTrace)
	if err != nil {
		return nil, fmt.Errorf("memory: encode decision llm_trace: %w", err)
	}
	return string(payload), nil
}

func nullStringForEmptyRaw(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func decisionRuleIDs(decision memcontract.Decision) string {
	ids := make([]string, 0, len(decision.RuleTrace))
	for _, hit := range decision.RuleTrace {
		if strings.TrimSpace(hit.Name) != "" {
			ids = append(ids, strings.TrimSpace(hit.Name))
		}
	}
	return strings.Join(ids, ",")
}

func entityFromFilename(filename string, header memcontract.Header) string {
	base := strings.TrimSuffix(strings.TrimSpace(filename), filepath.Ext(filename))
	prefix := string(header.Type.Normalize()) + "_"
	base = strings.TrimPrefix(base, prefix)
	base = strings.ReplaceAll(base, "_", " ")
	if strings.TrimSpace(base) == "" {
		return strings.ToLower(strings.TrimSpace(header.Name))
	}
	return strings.ToLower(strings.TrimSpace(base))
}

func attributeFromHeader(header memcontract.Header) string {
	return string(header.Type.Normalize())
}
