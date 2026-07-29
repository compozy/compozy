package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

const nativeLoopApprovalHashLen = len("sha256:") + 64

func decodeNativeLoopInput(req toolspkg.CallRequest, dst any) error {
	if path, found := retiredNativeLoopRuntimePath(req.Input); found {
		message := fmt.Sprintf(
			"retired runtime key %s; see MIGRATION_GUIDE.md#per-task-runtime-selection",
			path,
		)
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			req.ToolID,
			message,
			fmt.Errorf("%w: %s", toolspkg.ErrToolInvalidInput, message),
			toolspkg.ReasonSchemaInvalid,
		)
	}
	return decodeNativeInput(req, dst)
}

func retiredNativeLoopRuntimePath(raw json.RawMessage) (string, bool) {
	return looppkg.RetiredRuntimeKeyPath(raw)
}

func (n *daemonNativeTools) nativeLoopWorkspaceID(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceID string,
	scope toolspkg.Scope,
) (string, error) {
	resolved := nativeCallerWorkspaceInput(workspaceID, scope)
	if strings.TrimSpace(resolved) == "" {
		return "", nativeRequiredInputError(id, nativeWorkspaceInputKey)
	}
	resolved = strings.TrimSpace(resolved)
	if n == nil || n.deps == nil || n.deps.Workspaces == nil {
		return resolved, nil
	}
	workspace, err := n.deps.Workspaces.Resolve(ctx, resolved)
	if err != nil {
		return "", nativeNetworkInputError(id, err)
	}
	registryID, err := nativeResolvedRegistryWorkspaceID(&workspace)
	if err != nil {
		return "", nativeNetworkInputError(id, err)
	}
	return registryID, nil
}

func (n *daemonNativeTools) nativeLoopWorkspaceAndName(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceID string,
	name string,
	scope toolspkg.Scope,
) (string, string, error) {
	resolvedWorkspaceID, err := n.nativeLoopWorkspaceID(ctx, id, workspaceID, scope)
	if err != nil {
		return "", "", err
	}
	resolvedName, err := requiredNativeString(id, "name", name)
	if err != nil {
		return "", "", err
	}
	return resolvedWorkspaceID, resolvedName, nil
}

func (n *daemonNativeTools) nativeLoopWorkspaceAndRunID(
	ctx context.Context,
	id toolspkg.ToolID,
	workspaceID string,
	runID string,
	scope toolspkg.Scope,
) (string, string, error) {
	resolvedWorkspaceID, err := n.nativeLoopWorkspaceID(ctx, id, workspaceID, scope)
	if err != nil {
		return "", "", err
	}
	resolvedRunID, err := requiredNativeString(id, "run_id", runID)
	if err != nil {
		return "", "", err
	}
	return resolvedWorkspaceID, resolvedRunID, nil
}

func nativeLoopGateDecisionValid(decision looppkg.GateDecision) bool {
	switch decision {
	case looppkg.GateDecisionApprove, looppkg.GateDecisionRequestChanges, looppkg.GateDecisionReject:
		return true
	default:
		return false
	}
}

func validateNativeLoopApprovalHash(id toolspkg.ToolID, value string) error {
	hash := strings.TrimSpace(value)
	if hash == "" {
		return nil
	}
	if len(hash) != nativeLoopApprovalHashLen || !strings.HasPrefix(hash, "sha256:") {
		return nativeLoopApprovalHashError(id)
	}
	for _, char := range hash[len("sha256:"):] {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return nativeLoopApprovalHashError(id)
		}
	}
	return nil
}

func nativeLoopApprovalHashError(id toolspkg.ToolID) error {
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeInvalidInput,
		id,
		"approval_token_hash must be sha256:<64 lowercase hex>",
		toolspkg.ErrToolInvalidInput,
		toolspkg.ReasonSchemaInvalid,
	)
}

func nativeLoopApproveError(id toolspkg.ToolID, err error) error {
	if !errors.Is(err, taskpkg.ErrPermissionDenied) {
		return nativeLoopToolError(id, err)
	}
	return toolspkg.NewToolError(
		toolspkg.ErrorCodeDenied,
		id,
		err.Error(),
		fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
		toolspkg.ReasonApprovalSelfDenied,
		toolspkg.ReasonPolicyDenied,
	)
}

