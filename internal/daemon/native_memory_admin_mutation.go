package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	toolspkg "github.com/compozy/agh/internal/tools"
)

func (n *daemonNativeTools) memoryAdminPromote(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminPromoteInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	sourceSelector := memoryAdminSelectorFromPayload(input.From)
	sourceLocation, err := n.resolveMemoryLocation(ctx, scope, req.ToolID, input.Filename, sourceSelector)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	raw, err := sourceLocation.Store.Read(sourceLocation.Scope, sourceLocation.Filename)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	header, body, err := nativeMemoryAdminHeaderAndBody(raw)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	targetLocation, err := n.memoryAdminLocation(
		ctx,
		scope,
		req.ToolID,
		memoryAdminSelectorInputFromPayload(input.To),
		input.To.Scope.Normalize(),
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	header.Scope = targetLocation.Scope
	header.AgentName = targetLocation.AgentName
	header.AgentTier = targetLocation.AgentTier
	candidate := memcontract.Candidate{
		WorkspaceID: targetLocation.WorkspaceID,
		Scope:       targetLocation.Scope,
		AgentName:   targetLocation.AgentName,
		AgentTier:   targetLocation.AgentTier,
		Origin:      memcontract.OriginTool,
		Content:     strings.TrimSpace(body),
		Frontmatter: header,
		Metadata: map[string]string{
			nativeMemoryMetadataIDKey:             strings.TrimSpace(input.IdempotencyKey),
			nativeMemoryMetadataTargetFilenameKey: sourceLocation.Filename,
		},
		SubmittedAt: time.Now().UTC(),
	}
	var decision memcontract.Decision
	if input.DryRun {
		decision, err = targetLocation.Store.DecideCandidate(ctx, candidate)
	} else {
		decision, err = targetLocation.Store.ProposeCandidate(ctx, candidate)
	}
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	decision = redactNativeMemoryDecision(decision)
	applied := false
	if !input.DryRun {
		applied = nativeMemoryDecisionApplied(decision)
	}
	payload := contract.MemoryPromoteResponse{
		Decision: core.MemoryDecisionPayload(decision, nil),
		Applied:  applied,
		DryRun:   input.DryRun,
	}
	return structuredResult(payload, fmt.Sprintf("memory decision %s", decision.Op.String()))
}

func nativeMemoryDecisionApplied(decision memcontract.Decision) bool {
	switch decision.Op {
	case memcontract.OpAdd, memcontract.OpUpdate, memcontract.OpDelete:
		return true
	default:
		return false
	}
}

func (n *daemonNativeTools) memoryAdminReset(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminResetInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if !input.Confirm {
		payload := contract.MemoryResetResponse{ResetAt: time.Now().UTC(), DerivedOnly: input.DerivedOnly}
		return structuredResult(payload, "memory reset not confirmed")
	}
	if !input.DerivedOnly {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(
			req.ToolID,
			core.NewMemoryValidationError(errors.New("only derived memory reset is supported in Slice 1")),
		)
	}
	location, err := n.memoryAdminLocation(
		ctx,
		scope,
		req.ToolID,
		input.memoryAdminSelectorInput,
		memcontract.ScopeGlobal,
	)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	result, err := location.Store.ResetDerived(ctx, memcontract.ReindexOptions{
		Scope:     location.Scope,
		Workspace: location.Workspace,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(req.ToolID, err)
	}
	payload := contract.MemoryResetResponse{
		ResetAt:     result.ResetAt.UTC(),
		DerivedOnly: true,
		DeletedRows: result.DeletedRows,
	}
	return structuredResult(payload, fmt.Sprintf("deleted %d memory derived rows", payload.DeletedRows))
}

func (n *daemonNativeTools) memoryAdminReload(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminSelectorInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	reloadedAt := time.Now().UTC()
	payload := contract.MemoryReloadResponse{
		ReloadedAt: reloadedAt,
		Generation: reloadedAt.UnixNano(),
	}
	return structuredResult(payload, "memory snapshot reloaded")
}

func (n *daemonNativeTools) memoryAdminRecallTrace(
	_ context.Context,
	_ toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryAdminRecallTraceInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if _, err := requiredNativeString(req.ToolID, daemonPayloadSessionIDKey, input.SessionID); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if input.TurnSeq <= 0 {
		return toolspkg.ToolResult{}, nativeMemoryAdminToolError(
			req.ToolID,
			core.NewMemoryValidationError(errors.New("turn_seq must be a positive integer")),
		)
	}
	return toolspkg.ToolResult{}, nativeMemoryAdminToolError(
		req.ToolID,
		fmt.Errorf("%w: recall trace %s/%d is not materialized", os.ErrNotExist, input.SessionID, input.TurnSeq),
	)
}
