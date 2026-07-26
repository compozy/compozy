package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/core"
	memorypkg "github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/goccy/go-yaml"
)

type memoryProposeInput struct {
	Operation      string                      `json:"operation,omitempty"`
	Operations     []memoryBatchOperationInput `json:"operations,omitempty"`
	Filename       string                      `json:"filename,omitempty"`
	TargetFilename string                      `json:"target_filename,omitempty"`
	Content        string                      `json:"content,omitempty"`
	Name           string                      `json:"name,omitempty"`
	Description    string                      `json:"description,omitempty"`
	Type           string                      `json:"type,omitempty"`
	Scope          string                      `json:"scope,omitempty"`
	Workspace      string                      `json:"workspace,omitempty"`
	AgentName      string                      `json:"agent_name,omitempty"`
	AgentTier      string                      `json:"agent_tier,omitempty"`
	Entity         string                      `json:"entity,omitempty"`
	Attribute      string                      `json:"attribute,omitempty"`
}

type memoryBatchOperationInput struct {
	Action  string `json:"action"`
	Content string `json:"content,omitempty"`
	OldText string `json:"old_text,omitempty"`
}

type nativeMemoryWriteDocument struct {
	Filename    string
	Scope       memcontract.Scope
	AgentName   string
	AgentTier   memcontract.AgentTier
	Name        string
	Description string
	Type        string
	Content     string
}

func (n *daemonNativeTools) memoryPropose(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input memoryProposeInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	if len(input.Operations) > 0 {
		return n.memoryProposeBatch(ctx, scope, req, input)
	}
	return n.memoryProposeSingle(ctx, scope, req, input)
}

func (n *daemonNativeTools) memoryProposeSingle(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input memoryProposeInput,
) (toolspkg.ToolResult, error) {
	op, err := nativeMemoryProposalOperation(req.ToolID, input.Operation)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	location, actorKind, err := n.memoryProposalLocation(ctx, scope, req, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}

	if op == memcontract.OpDelete {
		filename, err := requiredNativeString(
			req.ToolID,
			"target_filename",
			firstNonEmpty(input.TargetFilename, input.Filename),
		)
		if err != nil {
			return toolspkg.ToolResult{}, err
		}
		result, err := location.Store.ProposeDelete(ctx, location.Scope, filename, memcontract.OriginTool)
		if err != nil {
			return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
		}
		n.recordMemoryToolWrite(scope, req, actorKind)
		return nativeMemoryDecisionResult(result)
	}

	content, err := requiredNativeString(req.ToolID, "content", input.Content)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	filename := firstNonEmpty(input.Filename, input.TargetFilename)
	if strings.TrimSpace(filename) == "" {
		filename = nativeMemoryFilename(input.Type, firstNonEmpty(input.Name, input.Entity, content))
	}
	document, err := renderNativeMemoryDocument(nativeMemoryWriteDocument{
		Filename:    filename,
		Scope:       location.Scope,
		AgentName:   location.AgentName,
		AgentTier:   location.AgentTier,
		Name:        input.Name,
		Description: input.Description,
		Type:        input.Type,
		Content:     content,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	result, err := location.Store.ProposeWrite(ctx, location.Scope, filename, document, memcontract.OriginTool)
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	n.recordMemoryToolWrite(scope, req, actorKind)
	return nativeMemoryDecisionResult(result)
}

func (n *daemonNativeTools) memoryProposeBatch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input memoryProposeInput,
) (toolspkg.ToolResult, error) {
	if strings.TrimSpace(input.Operation) != "" || strings.TrimSpace(input.Content) != "" {
		return toolspkg.ToolResult{}, nativeMemoryBatchShapeError(req.ToolID)
	}
	location, actorKind, err := n.memoryProposalLocation(ctx, scope, req, input)
	if err != nil {
		return toolspkg.ToolResult{}, err
	}
	seed := firstMemoryBatchContent(input.Operations)
	filename := firstNonEmpty(input.Filename, input.TargetFilename)
	if strings.TrimSpace(filename) == "" {
		if strings.TrimSpace(seed) == "" {
			return toolspkg.ToolResult{}, nativeRequiredInputError(req.ToolID, "filename")
		}
		filename = nativeMemoryFilename(input.Type, firstNonEmpty(input.Name, input.Entity, seed))
	}
	header, err := nativeMemoryWriteHeader(nativeMemoryWriteDocument{
		Filename:    filename,
		Scope:       location.Scope,
		AgentName:   location.AgentName,
		AgentTier:   location.AgentTier,
		Name:        input.Name,
		Description: input.Description,
		Type:        input.Type,
		Content:     seed,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	operations := make([]memorypkg.BatchOperation, 0, len(input.Operations))
	for _, operation := range input.Operations {
		operations = append(operations, memorypkg.BatchOperation{
			Action:  memorypkg.BatchAction(operation.Action),
			Content: operation.Content,
			OldText: operation.OldText,
		})
	}
	result, err := location.Store.ProposeBatch(ctx, memorypkg.BatchProposal{
		Scope:      location.Scope,
		Filename:   filename,
		Header:     header,
		Operations: operations,
		Origin:     memcontract.OriginTool,
	})
	if err != nil {
		return toolspkg.ToolResult{}, nativeMemoryToolError(req.ToolID, err)
	}
	n.recordMemoryToolWrite(scope, req, actorKind)
	return nativeMemoryBatchResult(result)
}

func (n *daemonNativeTools) memoryProposalLocation(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
	input memoryProposeInput,
) (memoryToolLocation, nativeMemoryActorKind, error) {
	location, err := n.memoryWriteStore(ctx, scope, req.ToolID, memoryToolSelector{
		Scope:     input.Scope,
		Workspace: input.Workspace,
		AgentName: input.AgentName,
		AgentTier: input.AgentTier,
	}, input.Type)
	if err != nil {
		return memoryToolLocation{}, "", nativeMemoryToolError(req.ToolID, err)
	}
	actorKind, err := n.memoryCallerActorKind(ctx, scope, req)
	if err != nil {
		return memoryToolLocation{}, "", nativeMemoryToolError(req.ToolID, err)
	}
	if err := n.denySubagentMemoryWrite(
		ctx,
		req,
		location,
		actorKind,
		firstNonEmpty(input.TargetFilename, input.Filename),
	); err != nil {
		return memoryToolLocation{}, "", err
	}
	return location, actorKind, nil
}

func nativeMemoryProposalOperation(id toolspkg.ToolID, raw string) (memcontract.Op, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", memcontract.OpAdd.String(), memcontract.OpUpdate.String():
		return memcontract.OpAdd, nil
	case memcontract.OpDelete.String():
		return memcontract.OpDelete, nil
	default:
		return 0, toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			"operation must be add, update, or delete",
			toolspkg.ErrToolInvalidInput,
			toolspkg.ReasonSchemaInvalid,
		)
	}
}

func nativeMemoryBatchShapeError(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"operations cannot be combined with operation or content",
		toolspkg.ErrToolInvalidInput,
		toolspkg.ReasonSchemaInvalid,
	)
}

