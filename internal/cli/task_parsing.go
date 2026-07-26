package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/spf13/cobra"
)

func parseOptionalReviewPolicy(raw string) (taskpkg.ReviewPolicy, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	policy := taskpkg.ReviewPolicy(trimmed).Normalize()
	if err := policy.Validate("review_policy"); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return policy, nil
}

func parseOptionalReviewStatus(raw string) (taskpkg.RunReviewStatus, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	status := taskpkg.RunReviewStatus(trimmed).Normalize()
	if err := status.Validate("review_status"); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return status, nil
}

func parseRequiredReviewOutcome(raw string) (taskpkg.RunReviewOutcome, error) {
	outcome := taskpkg.RunReviewOutcome(strings.TrimSpace(raw)).Normalize()
	if outcome == "" {
		return "", errors.New("cli: --outcome is required")
	}
	if err := outcome.Validate("review_outcome"); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return outcome, nil
}

func missingWorkFromFlags(items []string, raw string) (json.RawMessage, error) {
	hasRaw := strings.TrimSpace(raw) != ""
	if hasRaw && len(items) > 0 {
		return nil, errors.New("cli: --missing-work-json cannot be combined with --missing-work")
	}
	if hasRaw {
		payload, err := parseJSONFlag("missing-work-json", raw)
		if err != nil {
			return nil, err
		}
		var items []json.RawMessage
		if err := json.Unmarshal(payload, &items); err != nil {
			return nil, errors.New("cli: --missing-work-json must be a JSON array")
		}
		return payload, nil
	}

	normalized := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			normalized = append(normalized, trimmed)
		}
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("cli: encode --missing-work: %w", err)
	}
	return payload, nil
}

func resolveTaskScopeWorkspace(
	rawScope string,
	workspaceRef string,
	scopeRequired bool,
) (taskpkg.Scope, string, error) {
	scope, err := parseOptionalTaskScope(rawScope)
	if err != nil {
		return "", "", err
	}
	if scopeRequired && scope == "" {
		return "", "", errors.New("cli: --scope is required")
	}

	workspace := strings.TrimSpace(workspaceRef)
	switch scope.Normalize() {
	case taskpkg.ScopeGlobal:
		if workspace != "" {
			return "", "", errors.New("cli: --workspace must be empty when --scope is global")
		}
	case taskpkg.ScopeWorkspace:
		if workspace == "" {
			return "", "", errors.New("cli: --workspace is required when --scope is workspace")
		}
	}
	return scope, workspace, nil
}

func parseOptionalTaskScope(raw string) (taskpkg.Scope, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	scope := taskpkg.Scope(trimmed)
	if err := scope.Validate(taskScopeKey); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return scope, nil
}

func parseOptionalTaskStatus(raw string) (taskpkg.Status, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	status := taskpkg.Status(trimmed)
	if err := status.Validate(taskStatusKey); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return status, nil
}

func parseOptionalTaskRunStatus(raw string) (taskpkg.RunStatus, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return taskpkg.TaskRunStatusUnknown, nil
	}
	status := taskpkg.ParseRunStatus(trimmed)
	if err := status.Validate(taskStatusKey); err != nil {
		return taskpkg.TaskRunStatusUnknown, fmt.Errorf("cli: %w", err)
	}
	return status, nil
}

func parseOptionalTaskOwnerKind(raw string) (taskpkg.OwnerKind, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	kind := taskpkg.OwnerKind(trimmed)
	if err := kind.Validate("owner_kind"); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return kind, nil
}

func parseOptionalTaskDependencyKind(raw string) (taskpkg.DependencyKind, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	kind := taskpkg.DependencyKind(trimmed)
	if err := kind.Validate(taskKindKey); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return kind, nil
}

func parseRequiredTaskOwnership(kindRaw string, refRaw string) (*taskpkg.Ownership, error) {
	kindText := strings.TrimSpace(kindRaw)
	ref := strings.TrimSpace(refRaw)
	if kindText == "" || ref == "" {
		return nil, errors.New("cli: --owner-kind and --owner-ref must be provided together")
	}
	kind, err := parseOptionalTaskOwnerKind(kindText)
	if err != nil {
		return nil, err
	}
	owner := &taskpkg.Ownership{Kind: kind, Ref: ref}
	if err := owner.Validate(taskOwnerKey); err != nil {
		return nil, fmt.Errorf("cli: %w", err)
	}
	return owner, nil
}

func parseJSONFlag(flagName string, raw string) (json.RawMessage, error) {
	payload, err := parseRequiredJSONRawMessage(raw)
	if errors.Is(err, errEmptyJSONFlag) {
		return nil, fmt.Errorf("cli: --%s requires valid JSON", flagName)
	}
	if err != nil {
		return nil, fmt.Errorf("cli: invalid --%s JSON: %w", flagName, err)
	}
	return payload, nil
}

