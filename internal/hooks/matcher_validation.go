package hooks

import (
	"fmt"
	"path"

	"strings"
)

func matcherFieldNames(matcher HookMatcher) []string {
	fields := make([]string, 0, 16)

	appendIf := func(name string, present bool) {
		if present {
			fields = append(fields, name)
		}
	}

	appendIf(matcherAgentNameKey, matcher.AgentName != "")
	appendIf("agent_type", matcher.AgentType != "")
	appendIf(matcherWorkspaceIDKey, matcher.WorkspaceID != "")
	appendIf(matcherWorkspaceRootKey, matcher.WorkspaceRoot != "")
	appendIf("session_type", matcher.SessionType != "")
	appendIf("sandbox_id", matcher.SandboxID != "")
	appendIf("sandbox_backend", matcher.SandboxBackend != "")
	appendIf("sandbox_profile", matcher.SandboxProfile != "")
	appendIf("sync_direction", matcher.SyncDirection != "")
	appendIf(matcherInputClassKey, matcher.InputClass != "")
	appendIf("acp_event_type", matcher.ACPEventType != "")
	appendIf("turn_id", matcher.TurnID != "")
	appendIf("tool_id", matcher.ToolID != "")
	appendIf("tool_name", matcher.ToolName != "")
	appendIf("tool_read_only", matcher.ToolReadOnly != nil)
	appendIf("decision_class", matcher.DecisionClass != "")
	appendIf("message_role", matcher.MessageRole != "")
	appendIf("message_delta_type", matcher.MessageDeltaType != "")
	if matcher.NetworkMatcher != nil {
		appendNetworkMatcherFieldNames(&fields, matcher.NetworkMatcher)
	}
	if matcher.CompactionMatcher != nil {
		appendCompactionMatcherFieldNames(&fields, matcher.CompactionMatcher)
	}
	if matcher.Autonomy != nil {
		appendAutonomyMatcherFieldNames(&fields, matcher.Autonomy)
	}

	return fields
}

func appendCompactionMatcherFieldNames(fields *[]string, matcher *CompactionMatcher) {
	appendIf := func(name string, present bool) {
		if present {
			*fields = append(*fields, name)
		}
	}

	appendIf("compaction_reason", matcher.Reason != "")
	appendIf("compaction_strategy", matcher.Strategy != "")
}

func appendAutonomyMatcherFieldNames(fields *[]string, matcher *AutonomyMatcher) {
	appendIf := func(name string, present bool) {
		if present {
			*fields = append(*fields, name)
		}
	}

	appendIf(matcherTaskIDKey, matcher.TaskID != "")
	appendIf(matcherRunIDKey, matcher.RunID != "")
	appendIf(matcherLoopRunIDKey, matcher.LoopRunID != "")
	appendIf(matcherLoopNameKey, matcher.LoopName != "")
	appendIf(matcherNodeIDKey, matcher.NodeID != "")
	appendIf(matcherWorkflowIDKey, matcher.WorkflowID != "")
	appendIf(matcherParticipationChannelKey, matcher.ParticipationChannel != "")
	appendIf("coordinator_session_id", matcher.CoordinatorSessionID != "")
	appendIf("parent_session_id", matcher.ParentSessionID != "")
	appendIf("root_session_id", matcher.RootSessionID != "")
	appendIf("child_session_id", matcher.ChildSessionID != "")
	appendIf("spawn_role", matcher.SpawnRole != "")
	appendIf(matcherReleaseReasonKey, matcher.ReleaseReason != "")
}