func firstMemoryBatchContent(operations []memoryBatchOperationInput) string {
	for _, operation := range operations {
		if strings.EqualFold(strings.TrimSpace(operation.Action), string(memorypkg.BatchActionAdd)) {
			if content := strings.TrimSpace(operation.Content); content != "" {
				return content
			}
		}
	}
	return ""
}

func renderNativeMemoryDocument(doc nativeMemoryWriteDocument) ([]byte, error) {
	header, err := nativeMemoryWriteHeader(doc)
	if err != nil {
		return nil, err
	}
	metadata, err := yaml.Marshal(header)
	if err != nil {
		return nil, fmt.Errorf("daemon: marshal memory frontmatter: %w", err)
	}
	var builder strings.Builder
	builder.WriteString("---\n")
	builder.Write(metadata)
	builder.WriteString("---\n\n")
	builder.WriteString(strings.TrimSpace(doc.Content))
	return []byte(builder.String()), nil
}

func nativeMemoryWriteHeader(doc nativeMemoryWriteDocument) (memcontract.Header, error) {
	header := memcontract.Header{
		Name:        firstNonEmpty(doc.Name, nativeMemoryNameFromFilename(doc.Filename)),
		Description: firstNonEmpty(doc.Description, nativeMemoryDescription(doc.Content)),
		Type:        nativeMemoryTypeForScope(doc.Type, doc.Scope),
		Scope:       doc.Scope.Normalize(),
	}
	if header.Scope == memcontract.ScopeAgent {
		header.AgentName = strings.TrimSpace(doc.AgentName)
		header.AgentTier = doc.AgentTier.Normalize()
	}
	if err := header.Validate(); err != nil {
		return memcontract.Header{}, core.NewMemoryValidationError(err)
	}
	return header, nil
}

func nativeMemoryTypeForScope(raw string, scope memcontract.Scope) memcontract.Type {
	memoryType := memcontract.Type(strings.TrimSpace(raw)).Normalize()
	if memoryType != "" {
		return memoryType
	}
	if scope.Normalize() == memcontract.ScopeWorkspace {
		return memcontract.TypeProject
	}
	return memcontract.TypeUser
}

func nativeMemoryDecisionResult(result memorypkg.DecisionApplyResult) (toolspkg.ToolResult, error) {
	decision := redactNativeMemoryDecision(result.Decision)
	return structuredResult(map[string]any{
		"decision":         decision,
		nativeAppliedValue: result.Applied,
	}, fmt.Sprintf("memory decision %s", decision.Op.String()))
}

func nativeMemoryBatchResult(result memorypkg.BatchApplyResult) (toolspkg.ToolResult, error) {
	decision := redactNativeMemoryDecision(result.Decision)
	return structuredResult(map[string]any{
		"decision":         decision,
		nativeAppliedValue: result.Applied,
		"operations":       result.Operations,
	}, fmt.Sprintf("memory batch decision %s", decision.Op.String()))
}

func redactNativeMemoryDecision(decision memcontract.Decision) memcontract.Decision {
	redacted := decision
	redacted.Frontmatter.Name = taskpkg.RedactClaimTokens(strings.TrimSpace(redacted.Frontmatter.Name))
	redacted.Frontmatter.Description = taskpkg.RedactClaimTokens(strings.TrimSpace(redacted.Frontmatter.Description))
	redacted.PostContent = taskpkg.RedactClaimTokens(strings.TrimSpace(redacted.PostContent))
	redacted.PriorContent = taskpkg.RedactClaimTokens(strings.TrimSpace(redacted.PriorContent))
	redacted.Reason = taskpkg.RedactClaimTokens(strings.TrimSpace(redacted.Reason))
	if redacted.LLMTrace != nil {
		trace := *redacted.LLMTrace
		trace.RawResponse = taskpkg.RedactClaimTokens(strings.TrimSpace(trace.RawResponse))
		trace.Error = taskpkg.RedactClaimTokens(strings.TrimSpace(trace.Error))
		redacted.LLMTrace = &trace
	}
	return redacted
}
