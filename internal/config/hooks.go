package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
)

// HooksConfig holds config-defined hook declarations.
type HooksConfig struct {
	Declarations []hookspkg.HookDecl `toml:"declarations,omitempty"`
}

type parsedHookDeclaration struct {
	Name      string             `yaml:"name"                 toml:"name"`
	Event     string             `yaml:"event"                toml:"event"`
	Mode      string             `yaml:"mode,omitempty"       toml:"mode,omitempty"`
	Enabled   *bool              `yaml:"enabled,omitempty"    toml:"enabled,omitempty"`
	Required  bool               `yaml:"required,omitempty"   toml:"required,omitempty"`
	Priority  *int               `yaml:"priority,omitempty"   toml:"priority,omitempty"`
	Timeout   time.Duration      `yaml:"timeout,omitempty"    toml:"timeout,omitempty"`
	Matcher   parsedHookMatcher  `yaml:"matcher,omitempty"    toml:"matcher,omitempty"`
	Command   string             `yaml:"command,omitempty"    toml:"command,omitempty"`
	Args      []string           `yaml:"args,omitempty"       toml:"args,omitempty"`
	Env       map[string]string  `yaml:"env,omitempty"        toml:"env,omitempty"`
	SecretEnv map[string]string  `yaml:"secret_env,omitempty" toml:"secret_env,omitempty"`
	Metadata  map[string]string  `yaml:"metadata,omitempty"   toml:"metadata,omitempty"`
	Executor  parsedHookExecutor `yaml:"executor,omitempty"   toml:"executor,omitempty"`
}

type parsedHookExecutor struct {
	Kind      string            `yaml:"kind,omitempty"       toml:"kind,omitempty"`
	Command   string            `yaml:"command,omitempty"    toml:"command,omitempty"`
	Args      []string          `yaml:"args,omitempty"       toml:"args,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"        toml:"env,omitempty"`
	SecretEnv map[string]string `yaml:"secret_env,omitempty" toml:"secret_env,omitempty"`
}

type parsedHookMatcher struct {
	AgentName            string `yaml:"agent_name,omitempty"             toml:"agent_name,omitempty"`
	AgentType            string `yaml:"agent_type,omitempty"             toml:"agent_type,omitempty"`
	WorkspaceID          string `yaml:"workspace_id,omitempty"           toml:"workspace_id,omitempty"`
	WorkspaceRoot        string `yaml:"workspace_root,omitempty"         toml:"workspace_root,omitempty"`
	SessionType          string `yaml:"session_type,omitempty"           toml:"session_type,omitempty"`
	SandboxID            string `yaml:"sandbox_id,omitempty"             toml:"sandbox_id,omitempty"`
	SandboxBackend       string `yaml:"sandbox_backend,omitempty"        toml:"sandbox_backend,omitempty"`
	SandboxProfile       string `yaml:"sandbox_profile,omitempty"        toml:"sandbox_profile,omitempty"`
	SyncDirection        string `yaml:"sync_direction,omitempty"         toml:"sync_direction,omitempty"`
	InputClass           string `yaml:"input_class,omitempty"            toml:"input_class,omitempty"`
	ACPEventType         string `yaml:"acp_event_type,omitempty"         toml:"acp_event_type,omitempty"`
	TurnID               string `yaml:"turn_id,omitempty"                toml:"turn_id,omitempty"`
	ToolID               string `yaml:"tool_id,omitempty"                toml:"tool_id,omitempty"`
	ToolName             string `yaml:"tool_name,omitempty"              toml:"tool_name,omitempty"`
	ToolReadOnly         *bool  `yaml:"tool_read_only,omitempty"         toml:"tool_read_only,omitempty"`
	DecisionClass        string `yaml:"decision_class,omitempty"         toml:"decision_class,omitempty"`
	MessageRole          string `yaml:"message_role,omitempty"           toml:"message_role,omitempty"`
	MessageDeltaType     string `yaml:"message_delta_type,omitempty"     toml:"message_delta_type,omitempty"`
	Channel              string `yaml:"channel,omitempty"                toml:"channel,omitempty"`
	Surface              string `yaml:"surface,omitempty"                toml:"surface,omitempty"`
	Kind                 string `yaml:"kind,omitempty"                   toml:"kind,omitempty"`
	Direction            string `yaml:"direction,omitempty"              toml:"direction,omitempty"`
	WorkState            string `yaml:"work_state,omitempty"             toml:"work_state,omitempty"`
	CompactionReason     string `yaml:"compaction_reason,omitempty"      toml:"compaction_reason,omitempty"`
	CompactionStrategy   string `yaml:"compaction_strategy,omitempty"    toml:"compaction_strategy,omitempty"`
	TaskID               string `yaml:"task_id,omitempty"                toml:"task_id,omitempty"`
	RunID                string `yaml:"run_id,omitempty"                 toml:"run_id,omitempty"`
	WorkflowID           string `yaml:"workflow_id,omitempty"            toml:"workflow_id,omitempty"`
	ParticipationChannel string `yaml:"participation_channel,omitempty"  toml:"participation_channel,omitempty"`
	CoordinatorID        string `yaml:"coordinator_session_id,omitempty" toml:"coordinator_session_id,omitempty"`
	ParentSessionID      string `yaml:"parent_session_id,omitempty"      toml:"parent_session_id,omitempty"`
	RootSessionID        string `yaml:"root_session_id,omitempty"        toml:"root_session_id,omitempty"`
	ChildSessionID       string `yaml:"child_session_id,omitempty"       toml:"child_session_id,omitempty"`
	SpawnRole            string `yaml:"spawn_role,omitempty"             toml:"spawn_role,omitempty"`
	ReleaseReason        string `yaml:"release_reason,omitempty"         toml:"release_reason,omitempty"`
}

