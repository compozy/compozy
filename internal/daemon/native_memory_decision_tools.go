package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	toolspkg "github.com/compozy/agh/internal/tools"
)

type memoryAdminDecisionListInput struct {
	memoryAdminSelectorInput
	Operation      string `json:"operation,omitempty"`
	TargetFilename string `json:"filename,omitempty"`
	Since          string `json:"since,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Limit          int    `json:"limit,omitempty"`
}

type memoryAdminDecisionIDInput struct {
	DecisionID string `json:"decision_id"`
}

type memoryAdminDecisionRevertInput struct {
	DecisionID string `json:"decision_id"`
	Reason     string `json:"reason,omitempty"`
	DryRun     bool   `json:"dry_run,omitempty"`
}

func (n *daemonNativeTools) memoryAdminDecisionsList(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDecisionListInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	since, err := parseNativeOptionalRFC3339(req.ToolID, "since", input.Since)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	if err := nativeMemoryAdminLimit(req.ToolID, input.Limit); err != nil {
		return toolspkg.ToolResult{}, err
	}
	selector, err := n.memoryAdminResolvedSelector(ctx, scope, input.memoryAdminSelectorInput)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	records, err := n.deps.MemoryStore.ListDecisionRecords(ctx, memorypkg.DecisionListQuery{
		Scope:          selector.Scope,
		WorkspaceID:    selector.WorkspaceID,
		AgentName:      selector.AgentName,
		AgentTier:      selector.AgentTier,
		Operation:      strings.TrimSpace(input.Operation),
		TargetFilename: strings.TrimSpace(input.TargetFilename),
		Since:          since,
		Reason:         strings.TrimSpace(input.Reason),
		Limit:          input.Limit,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payloads := make([]contract.MemoryDecisionPayload, 0, len(records))
	for _, record := range records {
		payloads = append(payloads, core.MemoryDecisionRecordPayload(record))
	}
	return structuredResult(
		contract.MemoryDecisionListResponse{Decisions: payloads},
		fmt.Sprintf("%d memory decisions", len(payloads)),
	)
}

func (n *daemonNativeTools) memoryAdminDecisionsShow(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDecisionIDInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	decisionID, err := requiredNativeString(req.ToolID, "decision_id", input.DecisionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := n.deps.MemoryStore.LoadDecisionRecord(ctx, decisionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	if err := nativeMemoryAdminAuthorizeDecisionRecord(scope, req.ToolID, record); err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payload := contract.MemoryDecisionResponse{Decision: core.MemoryDecisionRecordPayload(record)}
	return structuredResult(payload, decisionID)
}

func (n *daemonNativeTools) memoryAdminDecisionsRevert(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminDecisionRevertInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	decisionID, err := requiredNativeString(req.ToolID, "decision_id", input.DecisionID)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	record, err := n.deps.MemoryStore.LoadDecisionRecord(ctx, decisionID)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	if err := nativeMemoryAdminAuthorizeDecisionRecord(scope, req.ToolID, record); err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	reverted := false
	if !input.DryRun {
		store, err := n.memoryAdminStoreForDecisionRecord(ctx, scope, req.ToolID, record)
		if err != nil {
			return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
		}
		result, err := store.RevertDecision(ctx, decisionID)
		if err != nil {
			return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
		}
		reverted = result.Reverted
	}
	payload := contract.MemoryDecisionRevertResponse{
		Decision: core.MemoryDecisionRecordPayload(record),
		Reverted: reverted,
		DryRun:   input.DryRun,
	}
	return structuredResult(payload, fmt.Sprintf("memory decision %s reverted=%t", decisionID, reverted))
}

func (n *daemonNativeTools) memoryAdminStoreForDecisionRecord(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
	record memorypkg.DecisionRecord,
) (*memorypkg.Store, error) {
	decisionScope := record.Decision.Frontmatter.Scope.Normalize()
	if decisionScope == memcontract.ScopeWorkspace ||
		(decisionScope == memcontract.ScopeAgent && record.AgentTier.Normalize() == memcontract.AgentTierWorkspace) {
		location, err := n.memoryAdminLocation(ctx, scope, id, memoryAdminSelectorInput{
			Scope:       string(decisionScope),
			WorkspaceID: record.WorkspaceID,
			AgentName:   record.AgentName,
			AgentTier:   string(record.AgentTier),
		}, decisionScope)
		if err != nil {
			return nil, err
		}
		return location.Store, nil
	}
	return n.deps.MemoryStore, nil
}

func nativeMemoryAdminAuthorizeDecisionRecord(
	callerScope toolspkg.Scope,
	id toolspkg.ToolID,
	record memorypkg.DecisionRecord,
) error {
	if callerScope.Operator {
		return nil
	}
	decisionScope := record.Decision.Frontmatter.Scope.Normalize()
	recordWorkspaceID := strings.TrimSpace(record.WorkspaceID)
	recordAgentName := strings.TrimSpace(firstNonEmpty(record.AgentName, record.Decision.Frontmatter.AgentName))
	recordAgentTier := record.AgentTier.Normalize()
	if recordAgentTier == "" {
		recordAgentTier = record.Decision.Frontmatter.AgentTier.Normalize()
	}
	switch decisionScope {
	case memcontract.ScopeGlobal:
		return nil
	case memcontract.ScopeWorkspace:
		if recordWorkspaceID != "" && recordWorkspaceID == strings.TrimSpace(callerScope.WorkspaceID) {
			return nil
		}
	case memcontract.ScopeAgent:
		if recordAgentName == "" || recordAgentName != strings.TrimSpace(callerScope.AgentName) {
			break
		}
		if recordAgentTier == memcontract.AgentTierWorkspace {
			if recordWorkspaceID != "" && recordWorkspaceID == strings.TrimSpace(callerScope.WorkspaceID) {
				return nil
			}
			break
		}
		return nil
	default:
		return core.NewMemoryValidationError(fmt.Errorf("unsupported decision scope %q", decisionScope))
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		"memory decision is outside caller scope",
		fmt.Errorf("%w: memory decision %q is outside caller scope", toolspkg.ErrToolDenied, record.Decision.ID),
		toolspkg.ReasonPolicyDenied,
	)
}
