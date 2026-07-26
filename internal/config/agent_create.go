package config

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/goccy/go-yaml"
)

var (
	// ErrInvalidAgentDefinition marks validation failures while authoring an AGENT.md file.
	ErrInvalidAgentDefinition = errors.New("config: invalid agent definition")
	// ErrAgentDefinitionExists marks a create request that would overwrite an existing AGENT.md file.
	ErrAgentDefinitionExists = errors.New("config: agent definition already exists")
)

// AgentDefinitionDraft captures the simple AGENT.md fields supported by authoring surfaces.
type AgentDefinitionDraft struct {
	Name            string
	Provider        string
	Command         string
	Model           string
	ReasoningEffort string
	Tools           []string
	Toolsets        []string
	DenyTools       []string
	Permissions     string
	Skills          AgentSkillsConfig
	CategoryPath    []string
	MCPServers      []MCPServer
	Hooks           []hookspkg.HookDecl
	Prompt          string
}

// CreateAgentDefFile renders, validates, and persists one AGENT.md definition.
func CreateAgentDefFile(path string, draft AgentDefinitionDraft, overwrite bool) (AgentDef, error) {
	normalizedPath := strings.TrimSpace(path)
	if normalizedPath == "" {
		return AgentDef{}, fmt.Errorf("config: agent definition path is required")
	}

	contents, agent, err := RenderAgentDefinition(draft)
	if err != nil {
		return AgentDef{}, err
	}
	if err := ensureAgentDefinitionWritable(normalizedPath, overwrite); err != nil {
		return AgentDef{}, err
	}
	write := writePersistedFile
	if !overwrite {
		write = writePersistedFileExclusive
	}
	if err := write(normalizedPath, contents); err != nil {
		if !overwrite && errors.Is(err, os.ErrExist) {
			return AgentDef{}, errors.Join(
				ErrAgentDefinitionExists,
				fmt.Errorf("config: agent definition already exists at %s: %w", normalizedPath, err),
			)
		}
		return AgentDef{}, fmt.Errorf("config: write agent definition %q: %w", normalizedPath, err)
	}

	agent.SourcePath = filepath.Clean(normalizedPath)
	return agent, nil
}

// RenderAgentDefinition renders a draft to AGENT.md bytes and validates by parsing the result.
func RenderAgentDefinition(draft AgentDefinitionDraft) ([]byte, AgentDef, error) {
	agentName := NormalizeAgentName(draft.Name)
	if err := ValidateAuthoredAgentName(agentName); err != nil {
		return nil, AgentDef{}, errors.Join(ErrInvalidAgentDefinition, err)
	}
	agent := canonicalAgentDefinition(AgentDef{
		Name:            agentName,
		Provider:        strings.TrimSpace(draft.Provider),
		Command:         strings.TrimSpace(draft.Command),
		Model:           strings.TrimSpace(draft.Model),
		ReasoningEffort: strings.TrimSpace(draft.ReasoningEffort),
		Tools:           trimAgentDefinitionAtoms(draft.Tools),
		Toolsets:        trimAgentDefinitionAtoms(draft.Toolsets),
		DenyTools:       trimAgentDefinitionAtoms(draft.DenyTools),
		Permissions:     strings.TrimSpace(draft.Permissions),
		Skills:          AgentSkillsConfig{Disabled: trimAgentDefinitionAtoms(draft.Skills.Disabled)},
		CategoryPath:    trimAgentDefinitionAtoms(draft.CategoryPath),
		MCPServers:      cloneMCPServers(draft.MCPServers),
		Hooks:           cloneHookDecls(draft.Hooks),
		Prompt:          strings.TrimSpace(draft.Prompt),
	})
	agent.Skills = normalizeAgentSkillsConfig(agent.Skills)
	agent.CategoryPath = normalizeAgentCategoryPath(agent.CategoryPath)
	if err := agent.Validate(); err != nil {
		return nil, AgentDef{}, errors.Join(ErrInvalidAgentDefinition, err)
	}

	parsed := parsedAgentDef{}
	if err := applyAgentDefToParsed(&parsed, agent); err != nil {
		return nil, AgentDef{}, fmt.Errorf("config: render agent definition fields: %w", err)
	}
	frontmatter, err := yaml.Marshal(parsed)
	if err != nil {
		return nil, AgentDef{}, fmt.Errorf("config: render agent frontmatter: %w", err)
	}
	contents := renderAgentMarkdown(frontmatter, agent.Prompt)
	validated, err := ParseAgentDef(contents)
	if err != nil {
		return nil, AgentDef{}, errors.Join(
			ErrInvalidAgentDefinition,
			fmt.Errorf("config: validate generated agent definition: %w", err),
		)
	}
	if validated.Name != agentName {
		return nil, AgentDef{}, errors.Join(
			ErrInvalidAgentDefinition,
			fmt.Errorf("config: generated agent name %q does not match %q", validated.Name, agentName),
		)
	}
	return contents, validated, nil
}