type hookValidationExecutor struct {
	kind hookspkg.HookExecutorKind
}

var _ hookspkg.Executor = hookValidationExecutor{}

func (e hookValidationExecutor) Kind() hookspkg.HookExecutorKind {
	return e.kind
}

func (hookValidationExecutor) Execute(context.Context, hookspkg.RegisteredHook, []byte) ([]byte, error) {
	return nil, errors.New("config: validation executor cannot execute")
}

// HookDeclarations returns normalized config and agent-definition hook declarations for registry consumption.
func HookDeclarations(hooksCfg HooksConfig, agents []AgentDef) ([]hookspkg.HookDecl, error) {
	capacity := len(hooksCfg.Declarations)
	for _, agent := range agents {
		capacity += len(agent.Hooks)
	}
	if capacity == 0 {
		return []hookspkg.HookDecl{}, nil
	}

	normalized := make([]hookspkg.HookDecl, 0, capacity)
	idx := 0
	for _, decl := range hooksCfg.Declarations {
		var err error
		normalized, err = appendNormalizedHookDecl(normalized, idx, decl)
		if err != nil {
			return nil, err
		}
		idx++
	}
	for _, agent := range agents {
		for _, decl := range agent.Hooks {
			var err error
			normalized, err = appendNormalizedHookDecl(normalized, idx, decl)
			if err != nil {
				return nil, err
			}
			idx++
		}
	}

	return normalized, nil
}

func appendNormalizedHookDecl(dst []hookspkg.HookDecl, idx int, decl hookspkg.HookDecl) ([]hookspkg.HookDecl, error) {
	if !decl.EnabledValue() {
		return dst, nil
	}
	resolved, err := hookspkg.NormalizeHookDecl(decl, hookDeclarationResolver)
	if err != nil {
		return nil, fmt.Errorf(
			"config: normalize hook declaration %d (%q): %w",
			idx,
			strings.TrimSpace(decl.Name),
			err,
		)
	}
	return append(dst, resolved.Decl), nil
}

// Validate ensures the hook declarations are internally consistent.
func (c HooksConfig) Validate() error {
	if len(c.Declarations) == 0 {
		return nil
	}
	if err := hookspkg.ValidateHookDecls(c.Declarations); err != nil {
		return fmt.Errorf("hooks.declarations: %w", err)
	}
	return nil
}

func (d *parsedHookDeclaration) toHookDecl(
	source hookspkg.HookSource,
	scopeAgentName string,
) (hookspkg.HookDecl, error) {
	command, args, env, secretEnv, kind, err := d.resolveExecutor()
	if err != nil {
		return hookspkg.HookDecl{}, err
	}

	matcher, err := d.Matcher.toHookMatcher(scopeAgentName)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}

	decl := hookspkg.HookDecl{
		Name:         strings.TrimSpace(d.Name),
		Event:        hookspkg.HookEvent(strings.TrimSpace(d.Event)),
		Source:       source,
		Mode:         hookspkg.HookMode(strings.TrimSpace(d.Mode)),
		Enabled:      cloneBoolPtr(d.Enabled),
		Required:     d.Required,
		Timeout:      d.Timeout,
		Matcher:      matcher,
		ExecutorKind: kind,
		Command:      command,
		Args:         args,
		Env:          env,
		SecretEnv:    secretEnv,
		Metadata:     mergeStringMaps(nil, d.Metadata),
	}
	if d.Priority != nil {
		priority, err := hookspkg.PriorityFromInt(*d.Priority)
		if err != nil {
			return hookspkg.HookDecl{}, err
		}
		decl.Priority = priority
		decl.PrioritySet = true
	}

	return decl, nil
}