func parseAgentTaskJSONFlag(flagName string, raw string) (json.RawMessage, error) {
	payload, err := parseJSONFlag(flagName, raw)
	if err != nil {
		return nil, err
	}
	if err := contract.ValidateNoRawClaimTokenField(payload); err != nil {
		return nil, fmt.Errorf(
			"cli: --%s must not contain raw lease credential fields: %w",
			flagName,
			err,
		)
	}
	return payload, nil
}

func requiredAgentTaskRunID(rawRunID string) (string, error) {
	runID := strings.TrimSpace(rawRunID)
	if runID == "" {
		return "", errors.New("cli: run id is required")
	}
	return runID, nil
}

func requiredTaskID(rawTaskID string) (string, error) {
	taskID := strings.TrimSpace(rawTaskID)
	if taskID == "" {
		return "", errors.New("cli: task id is required")
	}
	return taskID, nil
}

func requiredTaskRunIDs(rawRunIDs []string) ([]string, error) {
	if len(rawRunIDs) == 0 {
		return nil, errors.New("cli: at least one run id is required")
	}
	runIDs := make([]string, 0, len(rawRunIDs))
	for _, rawRunID := range rawRunIDs {
		runID, err := requiredAgentTaskRunID(rawRunID)
		if err != nil {
			return nil, err
		}
		runIDs = append(runIDs, runID)
	}
	return runIDs, nil
}

func validateAgentTaskLeaseSeconds(seconds int64) error {
	if seconds < 0 {
		return fmt.Errorf("cli: --lease-seconds must be zero or positive: %d", seconds)
	}
	maxSeconds := int64(taskpkg.MaxRunLeaseDuration.Seconds())
	if seconds > maxSeconds {
		return fmt.Errorf("cli: --lease-seconds must be less than or equal to %d", maxSeconds)
	}
	return nil
}

func trimAgentTaskCapabilities(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		trimmed = append(trimmed, value)
	}
	if len(trimmed) == 0 {
		return nil
	}
	return trimmed
}

func buildTaskCreateRequest(cmd *cobra.Command, input taskCreateInput) (CreateTaskRequest, error) {
	scope, workspace, err := resolveTaskScopeWorkspace(input.ScopeRaw, input.WorkspaceRef, true)
	if err != nil {
		return CreateTaskRequest{}, err
	}
	owner, err := parseOptionalTaskOwnership(cmd, input.OwnerKindRaw, input.OwnerRef)
	if err != nil {
		return CreateTaskRequest{}, err
	}
	metadata, err := parseOptionalTaskMetadata(cmd, input.MetadataRaw)
	if err != nil {
		return CreateTaskRequest{}, err
	}
	priority, err := parseOptionalChangedTaskPriority(cmd, input.PriorityRaw)
	if err != nil {
		return CreateTaskRequest{}, err
	}

	request := CreateTaskRequest{
		ID:                 strings.TrimSpace(input.ID),
		Identifier:         strings.TrimSpace(input.Identifier),
		Scope:              scope,
		Workspace:          workspace,
		Title:              strings.TrimSpace(input.Title),
		Description:        strings.TrimSpace(input.Description),
		Priority:           priority,
		AutoEnqueueOnReady: input.AutoEnqueueOnReady,
		Owner:              owner,
		Metadata:           metadata,
	}
	participationRequest, err := input.NetworkFlags.request()
	if err != nil {
		return CreateTaskRequest{}, err
	}
	request.NetworkParticipation = participationRequest
	if input.NoWakeCreator {
		wakeCreator := false
		request.WakeCreator = &wakeCreator
	}
	if request.Title == "" {
		return CreateTaskRequest{}, errors.New("cli: --title is required")
	}
	return request, nil
}

func parseOptionalTaskOwnership(
	cmd *cobra.Command,
	ownerKindRaw string,
	ownerRef string,
) (*taskpkg.Ownership, error) {
	if !cmd.Flags().Changed("owner-kind") && !cmd.Flags().Changed("owner-ref") {
		return nil, nil
	}
	return parseRequiredTaskOwnership(ownerKindRaw, ownerRef)
}

func parseOptionalTaskMetadata(cmd *cobra.Command, metadataRaw string) (json.RawMessage, error) {
	if !cmd.Flags().Changed("metadata") {
		return nil, nil
	}
	return parseJSONFlag("metadata", metadataRaw)
}

func parseOptionalChangedTaskPriority(cmd *cobra.Command, raw string) (taskpkg.Priority, error) {
	if !cmd.Flags().Changed("priority") {
		return "", nil
	}
	return parseOptionalTaskPriority(raw)
}

func parseOptionalTaskPriority(raw string) (taskpkg.Priority, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", errors.New("cli: --priority cannot be blank")
	}
	priority := taskpkg.Priority(trimmed)
	if err := priority.Validate("priority"); err != nil {
		return "", fmt.Errorf("cli: %w", err)
	}
	return priority, nil
}

func validateTaskLast(last int) error {
	if last < 0 {
		return fmt.Errorf("cli: --last must be zero or positive: %d", last)
	}
	return nil
}
