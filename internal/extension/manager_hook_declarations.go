package extensionpkg

import (
	"errors"

	"slices"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

type hookConfigExecutorFields struct {
	command   string
	args      []string
	env       map[string]string
	secretEnv map[string]string
	kind      hookspkg.HookExecutorKind
}

func resolveHookConfigExecutorFields(cfg *HookConfig) (hookConfigExecutorFields, error) {
	if cfg == nil {
		return hookConfigExecutorFields{}, errors.New("hook config is required")
	}
	fields := hookConfigExecutorFields{
		command:   strings.TrimSpace(cfg.Command),
		args:      slices.Clone(cfg.Args),
		env:       cloneStringMap(cfg.Env),
		secretEnv: cloneStringMap(cfg.SecretEnv),
		kind:      hookspkg.HookExecutorKind(strings.TrimSpace(cfg.Executor.Kind)),
	}
	rootSpecified := fields.command != "" || len(fields.args) > 0 || len(fields.env) > 0 ||
		len(fields.secretEnv) > 0
	nestedSpecified := strings.TrimSpace(cfg.Executor.Command) != "" || len(cfg.Executor.Args) > 0 ||
		len(cfg.Executor.Env) > 0 || len(cfg.Executor.SecretEnv) > 0
	if rootSpecified && nestedSpecified {
		return hookConfigExecutorFields{}, errors.New(
			"hook executor fields must be declared either at the top level or under executor, not both",
		)
	}
	if nestedSpecified {
		fields.command = strings.TrimSpace(cfg.Executor.Command)
		fields.args = slices.Clone(cfg.Executor.Args)
		fields.env = cloneStringMap(cfg.Executor.Env)
		fields.secretEnv = cloneStringMap(cfg.Executor.SecretEnv)
	}
	return fields, nil
}

func hookConfigMatcher(cfg HookMatcherConfig) hookspkg.HookMatcher {
	matcher := hookspkg.HookMatcher{
		AgentName:        strings.TrimSpace(cfg.AgentName),
		AgentType:        strings.TrimSpace(cfg.AgentType),
		WorkspaceID:      strings.TrimSpace(cfg.WorkspaceID),
		WorkspaceRoot:    strings.TrimSpace(cfg.WorkspaceRoot),
		SessionType:      strings.TrimSpace(cfg.SessionType),
		InputClass:       strings.TrimSpace(cfg.InputClass),
		ACPEventType:     strings.TrimSpace(cfg.ACPEventType),
		TurnID:           strings.TrimSpace(cfg.TurnID),
		ToolID:           strings.TrimSpace(cfg.ToolID),
		ToolName:         strings.TrimSpace(cfg.ToolName),
		DecisionClass:    strings.TrimSpace(cfg.DecisionClass),
		MessageRole:      strings.TrimSpace(cfg.MessageRole),
		MessageDeltaType: strings.TrimSpace(cfg.MessageDeltaType),
	}
	matcher.NetworkMatcher = &hookspkg.NetworkMatcher{
		Channel:   strings.TrimSpace(cfg.Channel),
		Surface:   strings.TrimSpace(cfg.Surface),
		Kind:      strings.TrimSpace(cfg.Kind),
		Direction: strings.TrimSpace(cfg.Direction),
		WorkState: strings.TrimSpace(cfg.WorkState),
	}
	matcher.CompactionMatcher = &hookspkg.CompactionMatcher{
		Reason:   strings.TrimSpace(cfg.CompactionReason),
		Strategy: strings.TrimSpace(cfg.CompactionStrategy),
	}
	if cfg.ToolReadOnly != nil {
		value := *cfg.ToolReadOnly
		matcher.ToolReadOnly = &value
	}
	return matcher
}