func (d *parsedHookDeclaration) resolveExecutor() (
	string,
	[]string,
	map[string]string,
	map[string]string,
	hookspkg.HookExecutorKind,
	error,
) {
	rootSpecified := strings.TrimSpace(d.Command) != "" || len(d.Args) > 0 || len(d.Env) > 0 ||
		len(d.SecretEnv) > 0
	nestedSpecified := strings.TrimSpace(d.Executor.Command) != "" || len(d.Executor.Args) > 0 ||
		len(d.Executor.Env) > 0 || len(d.Executor.SecretEnv) > 0
	if rootSpecified && nestedSpecified {
		return "", nil, nil, nil, "", errors.New(
			"hook executor fields must be declared either at the top level or under executor, not both",
		)
	}

	command := strings.TrimSpace(d.Command)
	args := cloneStrings(d.Args)
	env := mergeStringMaps(nil, d.Env)
	secretEnv := mergeStringMaps(nil, d.SecretEnv)
	if nestedSpecified {
		command = strings.TrimSpace(d.Executor.Command)
		args = cloneStrings(d.Executor.Args)
		env = mergeStringMaps(nil, d.Executor.Env)
		secretEnv = mergeStringMaps(nil, d.Executor.SecretEnv)
	}

	return command, args, env, secretEnv, hookspkg.HookExecutorKind(strings.TrimSpace(d.Executor.Kind)), nil
}

func (m *parsedHookMatcher) toHookMatcher(scopeAgentName string) (hookspkg.HookMatcher, error) {
	matcher := hookspkg.HookMatcher{
		AgentName:        strings.TrimSpace(m.AgentName),
		AgentType:        strings.TrimSpace(m.AgentType),
		WorkspaceID:      strings.TrimSpace(m.WorkspaceID),
		WorkspaceRoot:    strings.TrimSpace(m.WorkspaceRoot),
		SessionType:      strings.TrimSpace(m.SessionType),
		SandboxID:        strings.TrimSpace(m.SandboxID),
		SandboxBackend:   strings.TrimSpace(m.SandboxBackend),
		SandboxProfile:   strings.TrimSpace(m.SandboxProfile),
		SyncDirection:    strings.TrimSpace(m.SyncDirection),
		InputClass:       strings.TrimSpace(m.InputClass),
		ACPEventType:     strings.TrimSpace(m.ACPEventType),
		TurnID:           strings.TrimSpace(m.TurnID),
		ToolID:           strings.TrimSpace(m.ToolID),
		ToolName:         strings.TrimSpace(m.ToolName),
		DecisionClass:    strings.TrimSpace(m.DecisionClass),
		MessageRole:      strings.TrimSpace(m.MessageRole),
		MessageDeltaType: strings.TrimSpace(m.MessageDeltaType),
	}
	matcher.NetworkMatcher = &hookspkg.NetworkMatcher{
		Channel:   strings.TrimSpace(m.Channel),
		Surface:   strings.TrimSpace(m.Surface),
		Kind:      strings.TrimSpace(m.Kind),
		Direction: strings.TrimSpace(m.Direction),
		WorkState: strings.TrimSpace(m.WorkState),
	}
	matcher.CompactionMatcher = &hookspkg.CompactionMatcher{
		Reason:   strings.TrimSpace(m.CompactionReason),
		Strategy: strings.TrimSpace(m.CompactionStrategy),
	}
	autonomy := hookspkg.AutonomyMatcher{
		TaskID:               strings.TrimSpace(m.TaskID),
		RunID:                strings.TrimSpace(m.RunID),
		WorkflowID:           strings.TrimSpace(m.WorkflowID),
		ParticipationChannel: strings.TrimSpace(m.ParticipationChannel),
		CoordinatorSessionID: strings.TrimSpace(m.CoordinatorID),
		ParentSessionID:      strings.TrimSpace(m.ParentSessionID),
		RootSessionID:        strings.TrimSpace(m.RootSessionID),
		ChildSessionID:       strings.TrimSpace(m.ChildSessionID),
		SpawnRole:            strings.TrimSpace(m.SpawnRole),
		ReleaseReason:        strings.TrimSpace(m.ReleaseReason),
	}
	if !emptyParsedAutonomyMatcher(autonomy) {
		matcher.Autonomy = &autonomy
	}
	if m.ToolReadOnly != nil {
		value := *m.ToolReadOnly
		matcher.ToolReadOnly = &value
	}

	if scopeAgentName == "" {
		return matcher, nil
	}
	if matcher.AgentName != "" && matcher.AgentName != scopeAgentName {
		return hookspkg.HookMatcher{}, fmt.Errorf("matcher.agent_name must match agent name %q", scopeAgentName)
	}
	matcher.AgentName = scopeAgentName
	return matcher, nil
}