// AgentDefinitionDraftFromDef preserves every AGENT.md-authored field in a mutable draft.
func AgentDefinitionDraftFromDef(agent AgentDef) AgentDefinitionDraft {
	canonical := canonicalAgentDefinition(CloneAgentDef(agent))
	return AgentDefinitionDraft{
		Name:            canonical.Name,
		Provider:        canonical.Provider,
		Command:         canonical.Command,
		Model:           canonical.Model,
		ReasoningEffort: canonical.ReasoningEffort,
		Tools:           cloneStrings(canonical.Tools),
		Toolsets:        cloneStrings(canonical.Toolsets),
		DenyTools:       cloneStrings(canonical.DenyTools),
		Permissions:     canonical.Permissions,
		Skills:          normalizeAgentSkillsConfig(canonical.Skills),
		CategoryPath:    cloneStrings(canonical.CategoryPath),
		MCPServers:      cloneMCPServers(canonical.MCPServers),
		Hooks:           cloneHookDecls(canonical.Hooks),
		Prompt:          canonical.Prompt,
	}
}

// AgentDefinitionDigest returns the canonical semantic digest used for update CAS.
func AgentDefinitionDigest(agent AgentDef) (string, error) {
	contents, _, err := RenderAgentDefinition(AgentDefinitionDraftFromDef(agent))
	if err != nil {
		return "", fmt.Errorf("config: render agent definition digest: %w", err)
	}
	digest := sha256.Sum256(contents)
	return fmt.Sprintf("%x", digest[:]), nil
}

func canonicalAgentDefinition(agent AgentDef) AgentDef {
	agent.Tools = sortedAgentDefinitionAtoms(agent.Tools)
	agent.Toolsets = sortedAgentDefinitionAtoms(agent.Toolsets)
	agent.DenyTools = sortedAgentDefinitionAtoms(agent.DenyTools)
	agent.Skills.Disabled = sortedAgentDefinitionAtoms(agent.Skills.Disabled)
	agent.MCPServers = cloneMCPServers(agent.MCPServers)
	for index := range agent.MCPServers {
		slices.Sort(agent.MCPServers[index].Auth.Scopes)
	}
	slices.SortFunc(agent.MCPServers, func(left MCPServer, right MCPServer) int {
		if byName := strings.Compare(left.Name, right.Name); byName != 0 {
			return byName
		}
		return strings.Compare(string(left.Transport), string(right.Transport))
	})
	return agent
}

func sortedAgentDefinitionAtoms(values []string) []string {
	atoms := trimAgentDefinitionAtoms(values)
	slices.Sort(atoms)
	return slices.Compact(atoms)
}

func ensureAgentDefinitionWritable(path string, overwrite bool) error {
	if overwrite {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return errors.Join(
			ErrAgentDefinitionExists,
			fmt.Errorf("config: agent definition already exists at %s", path),
		)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("config: inspect agent definition %q: %w", path, err)
	}
	return nil
}

func trimAgentDefinitionAtoms(values []string) []string {
	atoms := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		atoms = append(atoms, trimmed)
	}
	return atoms
}
