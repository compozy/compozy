package hooks

import (
	"fmt"

	"sort"
	"strings"
)

const (
	matcherAgentNameKey            = "agent_name"
	matcherParticipationChannelKey = "participation_channel"
	matcherInputClassKey           = "input_class"
	matcherLoopNameKey             = "loop_name"
	matcherLoopRunIDKey            = "loop_run_id"
	matcherNodeIDKey               = "node_id"
	matcherReleaseReasonKey        = "release_reason"
	matcherRunIDKey                = "run_id"
	matcherTaskIDKey               = "task_id"
	matcherWorkflowIDKey           = "workflow_id"
	matcherWorkspaceIDKey          = "workspace_id"
	matcherWorkspaceRootKey        = "workspace_root"
)

type matcherFunc[P any] func(HookMatcher, P) bool

var allowedMatcherFieldsByFamily = map[HookEventFamily]map[string]struct{}{
	HookEventFamilySession: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		"session_type":          {},
	},
	HookEventFamilySandbox: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		"sandbox_id":            {},
		"sandbox_backend":       {},
		"sandbox_profile":       {},
		"sync_direction":        {},
	},
	HookEventFamilyInput: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherInputClassKey:    {},
	},
	HookEventFamilyPrompt: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherInputClassKey:    {},
	},
	HookEventFamilyEvent: {
		matcherAgentNameKey: {},
		"acp_event_type":    {},
		"turn_id":           {},
	},
	HookEventFamilyAutomation: {
		matcherAgentNameKey:   {},
		matcherWorkspaceIDKey: {},
	},
	HookEventFamilyAgent: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
	},
	HookEventFamilyTurn: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		matcherInputClassKey:    {},
	},
	HookEventFamilyTool: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		"tool_id":               {},
		"tool_read_only":        {},
	},
	HookEventFamilyPermission: {
		matcherAgentNameKey:     {},
		matcherWorkspaceIDKey:   {},
		matcherWorkspaceRootKey: {},
		"tool_name":             {},
		"decision_class":        {},
	},
	HookEventFamilyMessage: {
		"message_role":       {},
		"message_delta_type": {},
	},
	HookEventFamilyContext: {
		"compaction_reason":   {},
		"compaction_strategy": {},
	},
	HookEventFamilyCoordinator: {
		matcherAgentNameKey:            {},
		matcherWorkspaceIDKey:          {},
		matcherWorkspaceRootKey:        {},
		matcherTaskIDKey:               {},
		matcherRunIDKey:                {},
		matcherWorkflowIDKey:           {},
		matcherParticipationChannelKey: {},
		"coordinator_session_id":       {},
	},
	HookEventFamilyTask: {
		matcherAgentNameKey:            {},
		matcherWorkspaceIDKey:          {},
		matcherTaskIDKey:               {},
		matcherRunIDKey:                {},
		matcherWorkflowIDKey:           {},
		matcherParticipationChannelKey: {},
		matcherReleaseReasonKey:        {},
	},
	HookEventFamilyTaskRun: {
		matcherAgentNameKey:            {},
		matcherWorkspaceIDKey:          {},
		matcherTaskIDKey:               {},
		matcherRunIDKey:                {},
		matcherLoopRunIDKey:            {},
		matcherWorkflowIDKey:           {},
		matcherParticipationChannelKey: {},
		matcherReleaseReasonKey:        {},
	},
	HookEventFamilyLoop: {
		matcherAgentNameKey:            {},
		matcherWorkspaceIDKey:          {},
		matcherTaskIDKey:               {},
		matcherRunIDKey:                {},
		matcherLoopRunIDKey:            {},
		matcherLoopNameKey:             {},
		matcherNodeIDKey:               {},
		matcherWorkflowIDKey:           {},
		matcherParticipationChannelKey: {},
	},
	HookEventFamilySpawn: {
		matcherAgentNameKey:            {},
		matcherWorkspaceIDKey:          {},
		matcherWorkspaceRootKey:        {},
		matcherTaskIDKey:               {},
		matcherRunIDKey:                {},
		matcherWorkflowIDKey:           {},
		matcherParticipationChannelKey: {},
		"parent_session_id":            {},
		"root_session_id":              {},
		"child_session_id":             {},
		"spawn_role":                   {},
	},
	HookEventFamilyNetwork: {
		matcherChannelKey:   {},
		"surface":           {},
		"kind":              {},
		"direction":         {},
		matcherWorkStateKey: {},
	},
	HookEventFamilyWindowManager: {
		matcherWorkspaceIDKey: {},
	},
}

var allowedMatcherFieldsByEvent = map[HookEvent]map[string]struct{}{
	HookNetworkParticipationPreResolve: {
		matcherWorkspaceIDKey:         {},
		matcherChannelKey:             {},
		matcherParticipationModeKey:   {},
		matcherParticipationSourceKey: {},
	},
	HookNetworkParticipationResolved: {
		matcherWorkspaceIDKey:         {},
		matcherChannelKey:             {},
		matcherParticipationModeKey:   {},
		matcherParticipationSourceKey: {},
	},
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

	sort.Strings(invalid)
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