func parsedHookDeclarationsFromHookDecls(
	decls []hookspkg.HookDecl,
	scopeAgentName string,
) ([]parsedHookDeclaration, error) {
	if len(decls) == 0 {
		return nil, nil
	}

	parsed := make([]parsedHookDeclaration, 0, len(decls))
	for idx, decl := range decls {
		rendered, err := parsedHookDeclarationFromHookDecl(decl, scopeAgentName)
		if err != nil {
			return nil, fmt.Errorf("agent.hooks[%d]: %w", idx, err)
		}
		parsed = append(parsed, rendered)
	}
	return parsed, nil
}

func parsedHookDeclarationFromHookDecl(
	decl hookspkg.HookDecl,
	scopeAgentName string,
) (parsedHookDeclaration, error) {
	if strings.TrimSpace(decl.WorkingDir) != "" {
		return parsedHookDeclaration{}, errors.New("working_dir cannot be stored in AGENT.md hooks")
	}
	if decl.SkillSource != "" {
		return parsedHookDeclaration{}, errors.New("skill_source cannot be stored in AGENT.md hooks")
	}

	matcher, err := parsedHookMatcherFromHookMatcher(decl.Matcher, scopeAgentName)
	if err != nil {
		return parsedHookDeclaration{}, err
	}

	var priority *int
	if decl.PrioritySet || decl.Priority != 0 {
		value := int(decl.Priority)
		priority = &value
	}

	executor := parsedHookExecutor{}
	if decl.ExecutorKind != "" {
		executor.Kind = string(decl.ExecutorKind)
	}

	return parsedHookDeclaration{
		Name:      strings.TrimSpace(decl.Name),
		Event:     strings.TrimSpace(string(decl.Event)),
		Mode:      strings.TrimSpace(string(decl.Mode)),
		Enabled:   cloneBoolPtr(decl.Enabled),
		Required:  decl.Required,
		Priority:  priority,
		Timeout:   decl.Timeout,
		Matcher:   matcher,
		Command:   strings.TrimSpace(decl.Command),
		Args:      cloneStrings(decl.Args),
		Env:       mergeStringMaps(nil, decl.Env),
		SecretEnv: mergeStringMaps(nil, decl.SecretEnv),
		Metadata:  mergeStringMaps(nil, decl.Metadata),
		Executor:  executor,
	}, nil
}