func nativeLoopToolError(id toolspkg.ToolID, err error) error {
	if err == nil {
		return nil
	}
	code := toolspkg.ErrorCodeBackendFailed
	cause := toolspkg.ErrToolBackendFailed
	reason := toolspkg.ReasonBackendUnhealthy
	switch core.StatusForLoopError(err) {
	case http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusRequestEntityTooLarge:
		code = toolspkg.ErrorCodeInvalidInput
		cause = toolspkg.ErrToolInvalidInput
		reason = toolspkg.ReasonSchemaInvalid
	case http.StatusForbidden:
		code = toolspkg.ErrorCodeDenied
		cause = toolspkg.ErrToolDenied
		reason = toolspkg.ReasonPolicyDenied
	case http.StatusNotFound:
		code = toolspkg.ErrorCodeNotFound
		cause = toolspkg.ErrToolNotFound
		reason = toolspkg.ReasonToolUnknown
	case http.StatusConflict:
		code = toolspkg.ErrorCodeConflict
		cause = toolspkg.ErrToolConflict
		reason = toolspkg.ReasonConflictedID
	case http.StatusServiceUnavailable:
		code = toolspkg.ErrorCodeUnavailable
		cause = toolspkg.ErrToolUnavailable
		reason = toolspkg.ReasonDependencyMissing
	}
	toolErr := toolspkg.NewToolError(code, id, err.Error(), fmt.Errorf("%w: %w", cause, err), reason)
	if inputDefault, ok := errors.AsType[*looppkg.InputDefaultError](err); ok {
		structured, marshalErr := json.Marshal(map[string]any{
			nativeLoopValidationKey: contract.LoopValidationResponse{
				Valid: false,
				InputDefault: &contract.LoopInputDefaultErrorPayload{
					Loop: inputDefault.Loop, Key: inputDefault.Key, Reason: string(inputDefault.Reason),
				},
			},
		})
		if marshalErr != nil {
			return toolspkg.NewToolError(
				toolspkg.ErrorCodeBackendFailed,
				id,
				"marshal Loop input-default validation payload",
				fmt.Errorf("%w: %w", toolspkg.ErrToolBackendFailed, marshalErr),
				toolspkg.ReasonBackendUnhealthy,
			)
		}
		return toolErr.WithPartialResult(toolspkg.ToolResult{
			Structured: structured,
			Preview:    "loop input default validation failed",
		})
	}
	return toolErr
}

type nativeLoopWorkspaceInput struct {
	WorkspaceID string              `json:"workspace,omitempty"`
	Q           string              `json:"q,omitempty"`
	Kind        looppkg.CatalogKind `json:"kind,omitempty"`
	Category    string              `json:"category,omitempty"`
	Status      looppkg.Status      `json:"status,omitempty"`
	Sort        string              `json:"sort,omitempty"`
	Cursor      string              `json:"cursor,omitempty"`
	Limit       int                 `json:"limit,omitempty"`
}

type nativeLoopNameInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	Name        string `json:"name"`
}

type nativeLoopRunIDInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	RunID       string `json:"run_id"`
}

type nativeLoopValidateInput struct {
	WorkspaceID string         `json:"workspace,omitempty"`
	Name        string         `json:"name,omitempty"`
	Definition  dsl.Definition `json:"definition"`
}

type nativeLoopCreateInput struct {
	WorkspaceID     string          `json:"workspace,omitempty"`
	Definition      *dsl.Definition `json:"definition,omitempty"`
	ForkFromName    string          `json:"fork_from_name,omitempty"`
	ExpectedVersion *int            `json:"expected_version,omitempty"`
}

type nativeLoopRunInput struct {
	WorkspaceID          string                 `json:"workspace,omitempty"`
	Name                 string                 `json:"name"`
	Inputs               map[string]any         `json:"inputs,omitempty"`
	ParentLoopRunID      string                 `json:"parent_loop_run_id,omitempty"`
	ConfigOverrides      *looppkg.LoopConfig    `json:"config_overrides,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Dry                  bool                   `json:"dry,omitempty"`
}

type nativeLoopRunsInput struct {
	WorkspaceID string `json:"workspace,omitempty"`
	LoopName    string `json:"loop_name,omitempty"`
	Status      string `json:"status,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type nativeLoopConfigureInput struct {
	WorkspaceID string             `json:"workspace,omitempty"`
	Name        string             `json:"name"`
	Config      looppkg.LoopConfig `json:"config"`
}

type nativeLoopApproveInput struct {
	WorkspaceID       string `json:"workspace,omitempty"`
	RunID             string `json:"run_id"`
	GateID            string `json:"gate_id"`
	Decision          string `json:"decision"`
	ApprovalTokenHash string `json:"approval_token_hash,omitempty"`
}