func validateMatcherPatterns(matcher HookMatcher) error {
	patterns := []struct {
		field   string
		pattern string
	}{
		{field: matcherAgentNameKey, pattern: matcher.AgentName},
		{field: "agent_type", pattern: matcher.AgentType},
		{field: matcherWorkspaceIDKey, pattern: matcher.WorkspaceID},
		{field: matcherWorkspaceRootKey, pattern: matcher.WorkspaceRoot},
		{field: "session_type", pattern: matcher.SessionType},
		{field: "sandbox_id", pattern: matcher.SandboxID},
		{field: "sandbox_backend", pattern: matcher.SandboxBackend},
		{field: "sandbox_profile", pattern: matcher.SandboxProfile},
		{field: "sync_direction", pattern: matcher.SyncDirection},
		{field: matcherInputClassKey, pattern: matcher.InputClass},
		{field: "acp_event_type", pattern: matcher.ACPEventType},
		{field: "turn_id", pattern: matcher.TurnID},
		{field: "tool_id", pattern: matcher.ToolID},
		{field: "tool_name", pattern: matcher.ToolName},
		{field: "decision_class", pattern: matcher.DecisionClass},
		{field: "message_role", pattern: matcher.MessageRole},
		{field: "message_delta_type", pattern: matcher.MessageDeltaType},
	}
	for _, item := range patterns {
		if err := validateMatcherPattern(item.field, item.pattern); err != nil {
			return err
		}
	}
	if err := validateNetworkMatcherPatterns(matcher.NetworkMatcher); err != nil {
		return err
	}
	if err := validateCompactionMatcherPatterns(matcher.CompactionMatcher); err != nil {
		return err
	}
	return validateAutonomyMatcherPatterns(matcher.Autonomy)
}

func validateCompactionMatcherPatterns(matcher *CompactionMatcher) error {
	if matcher == nil {
		return nil
	}
	patterns := []struct {
		field   string
		pattern string
	}{
		{field: "compaction_reason", pattern: matcher.Reason},
		{field: "compaction_strategy", pattern: matcher.Strategy},
	}
	for _, item := range patterns {
		if err := validateMatcherPattern(item.field, item.pattern); err != nil {
			return err
		}
	}
	return nil
}

func validateAutonomyMatcherPatterns(matcher *AutonomyMatcher) error {
	if matcher == nil {
		return nil
	}
	patterns := []struct {
		field   string
		pattern string
	}{
		{field: matcherTaskIDKey, pattern: matcher.TaskID},
		{field: matcherRunIDKey, pattern: matcher.RunID},
		{field: matcherLoopRunIDKey, pattern: matcher.LoopRunID},
		{field: matcherLoopNameKey, pattern: matcher.LoopName},
		{field: matcherNodeIDKey, pattern: matcher.NodeID},
		{field: matcherWorkflowIDKey, pattern: matcher.WorkflowID},
		{field: matcherParticipationChannelKey, pattern: matcher.ParticipationChannel},
		{field: "coordinator_session_id", pattern: matcher.CoordinatorSessionID},
		{field: "parent_session_id", pattern: matcher.ParentSessionID},
		{field: "root_session_id", pattern: matcher.RootSessionID},
		{field: "child_session_id", pattern: matcher.ChildSessionID},
		{field: "spawn_role", pattern: matcher.SpawnRole},
		{field: matcherReleaseReasonKey, pattern: matcher.ReleaseReason},
	}
	for _, item := range patterns {
		if err := validateMatcherPattern(item.field, item.pattern); err != nil {
			return err
		}
	}
	return nil
}

func validateMatcherPattern(field string, pattern string) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || !strings.ContainsAny(pattern, "*?[]") {
		return nil
	}
	if _, err := path.Match(pattern, ""); err != nil {
		return fmt.Errorf("hooks: matcher.%s pattern %q is invalid: %w", field, pattern, err)
	}
	return nil
}

func matchStringField(pattern string, value string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || pattern == "*" {
		return true
	}

	value = strings.TrimSpace(value)
	if !strings.ContainsAny(pattern, "*?[]") {
		return pattern == value
	}

	matched, err := path.Match(pattern, value)
	// Invalid patterns are treated as non-matching at runtime; validation should
	// reject them earlier during normalization.
	return err == nil && matched
}
