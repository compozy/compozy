package task

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Validate reports whether the actor identity contains a supported kind and non-empty reference.
func (a ActorIdentity) Validate(path string) error {
	if err := a.Kind.Validate(nestedPath(path, "kind")); err != nil {
		return err
	}
	if strings.TrimSpace(a.Ref) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "ref"))
	}
	return nil
}

// IsZero reports whether the actor identity is empty.
func (a ActorIdentity) IsZero() bool {
	return a.Kind.Normalize() == "" && strings.TrimSpace(a.Ref) == ""
}

// Validate reports whether the ownership value contains a supported kind and non-empty reference.
func (o Ownership) Validate(path string) error {
	if err := o.Kind.Validate(nestedPath(path, "kind")); err != nil {
		return err
	}
	if strings.TrimSpace(o.Ref) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "ref"))
	}
	return nil
}

// IsZero reports whether the ownership value is empty.
func (o Ownership) IsZero() bool {
	return o.Kind.Normalize() == "" && strings.TrimSpace(o.Ref) == ""
}

// Validate reports whether the origin contains a supported kind and non-empty reference.
func (o Origin) Validate(path string) error {
	if err := o.Kind.Validate(nestedPath(path, "kind")); err != nil {
		return err
	}
	if strings.TrimSpace(o.Ref) == "" {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "ref"))
	}
	return nil
}

// Validate reports whether the authority flags are internally consistent.
func (a Authority) Validate(path string) error {
	if !a.Write && (a.CreateGlobal || a.CreateWorkspace) {
		return fmt.Errorf("%w: %s create permissions require write permission", ErrValidation, path)
	}
	return nil
}

// Validate reports whether caller ownership is canonical for the actor kind.
func (s CallerScope) Validate(actor ActorIdentity) error {
	sessionID := strings.TrimSpace(s.SessionID)
	workspaceID := strings.TrimSpace(s.WorkspaceID)
	if sessionID != s.SessionID {
		return fmt.Errorf("%w: scope.session_id must be canonical", ErrValidation)
	}
	if workspaceID != s.WorkspaceID {
		return fmt.Errorf("%w: scope.workspace_id must be canonical", ErrValidation)
	}

	switch actor.Kind.Normalize() {
	case ActorKindAgentSession:
		if s.Operator {
			return fmt.Errorf("%w: scope.operator is not allowed for agent_session", ErrValidation)
		}
		if sessionID == "" {
			return fmt.Errorf("%w: scope.session_id is required for agent_session", ErrValidation)
		}
		if sessionID != strings.TrimSpace(actor.Ref) {
			return fmt.Errorf("%w: scope.session_id must match actor.ref", ErrValidation)
		}
		if workspaceID == "" {
			return fmt.Errorf("%w: scope.workspace_id is required for agent_session", ErrValidation)
		}
	case ActorKindHuman:
		if !s.Operator {
			return fmt.Errorf("%w: scope.operator is required for human", ErrValidation)
		}
		if sessionID != "" {
			return fmt.Errorf("%w: scope.session_id is not allowed for human", ErrValidation)
		}
	default:
		if s.Operator {
			return fmt.Errorf("%w: scope.operator is reserved for human", ErrValidation)
		}
		if sessionID != "" {
			return fmt.Errorf("%w: scope.session_id is reserved for agent_session", ErrValidation)
		}
	}
	return nil
}

// Validate reports whether the actor context contains a valid principal, origin, and authority envelope.
func (a ActorContext) Validate() error {
	if err := a.Actor.Validate("actor"); err != nil {
		return err
	}
	if err := a.Origin.Validate("origin"); err != nil {
		return err
	}
	if err := validateActorOriginPair(a.Actor, a.Origin); err != nil {
		return err
	}
	if err := a.Authority.Validate("authority"); err != nil {
		return err
	}
	if err := a.Scope.Validate(a.Actor); err != nil {
		return err
	}
	return nil
}

