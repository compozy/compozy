package skills

import (
	"bytes"

	"fmt"

	"slices"
	"strings"
	"time"

	"github.com/compozy/agh/internal/frontmatter"
	hookspkg "github.com/compozy/agh/internal/hooks"
	yaml "gopkg.in/yaml.v3"
)

func parseSkillContent(content []byte) (SkillMeta, string, error) {
	parts, err := frontmatter.Split(content)
	if err != nil {
		return SkillMeta{}, "", err
	}

	meta, err := decodeSkillMeta(string(parts.Metadata))
	if err != nil {
		return SkillMeta{}, "", fmt.Errorf("decode YAML frontmatter: %w", err)
	}

	return meta, parts.Body, nil
}

func parseMCPServerDecls(skill *Skill, raw any) []MCPServerDecl {
	items, ok := raw.([]any)
	if !ok {
		warnAGHMetadata(skill, "skills: malformed metadata.agh.mcp_servers field", "type", fmt.Sprintf("%T", raw))
		return nil
	}

	servers := make([]MCPServerDecl, 0, len(items))
	for idx, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			warnAGHMetadata(
				skill,
				"skills: malformed metadata.agh.mcp_servers entry",
				"index",
				idx,
				"type",
				fmt.Sprintf("%T", item),
			)
			continue
		}

		server := MCPServerDecl{
			Name:      strings.TrimSpace(stringValue(entry["name"])),
			Command:   strings.TrimSpace(stringValue(entry["command"])),
			Args:      stringSliceValue(skill, "metadata.agh.mcp_servers", idx, "args", entry["args"]),
			Env:       stringMapValue(skill, "metadata.agh.mcp_servers", idx, "env", entry["env"]),
			SecretEnv: stringMapValue(skill, "metadata.agh.mcp_servers", idx, "secret_env", entry["secret_env"]),
		}
		if server.Name == "" {
			warnAGHMetadata(
				skill,
				"skills: invalid metadata.agh.mcp_servers entry",
				"index",
				idx,
				"reason",
				"missing name",
			)
			continue
		}
		if server.Command == "" {
			warnAGHMetadata(
				skill,
				"skills: invalid metadata.agh.mcp_servers entry",
				"index",
				idx,
				"reason",
				"missing command",
			)
			continue
		}

		servers = append(servers, server)
	}

	if len(servers) == 0 {
		return nil
	}

	return slices.Clip(servers)
}

type parsedSkillHookDecl struct {
	Event     string               `yaml:"event"`
	Command   string               `yaml:"command"`
	Args      []string             `yaml:"args,omitempty"`
	Timeout   time.Duration        `yaml:"timeout,omitempty"`
	Env       map[string]string    `yaml:"env,omitempty"`
	SecretEnv map[string]string    `yaml:"secret_env,omitempty"`
	Mode      hookspkg.HookMode    `yaml:"mode,omitempty"`
	Priority  *int                 `yaml:"priority,omitempty"`
	Matcher   hookspkg.HookMatcher `yaml:"matcher,omitempty"`
}

func parseHookDecls(skill *Skill, raw any) ([]hookspkg.HookDecl, error) {
	items, ok := raw.([]any)
	if !ok {
		warnAGHMetadata(skill, "skills: malformed metadata.agh.hooks field", "type", fmt.Sprintf("%T", raw))
		return nil, nil
	}

	hooks := make([]hookspkg.HookDecl, 0, len(items))
	for idx, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			warnAGHMetadata(
				skill,
				"skills: malformed metadata.agh.hooks entry",
				"index",
				idx,
				"type",
				fmt.Sprintf("%T", item),
			)
			continue
		}

		decoded, err := decodeSkillHookDecl(entry)
		if err != nil {
			return nil, fmt.Errorf(
				"skills: invalid metadata.agh.hooks entry for %q at index %d: %w",
				skillIdentifier(skill),
				idx,
				err,
			)
		}

		hook, err := buildSkillHookDecl(skill, decoded, idx, len(items))
		if err != nil {
			return nil, err
		}
		if err := hookspkg.ValidateHookDecl(hook); err != nil {
			return nil, fmt.Errorf(
				"skills: invalid metadata.agh.hooks entry for %q at index %d: %w",
				skillIdentifier(skill),
				idx,
				err,
			)
		}

		hooks = append(hooks, hook)
	}

	if len(hooks) == 0 {
		return nil, nil
	}

	return slices.Clip(hooks), nil
}

func buildSkillHookDecl(
	skill *Skill,
	decoded parsedSkillHookDecl,
	index int,
	total int,
) (hookspkg.HookDecl, error) {
	event, err := skillHookEvent(skill, strings.TrimSpace(decoded.Event), index)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}

	mode := decoded.Mode
	if mode == "" {
		mode = hookspkg.HookModeAsync
	}

	hook := normalizeSkillHookDecl(skill, hookspkg.HookDecl{
		Name:        skillHookName(skill, index, total),
		Event:       event,
		Mode:        mode,
		Priority:    0,
		Timeout:     decoded.Timeout,
		Matcher:     decoded.Matcher,
		Command:     strings.TrimSpace(decoded.Command),
		Args:        append([]string(nil), decoded.Args...),
		Env:         cloneStringMap(decoded.Env),
		SecretEnv:   cloneStringMap(decoded.SecretEnv),
		PrioritySet: decoded.Priority != nil,
	}, index, total)
	if decoded.Priority != nil {
		priority, err := hookspkg.PriorityFromInt(*decoded.Priority)
		if err != nil {
			return hookspkg.HookDecl{}, err
		}
		hook.Priority = priority
	}
	if hook.Command == "" {
		return hookspkg.HookDecl{}, fmt.Errorf(
			"skills: invalid metadata.agh.hooks entry for %q at index %d: command is required",
			skillIdentifier(skill),
			index,
		)
	}

	return hook, nil
}

func skillHookEvent(skill *Skill, rawEvent string, index int) (hookspkg.HookEvent, error) {
	event := hookspkg.HookEvent(rawEvent)
	if replacement, ok := legacyHookEventReplacement(rawEvent); ok {
		return "", fmt.Errorf(
			"skills: invalid metadata.agh.hooks entry for %q at index %d: hook event %q was removed; use %q",
			skillIdentifier(skill),
			index,
			rawEvent,
			replacement,
		)
	}
	if !validHookEvent(event) {
		return "", fmt.Errorf(
			"skills: invalid metadata.agh.hooks entry for %q at index %d: unknown hook event %q",
			skillIdentifier(skill),
			index,
			rawEvent,
		)
	}
	return event, nil
}

func decodeSkillHookDecl(entry map[string]any) (parsedSkillHookDecl, error) {
	var decoded parsedSkillHookDecl

	payload, err := yaml.Marshal(entry)
	if err != nil {
		return parsedSkillHookDecl{}, fmt.Errorf("encode hook declaration: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(payload))
	decoder.KnownFields(true)
	if err := decoder.Decode(&decoded); err != nil {
		return parsedSkillHookDecl{}, fmt.Errorf("decode hook declaration: %w", err)
	}

	return decoded, nil
}

func validHookEvent(event hookspkg.HookEvent) bool {
	return event.Validate() == nil
}

func legacyHookEventReplacement(event string) (hookspkg.HookEvent, bool) {
	switch strings.TrimSpace(event) {
	case "on_session_created":
		return hookspkg.HookSessionPostCreate, true
	case "on_session_stopped":
		return hookspkg.HookSessionPostStop, true
	default:
		return "", false
	}
}
