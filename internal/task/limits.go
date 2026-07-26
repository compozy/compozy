package task

const (
	// MaxMetadataBytes caps task metadata payloads at 16 KiB.
	MaxMetadataBytes = 16 * 1024
	// MaxPayloadBytes caps persisted event-style JSON payloads at 64 KiB.
	MaxPayloadBytes = 64 * 1024
	// MaxResultBytes caps persisted run results at 64 KiB.
	MaxResultBytes = 64 * 1024
	// DefaultTaskMaxAttempts is the canonical retry budget used when callers omit an explicit value.
	DefaultTaskMaxAttempts = 3
	// MaxTaskMaxAttempts caps task-level attempt policy at ten tries.
	MaxTaskMaxAttempts = 10
	// MaxHierarchyDepth caps parent/child nesting at eight levels.
	MaxHierarchyDepth = 8
	// MaxDependencyCount caps dependency edges per task at thirty-two.
	MaxDependencyCount = 32
	// MaxDirectChildren caps direct child tasks per parent at sixty-four.
	MaxDirectChildren = 64
)

const (
	// TaskFieldCreatedBy identifies the immutable creator identity field.
	TaskFieldCreatedBy = "created_by"
	// TaskFieldOrigin identifies the immutable technical ingress field.
	TaskFieldOrigin = "origin"
	// TaskFieldScope identifies the immutable task scope field.
	TaskFieldScope = "scope"
	// TaskFieldWorkspaceID identifies the immutable workspace binding field.
	TaskFieldWorkspaceID = "workspace_id"
	// TaskFieldParentTaskID identifies the immutable parent-task linkage field.
	TaskFieldParentTaskID = "parent_task_id"
	// TaskFieldTitle identifies the mutable task title field.
	TaskFieldTitle = "title"
	// TaskFieldDescription identifies the mutable task description field.
	TaskFieldDescription = "description"
	// TaskFieldPriority identifies the mutable task priority field.
	TaskFieldPriority = "priority"
	// TaskFieldMaxAttempts identifies the mutable task attempt-policy field.
	TaskFieldMaxAttempts = "max_attempts"
	// TaskFieldAutoEnqueueOnReady identifies the mutable auto-enqueue policy field.
	TaskFieldAutoEnqueueOnReady = "auto_enqueue_on_ready"
	// TaskFieldApprovalPolicy identifies the mutable approval-policy field.
	TaskFieldApprovalPolicy = "approval_policy"
	// TaskFieldMetadata identifies the mutable task metadata field.
	TaskFieldMetadata = "metadata"
	// TaskFieldOwner identifies the mutable ownership field.
	TaskFieldOwner = "owner"
)

// ImmutableTaskFields returns the canonical immutable task field names.
func ImmutableTaskFields() []string {
	return []string{
		TaskFieldCreatedBy,
		TaskFieldOrigin,
		TaskFieldScope,
		TaskFieldWorkspaceID,
		TaskFieldParentTaskID,
	}
}

// MutableTaskFields returns the canonical mutable task field names.
func MutableTaskFields() []string {
	return []string{
		TaskFieldTitle,
		TaskFieldDescription,
		TaskFieldPriority,
		TaskFieldMaxAttempts,
		TaskFieldAutoEnqueueOnReady,
		TaskFieldApprovalPolicy,
		TaskFieldMetadata,
		TaskFieldOwner,
	}
}

// IsImmutableTaskField reports whether the supplied field name is immutable after task creation.
func IsImmutableTaskField(field string) bool {
	switch field {
	case TaskFieldCreatedBy, TaskFieldOrigin, TaskFieldScope, TaskFieldWorkspaceID, TaskFieldParentTaskID:
		return true
	default:
		return false
	}
}

// IsMutableTaskField reports whether the supplied field name is directly mutable on a task.
func IsMutableTaskField(field string) bool {
	switch field {
	case TaskFieldTitle,
		TaskFieldDescription,
		TaskFieldPriority,
		TaskFieldMaxAttempts,
		TaskFieldAutoEnqueueOnReady,
		TaskFieldApprovalPolicy,
		TaskFieldMetadata,
		TaskFieldOwner:
		return true
	default:
		return false
	}
}
