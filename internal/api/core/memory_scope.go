package core

import (
	"context"
	"errors"
	"fmt"

	"path/filepath"
	"strconv"
	"strings"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/memory"

	"github.com/gin-gonic/gin"
)

func firstNonEmptyScope(values ...memcontract.Scope) memcontract.Scope {
	for _, value := range values {
		if normalized := value.Normalize(); normalized != "" {
			return normalized
		}
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneMemoryStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+4)
	for key, value := range in {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return out
}

func defaultMemorySelectorScope(selector memorySelector) memcontract.Scope {
	if scope := selector.Scope.Normalize(); scope != "" {
		return scope
	}
	switch {
	case strings.TrimSpace(selector.AgentName) != "":
		return memcontract.ScopeAgent
	case strings.TrimSpace(firstNonEmptyString(selector.WorkspaceID, selector.Workspace)) != "":
		return memcontract.ScopeWorkspace
	default:
		return memcontract.ScopeGlobal
	}
}

func memoryDecisionApplied(decision memcontract.Decision) bool {
	switch decision.Op {
	case memcontract.OpAdd, memcontract.OpUpdate, memcontract.OpDelete:
		return true
	default:
		return false
	}
}

func (h *BaseHandlers) memoryHealthWorkspaces(ctx context.Context, rawWorkspace string) ([]string, error) {
	if strings.TrimSpace(rawWorkspace) != "" {
		workspace, _, err := h.resolveMemoryWorkspaceRef(ctx, rawWorkspace)
		if err != nil {
			return nil, err
		}
		return []string{workspace}, nil
	}

	infos, err := h.Sessions.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	workspaces := make([]string, 0, len(infos))
	seen := make(map[string]struct{}, len(infos))
	for _, info := range infos {
		if info == nil || strings.TrimSpace(info.Workspace) == "" {
			continue
		}
		workspace, err := resolveMemoryWorkspace(info.Workspace)
		if err != nil {
			return nil, err
		}
		if _, exists := seen[workspace]; exists {
			continue
		}
		seen[workspace] = struct{}{}
		workspaces = append(workspaces, workspace)
	}

	return workspaces, nil
}

// MemoryHealthWorkspaces returns the workspaces considered for memory health checks.
func (h *BaseHandlers) MemoryHealthWorkspaces(ctx context.Context, rawWorkspace string) ([]string, error) {
	return h.memoryHealthWorkspaces(ctx, rawWorkspace)
}

// ResolveMemoryWriteScope validates a write request and infers its target scope.
func ResolveMemoryWriteScope(req contract.MemoryWriteRequest) (memcontract.Scope, string, error) {
	return resolveMemoryWriteScope(req)
}

func resolveMemoryWriteScope(req contract.MemoryWriteRequest) (memcontract.Scope, string, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return "", "", NewMemoryValidationError(errors.New("content is required"))
	}

	scope, err := parseOptionalMemoryScope(req.Scope)
	if err != nil {
		return "", "", err
	}
	if scope == "" {
		header, err := memory.ParseHeader([]byte(content))
		if err != nil {
			return "", "", err
		}
		scope, err = memcontract.DefaultScopeForType(header.Type)
		if err != nil {
			return "", "", NewMemoryValidationError(err)
		}
	}

	if scope == memcontract.ScopeWorkspace {
		workspace, err := resolveMemoryWorkspace(req.Workspace)
		if err != nil {
			return "", "", err
		}
		return scope, workspace, nil
	}

	return scope, "", nil
}

// ParseOptionalMemoryScope validates an optional memory scope value.
func ParseOptionalMemoryScope(raw string) (memcontract.Scope, error) {
	return parseOptionalMemoryScope(raw)
}

func parseOptionalMemoryScope(raw string) (memcontract.Scope, error) {
	scope := memcontract.Scope(strings.TrimSpace(raw)).Normalize()
	switch scope {
	case "":
		return "", nil
	case memcontract.ScopeGlobal, memcontract.ScopeWorkspace, memcontract.ScopeAgent:
		return scope, nil
	default:
		return "", NewMemoryValidationError(fmt.Errorf("scope must be one of global, workspace, or agent"))
	}
}

// ResolveMemoryWorkspace validates and canonicalizes a workspace memory location.
func ResolveMemoryWorkspace(raw string) (string, error) {
	return resolveMemoryWorkspace(raw)
}

func resolveMemoryWorkspace(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", NewMemoryValidationError(errors.New("workspace is required for workspace scope"))
	}

	workspace, err := filepath.Abs(filepath.Clean(trimmed))
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", trimmed, err)
	}
	return workspace, nil
}

func resolveMemoryScopeAndWorkspace(rawScope string, rawWorkspace string) (memcontract.Scope, string, error) {
	scope, err := parseOptionalMemoryScope(rawScope)
	if err != nil {
		return "", "", err
	}
	if scope == memcontract.ScopeWorkspace || strings.TrimSpace(rawWorkspace) != "" {
		workspace, err := resolveMemoryWorkspace(rawWorkspace)
		if err != nil {
			return "", "", err
		}
		return scope, workspace, nil
	}
	return scope, "", nil
}

func parseMemoryLimit(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(trimmed)
	if err != nil || limit <= 0 {
		return 0, NewMemoryValidationError(errors.New("limit must be a positive integer"))
	}
	return limit, nil
}

func parseMemoryHistoryQuery(c *gin.Context) (memcontract.OperationHistoryQuery, error) {
	scope, workspace, err := resolveMemoryScopeAndWorkspace(
		c.Query("scope"),
		firstNonEmptyString(c.Query("workspace_id"), c.Query("workspace")),
	)
	if err != nil {
		return memcontract.OperationHistoryQuery{}, err
	}
	since, err := ParseOptionalTime(c.Query("since"))
	if err != nil {
		return memcontract.OperationHistoryQuery{}, NewMemoryValidationError(err)
	}
	limit, err := parseMemoryLimit(c.Query("limit"))
	if err != nil {
		return memcontract.OperationHistoryQuery{}, err
	}
	return memcontract.OperationHistoryQuery{
		Scope:     scope,
		Workspace: workspace,
		Operation: memcontract.Operation(strings.TrimSpace(c.Query("operation"))),
		Since:     since,
		Limit:     limit,
	}, nil
}
