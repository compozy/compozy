package daemon

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	core "github.com/compozy/agh/internal/api/core"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const nativeLoopApprovalHashLen = len("sha256:") + 64

func nativeLoopWorkspaceID(id toolspkg.ToolID, workspaceID string, scope toolspkg.Scope) (string, error) {
	resolved, err := nativeCallerWorkspaceInput(id, "workspace_id", workspaceID, scope)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resolved) == "" {
		return "", nativeRequiredInputError(id, "workspace_id")
	}
	return strings.TrimSpace(resolved), nil
}

func nativeLoopWorkspaceAndName(
	id toolspkg.ToolID,
	workspaceID string,
	name string,
	scope toolspkg.Scope,
) (string, string, error) {
	resolvedWorkspaceID, err := nativeLoopWorkspaceID(id, workspaceID, scope)
	if err != nil {
		return "", "", err
	}
	resolvedName, err := requiredNativeString(id, "name", name)
	if err != nil {
		return "", "", err
	}
	return resolvedWorkspaceID, resolvedName, nil
}

func nativeLoopWorkspaceAndRunID(
	id toolspkg.ToolID,
	workspaceID string,
	runID string,
	scope toolspkg.Scope,
) (string, string, error) {
	resolvedWorkspaceID, err := nativeLoopWorkspaceID(id, workspaceID, scope)
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
	return toolspkg.NewToolError(code, id, err.Error(), fmt.Errorf("%w: %w", cause, err), reason)
}

type nativeLoopWorkspaceInput struct {
	WorkspaceID string              `json:"workspace_id,omitempty"`
	Q           string              `json:"q,omitempty"`
	Kind        looppkg.CatalogKind `json:"kind,omitempty"`
	Category    string              `json:"category,omitempty"`
	Status      looppkg.Status      `json:"status,omitempty"`
	Sort        string              `json:"sort,omitempty"`
	Cursor      string              `json:"cursor,omitempty"`
	Limit       int                 `json:"limit,omitempty"`
}

type nativeLoopNameInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	Name        string `json:"name"`
}

type nativeLoopRunIDInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	RunID       string `json:"run_id"`
}

type nativeLoopValidateInput struct {
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Definition  dsl.Definition `json:"definition"`
}

type nativeLoopCreateInput struct {
	WorkspaceID     string          `json:"workspace_id,omitempty"`
	Definition      *dsl.Definition `json:"definition,omitempty"`
	ForkFromName    string          `json:"fork_from_name,omitempty"`
	ExpectedVersion *int            `json:"expected_version,omitempty"`
}

type nativeLoopRunInput struct {
	WorkspaceID          string                 `json:"workspace_id,omitempty"`
	Name                 string                 `json:"name"`
	Inputs               map[string]any         `json:"inputs,omitempty"`
	ParentLoopRunID      string                 `json:"parent_loop_run_id,omitempty"`
	ConfigOverrides      *looppkg.LoopConfig    `json:"config_overrides,omitempty"`
	NetworkParticipation *participation.Request `json:"network_participation,omitempty"`
	Dry                  bool                   `json:"dry,omitempty"`
}

type nativeLoopRunsInput struct {
	WorkspaceID string `json:"workspace_id,omitempty"`
	LoopName    string `json:"loop_name,omitempty"`
	Status      string `json:"status,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type nativeLoopConfigureInput struct {
	WorkspaceID string             `json:"workspace_id,omitempty"`
	Name        string             `json:"name"`
	Config      looppkg.LoopConfig `json:"config"`
}

type nativeLoopApproveInput struct {
	WorkspaceID       string `json:"workspace_id,omitempty"`
	RunID             string `json:"run_id"`
	GateID            string `json:"gate_id"`
	Decision          string `json:"decision"`
	ApprovalTokenHash string `json:"approval_token_hash,omitempty"`
}