// Validate reports whether the task record contains the canonical persisted shape.
func (t Task) Validate() error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("%w: task.id is required", ErrValidation)
	}
	if err := ValidateScopeBinding(t.Scope, t.WorkspaceID, "task", "workspace_id"); err != nil {
		return err
	}
	if strings.TrimSpace(t.ParentTaskID) != "" &&
		strings.TrimSpace(t.ParentTaskID) == strings.TrimSpace(t.ID) {
		return fmt.Errorf("%w: task.parent_task_id cannot equal task.id", ErrValidation)
	}
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("%w: task.title is required", ErrValidation)
	}
	if err := t.Status.Validate("task.status"); err != nil {
		return err
	}
	if t.Status.Normalize() == TaskStatusDraft && !t.ClosedAt.IsZero() {
		return fmt.Errorf(
			"%w: task.closed_at must be empty while task.status is %q",
			ErrValidation,
			TaskStatusDraft,
		)
	}
	if err := normalizePriorityOrDefault(t.Priority).Validate("task.priority"); err != nil {
		return err
	}
	if err := validateTaskMaxAttempts(t.MaxAttempts, "task.max_attempts", true); err != nil {
		return err
	}
	approvalPolicy := normalizeApprovalPolicyOrDefault(t.ApprovalPolicy)
	if err := approvalPolicy.Validate("task.approval_policy"); err != nil {
		return err
	}
	approvalState := normalizeApprovalStateOrDefault(approvalPolicy, t.ApprovalState)
	if err := approvalState.Validate("task.approval_state"); err != nil {
		return err
	}
	if err := ValidateApprovalSemantics(approvalPolicy, approvalState, "task"); err != nil {
		return err
	}
	if err := t.CreatedBy.Validate("task.created_by"); err != nil {
		return err
	}
	if err := validateNeedsAttention(t.NeedsAttention); err != nil {
		return err
	}
	if err := t.Origin.Validate("task.origin"); err != nil {
		return err
	}
	if t.Owner != nil {
		if err := t.Owner.Validate("task.owner"); err != nil {
			return err
		}
	}
	if err := ValidateMetadataSize(t.Metadata, "task.metadata"); err != nil {
		return err
	}
	return nil
}

func validateNeedsAttention(attention *NeedsAttention) error {
	if attention == nil {
		return nil
	}
	if strings.TrimSpace(attention.Reason) == "" {
		return fmt.Errorf("%w: task.needs_attention.reason is required", ErrValidation)
	}
	if attention.At.IsZero() {
		return fmt.Errorf("%w: task.needs_attention.at is required", ErrValidation)
	}
	if err := attention.By.Validate("task.needs_attention.by"); err != nil {
		return err
	}
	return nil
}

// Validate reports whether the dependency edge contains the canonical persisted shape.
func (d Dependency) Validate() error {
	if strings.TrimSpace(d.TaskID) == "" {
		return fmt.Errorf("%w: task_dependency.task_id is required", ErrValidation)
	}
	if strings.TrimSpace(d.DependsOnTaskID) == "" {
		return fmt.Errorf("%w: task_dependency.depends_on_task_id is required", ErrValidation)
	}
	if strings.TrimSpace(d.TaskID) == strings.TrimSpace(d.DependsOnTaskID) {
		return fmt.Errorf("%w: task_dependency cannot depend on itself", ErrValidation)
	}
	if err := d.Kind.Validate("task_dependency.kind"); err != nil {
		return err
	}
	return nil
}

// ValidateCapabilityIDs reports whether every capability identifier is safe for exact matching.
func ValidateCapabilityIDs(values []string, path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%w: capability path is required", ErrValidation)
	}
	seen := make(map[string]struct{}, len(values))
	for idx, raw := range values {
		value := strings.TrimSpace(raw)
		field := fmt.Sprintf("%s[%d]", path, idx)
		if value == "" {
			return fmt.Errorf("%w: %s is required", ErrValidation, field)
		}
		if len(value) > maxCapabilityIDLength {
			return fmt.Errorf(
				"%w: %s exceeds %d bytes",
				ErrValidation,
				field,
				maxCapabilityIDLength,
			)
		}
		if containsCapabilitySeparator(value) {
			return fmt.Errorf("%w: %s must not contain whitespace or commas", ErrValidation, field)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%w: %s duplicates capability %q", ErrValidation, field, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func containsCapabilitySeparator(value string) bool {
	return strings.ContainsAny(value, " \t\r\n,")
}

func hasRawClaimTokenField(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return false
	}
	return hasRawClaimTokenFieldValue(decoded)
}

func hasRawClaimTokenFieldValue(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, nested := range typed {
			if strings.EqualFold(strings.TrimSpace(key), "claim_token") {
				return true
			}
			if hasRawClaimTokenFieldValue(nested) {
				return true
			}
		}
	case []any:
		if slices.ContainsFunc(typed, hasRawClaimTokenFieldValue) {
			return true
		}
	}
	return false
}