func parsedHookMatcherFromHookMatcher(
	matcher hookspkg.HookMatcher,
	scopeAgentName string,
) (parsedHookMatcher, error) {
	parsed := parsedHookMatcher{
		AgentName:        strings.TrimSpace(matcher.AgentName),
		AgentType:        strings.TrimSpace(matcher.AgentType),
		WorkspaceID:      strings.TrimSpace(matcher.WorkspaceID),
		WorkspaceRoot:    strings.TrimSpace(matcher.WorkspaceRoot),
		SessionType:      strings.TrimSpace(matcher.SessionType),
		SandboxID:        strings.TrimSpace(matcher.SandboxID),
		SandboxBackend:   strings.TrimSpace(matcher.SandboxBackend),
		SandboxProfile:   strings.TrimSpace(matcher.SandboxProfile),
		SyncDirection:    strings.TrimSpace(matcher.SyncDirection),
		InputClass:       strings.TrimSpace(matcher.InputClass),
		ACPEventType:     strings.TrimSpace(matcher.ACPEventType),
		TurnID:           strings.TrimSpace(matcher.TurnID),
		ToolID:           strings.TrimSpace(matcher.ToolID),
		ToolName:         strings.TrimSpace(matcher.ToolName),
		ToolReadOnly:     cloneBoolPtr(matcher.ToolReadOnly),
		DecisionClass:    strings.TrimSpace(matcher.DecisionClass),
		MessageRole:      strings.TrimSpace(matcher.MessageRole),
		MessageDeltaType: strings.TrimSpace(matcher.MessageDeltaType),
	}
	if matcher.NetworkMatcher != nil {
		parsed.Channel = strings.TrimSpace(matcher.Channel)
		parsed.Surface = strings.TrimSpace(matcher.Surface)
		parsed.Kind = strings.TrimSpace(matcher.Kind)
		parsed.Direction = strings.TrimSpace(matcher.Direction)
		parsed.WorkState = strings.TrimSpace(matcher.WorkState)
	}
	if matcher.CompactionMatcher != nil {
		parsed.CompactionReason = strings.TrimSpace(matcher.Reason)
		parsed.CompactionStrategy = strings.TrimSpace(matcher.Strategy)
	}
	if matcher.Autonomy != nil {
		parsed.TaskID = strings.TrimSpace(matcher.Autonomy.TaskID)
		parsed.RunID = strings.TrimSpace(matcher.Autonomy.RunID)
		parsed.WorkflowID = strings.TrimSpace(matcher.Autonomy.WorkflowID)
		parsed.ParticipationChannel = strings.TrimSpace(matcher.Autonomy.ParticipationChannel)
		parsed.CoordinatorID = strings.TrimSpace(matcher.Autonomy.CoordinatorSessionID)
		parsed.ParentSessionID = strings.TrimSpace(matcher.Autonomy.ParentSessionID)
		parsed.RootSessionID = strings.TrimSpace(matcher.Autonomy.RootSessionID)
		parsed.ChildSessionID = strings.TrimSpace(matcher.Autonomy.ChildSessionID)
		parsed.SpawnRole = strings.TrimSpace(matcher.Autonomy.SpawnRole)
		parsed.ReleaseReason = strings.TrimSpace(matcher.Autonomy.ReleaseReason)
	}

	if scopeAgentName == "" {
		return parsed, nil
	}
	if parsed.AgentName != "" && parsed.AgentName != scopeAgentName {
		return parsedHookMatcher{}, fmt.Errorf("matcher.agent_name must match agent name %q", scopeAgentName)
	}
	parsed.AgentName = ""
	return parsed, nil
}

func emptyParsedAutonomyMatcher(matcher hookspkg.AutonomyMatcher) bool {
	return matcher.TaskID == "" &&
		matcher.RunID == "" &&
		matcher.WorkflowID == "" &&
		matcher.ParticipationChannel == "" &&
		matcher.CoordinatorSessionID == "" &&
		matcher.ParentSessionID == "" &&
		matcher.RootSessionID == "" &&
		matcher.ChildSessionID == "" &&
		matcher.SpawnRole == "" &&
		matcher.ReleaseReason == ""
}

func hookDeclarationResolver(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
	return hookValidationExecutor{kind: decl.ExecutorKind}, nil
}

func cloneHookDecls(src []hookspkg.HookDecl) []hookspkg.HookDecl {
	if len(src) == 0 {
		return nil
	}

	cloned := make([]hookspkg.HookDecl, 0, len(src))
	for _, decl := range src {
		cloned = append(cloned, cloneHookDecl(decl))
	}

	return cloned
}

func cloneHookDecl(src hookspkg.HookDecl) hookspkg.HookDecl {
	cloned := src
	cloned.Args = cloneStrings(src.Args)
	cloned.Env = mergeStringMaps(nil, src.Env)
	cloned.SecretEnv = mergeStringMaps(nil, src.SecretEnv)
	cloned.Metadata = mergeStringMaps(nil, src.Metadata)
	cloned.Enabled = cloneBoolPtr(src.Enabled)
	cloned.Matcher = cloneHookMatcher(src.Matcher)
	return cloned
}

func cloneHookMatcher(src hookspkg.HookMatcher) hookspkg.HookMatcher {
	cloned := src
	if src.NetworkMatcher != nil {
		value := *src.NetworkMatcher
		cloned.NetworkMatcher = &value
	}
	if src.CompactionMatcher != nil {
		value := *src.CompactionMatcher
		cloned.CompactionMatcher = &value
	}
	if src.Autonomy != nil {
		value := *src.Autonomy
		cloned.Autonomy = &value
	}
	if src.ToolReadOnly != nil {
		value := *src.ToolReadOnly
		cloned.ToolReadOnly = &value
	}
	return cloned
}

func cloneBoolPtr(src *bool) *bool {
	if src == nil {
		return nil
	}
	value := *src
	return &value
}
