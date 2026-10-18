package hooks

import (
	"fmt"

	"slices"
	"strings"
)

const (
	matcherAgentNameKey     = "agent_name"
	matcherInputClassKey    = "input_class"
	matcherLoopNameKey      = "loop_name"
	matcherLoopRunIDKey     = "loop_run_id"
	matcherNodeIDKey        = "node_id"
	matcherReleaseReasonKey = "release_reason"
	matcherRunIDKey         = "run_id"
	matcherTaskIDKey        = "task_id"
	matcherWorkflowIDKey    = "workflow_id"
	matcherWorkspaceIDKey   = "workspace_id"
	matcherWorkspaceRootKey = "workspace_root"
	matcherWorktreeIDKey    = "worktree_id"
)

type matcherFunc[P any] func(HookMatcher, P) bool

var allowedMatcherFieldsByFamily = map[HookEventFamily]map[string]struct{}{
	HookEventFamilySession: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
		"session_type":          {},
	},
	HookEventFamilyInput: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
		matcherInputClassKey:    {},
	},
	HookEventFamilyPrompt: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
		matcherInputClassKey:    {},
	},
	HookEventFamilyEvent: {
		matcherAgentNameKey:  {},
		matcherWorktreeIDKey: {},
		"acp_event_type":     {},
		"turn_id":            {},
	},
	HookEventFamilyAutomation: {
		matcherAgentNameKey:   {},
		matcherWorkspaceIDKey: {},
	},
	HookEventFamilyAgent: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
	},
	HookEventFamilyTurn: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
		matcherInputClassKey:    {},
	},
	HookEventFamilyTool: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
		"tool_id":               {},
		"tool_read_only":        {},
	},
	HookEventFamilyPermission: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
		"tool_name":             {},
		"decision_class":        {},
	},
	HookEventFamilyMessage: {
		matcherWorktreeIDKey: {},
		"message_role":       {},
		"message_delta_type": {},
	},
	HookEventFamilyContext: {
		matcherWorktreeIDKey: {},
		"compaction_trigger": {},
	},
	HookEventFamilyCoordinator: {
		matcherAgentNameKey:      {},
		matcherWorkspaceIDKey:    {},
		matcherWorkspaceRootKey:  {},
		matcherTaskIDKey:         {},
		matcherRunIDKey:          {},
		matcherWorkflowIDKey:     {},
		"coordinator_session_id": {},
	},
	HookEventFamilyTask: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherTaskIDKey:        {},
		matcherRunIDKey:         {},
		matcherWorkflowIDKey:    {},
		matcherReleaseReasonKey: {},
	},
	HookEventFamilyTaskRun: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherTaskIDKey:        {},
		matcherRunIDKey:         {},
		matcherLoopRunIDKey:     {},
		matcherWorkflowIDKey:    {},
		matcherReleaseReasonKey: {},
	},
	HookEventFamilyLoop: {
		matcherAgentNameKey:   {},
		matcherWorkspaceIDKey: {},
		matcherTaskIDKey:      {},
		matcherRunIDKey:       {},
		matcherLoopRunIDKey:   {},
		matcherLoopNameKey:    {},
		matcherNodeIDKey:      {},
		matcherWorkflowIDKey:  {},
	},
	HookEventFamilySpawn: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherTaskIDKey:        {},
		matcherRunIDKey:         {},
		matcherWorkflowIDKey:    {},
		"parent_session_id":     {},
		"root_session_id":       {},
		"child_session_id":      {},
		"spawn_role":            {},
	},
	HookEventFamilyWindowManager: {
		matcherWorkspaceIDKey: {},
	},
	HookEventFamilyWorktree: {
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherWorktreeIDKey:    {},
	},
	HookEventFamilyTerminal: {
		matcherWorkspaceIDKey: {},
	},
}

var allowedMatcherFieldsByEvent = map[HookEvent]map[string]struct{}{
	HookAgentSoulSnapshotResolved: {
		matcherAgentNameKey:   {},
		matcherWorkspaceIDKey: {},
	},
	HookAgentSoulMutationAfter: {
		matcherAgentNameKey:   {},
		matcherWorkspaceIDKey: {},
	},
	HookAgentHeartbeatPolicyResolved: {
		matcherAgentNameKey:   {},
		matcherWorkspaceIDKey: {},
	},
}

// ValidateMatcherForEvent ensures only the matcher fields defined for the event
// family are present.
func ValidateMatcherForEvent(event HookEvent, matcher HookMatcher) error {
	if err := event.Validate(); err != nil {
		return err
	}

	fields := matcherFieldNames(matcher)
	if len(fields) == 0 {
		return nil
	}

	allowed := allowedMatcherFieldsForEvent(event)
	invalid := make([]string, 0, len(fields))
	for _, field := range fields {
		if _, ok := allowed[field]; ok {
			continue
		}
		invalid = append(invalid, field)
	}
	if len(invalid) == 0 {
		return validateMatcherPatterns(matcher)
	}

	slices.Sort(invalid)
	return fmt.Errorf("hooks: matcher fields [%s] are not valid for event %q", strings.Join(invalid, ", "), event)
}

// MatcherFieldAllowedForEvent reports whether a matcher field is valid for the event family.
func MatcherFieldAllowedForEvent(event HookEvent, field string) bool {
	if err := event.Validate(); err != nil {
		return false
	}
	allowed := allowedMatcherFieldsForEvent(event)
	_, ok := allowed[strings.TrimSpace(field)]
	return ok
}

func allowedMatcherFieldsForEvent(event HookEvent) map[string]struct{} {
	if allowed, ok := allowedMatcherFieldsByEvent[event]; ok {
		return allowed
	}
	return allowedMatcherFieldsByFamily[event.Family()]
}
