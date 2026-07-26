package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/compozy/agh/internal/frontmatter"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/goccy/go-yaml"
)

// AgentDef is the parsed representation of an AGENT.md file.
type AgentDef struct {
	Name            string              `json:"name"                       yaml:"name"                       toml:"name"`
	Provider        string              `json:"provider,omitempty"         yaml:"provider"                   toml:"provider"`
	Command         string              `json:"command,omitempty"          yaml:"command,omitempty"          toml:"command,omitempty"`
	Model           string              `json:"model,omitempty"            yaml:"model,omitempty"            toml:"model,omitempty"`
	ReasoningEffort string              `json:"reasoning_effort,omitempty" yaml:"reasoning_effort,omitempty" toml:"reasoning_effort,omitempty"`
	Tools           []string            `json:"tools,omitempty"            yaml:"tools,omitempty"            toml:"tools,omitempty"`
	Toolsets        []string            `json:"toolsets,omitempty"         yaml:"toolsets,omitempty"         toml:"toolsets,omitempty"`
	DenyTools       []string            `json:"deny_tools,omitempty"       yaml:"deny_tools,omitempty"       toml:"deny_tools,omitempty"`
	Permissions     string              `json:"permissions,omitempty"      yaml:"permissions,omitempty"      toml:"permissions,omitempty"`
	Skills          AgentSkillsConfig   `json:"skills,omitzero"            yaml:"skills,omitempty"           toml:"skills,omitempty"`
	CategoryPath    []string            `json:"category_path,omitempty"    yaml:"category_path,omitempty"    toml:"category_path,omitempty"`
	MCPServers      []MCPServer         `json:"mcp_servers,omitempty"      yaml:"mcp_servers,omitempty"      toml:"mcp_servers,omitempty"`
	Hooks           []hookspkg.HookDecl `json:"hooks,omitempty"            yaml:"hooks,omitempty"            toml:"hooks,omitempty"`
	Capabilities    *CapabilityCatalog  `json:"capabilities,omitempty"     yaml:"-"                          toml:"-"`
	Prompt          string              `json:"prompt,omitempty"           yaml:"-"`
	SourcePath      string              `json:"-"                          yaml:"-"                          toml:"-"`
}

type parsedAgentDef struct {
	Name            string                  `yaml:"name"                       toml:"name"`
	Provider        string                  `yaml:"provider"                   toml:"provider"`
	Command         string                  `yaml:"command,omitempty"          toml:"command,omitempty"`
	Model           string                  `yaml:"model,omitempty"            toml:"model,omitempty"`
	ReasoningEffort string                  `yaml:"reasoning_effort,omitempty" toml:"reasoning_effort,omitempty"`
	Tools           []string                `yaml:"tools,omitempty"            toml:"tools,omitempty"`
	Toolsets        []string                `yaml:"toolsets,omitempty"         toml:"toolsets,omitempty"`
	DenyTools       []string                `yaml:"deny_tools,omitempty"       toml:"deny_tools,omitempty"`
	Permissions     string                  `yaml:"permissions,omitempty"      toml:"permissions,omitempty"`
	Skills          AgentSkillsConfig       `yaml:"skills,omitempty"           toml:"skills,omitempty"`
	CategoryPath    []string                `yaml:"category_path,omitempty"    toml:"category_path,omitempty"`
	MCPServers      []MCPServer             `yaml:"mcp_servers,omitempty"      toml:"mcp_servers,omitempty"`
	Hooks           []parsedHookDeclaration `yaml:"hooks,omitempty"            toml:"hooks,omitempty"`
}

// WorkspaceDiscoverySource identifies where a discovery root came from.
type WorkspaceDiscoverySource string

const (
	// WorkspaceDiscoverySourceWorkspace marks the primary workspace root.
	WorkspaceDiscoverySourceWorkspace WorkspaceDiscoverySource = "workspace"
	// WorkspaceDiscoverySourceAdditional marks an additional workspace root.
	WorkspaceDiscoverySourceAdditional WorkspaceDiscoverySource = "additional"
	// WorkspaceDiscoverySourceGlobal marks the global AGH home root.
	WorkspaceDiscoverySourceGlobal WorkspaceDiscoverySource = "global"
)

// WorkspaceDiscoveryRoot describes a filesystem root participating in multi-root resource discovery.
type WorkspaceDiscoveryRoot struct {
	Dir    string
	Source WorkspaceDiscoverySource
}

var (
	// ErrMissingAgentFrontmatter reports a missing YAML frontmatter block in AGENT.md content.
	ErrMissingAgentFrontmatter = errors.New("config: missing YAML frontmatter")
	// ErrUnterminatedAgentFrontmatter reports an unterminated YAML frontmatter block in AGENT.md content.
	ErrUnterminatedAgentFrontmatter = errors.New("config: unterminated YAML frontmatter")
	// ErrBOMAgentFrontmatter reports a UTF-8 BOM before the YAML frontmatter block.
	ErrBOMAgentFrontmatter = errors.New("config: UTF-8 BOM before YAML frontmatter")
	// ErrInvalidAgentFrontmatterKey reports an unsupported frontmatter key shape.
	ErrInvalidAgentFrontmatterKey = errors.New("config: invalid YAML frontmatter key")
)

// LoadAgentDef loads an AGENT.md file from the configured AGH home directory.
func LoadAgentDef(name string, homePaths HomePaths) (AgentDef, error) {
	target := NormalizeAgentName(name)
	if target == "" {
		return AgentDef{}, errors.New("agent name is required")
	}
	if err := ValidateAgentName(target); err != nil {
		return AgentDef{}, err
	}

	path := filepath.Join(homePaths.AgentsDir, target, agentDefName)
	agent, err := LoadAgentDefFile(path)
	if err != nil {
		return AgentDef{}, err
	}
	if agent.Name != target {
		return AgentDef{}, fmt.Errorf("agent file %q defines name %q, expected %q", path, agent.Name, target)
	}

	return agent, nil
}

// LoadAgentDefFile loads and parses an AGENT.md file from an explicit path.
func LoadAgentDefFile(path string) (AgentDef, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return AgentDef{}, fmt.Errorf("read agent file %q: %w", path, err)
	}

	agent, err := ParseAgentDef(contents)
	if err != nil {
		return AgentDef{}, fmt.Errorf("parse agent file %q: %w", path, err)
	}
	if err := mergeAgentMCPSidecar(filepath.Dir(path), &agent); err != nil {
		return AgentDef{}, fmt.Errorf("load agent file %q MCP JSON: %w", path, err)
	}
	capabilities, err := LoadAgentCapabilities(filepath.Dir(path))
	if err != nil {
		return AgentDef{}, fmt.Errorf("load agent file %q capability catalog: %w", path, err)
	}
	agent.Capabilities = capabilities
	if err := agent.Validate(); err != nil {
		return AgentDef{}, fmt.Errorf("validate agent file %q: %w", path, err)
	}
	agent.SourcePath = filepath.Clean(path)

	return agent, nil
}

// WorkspaceDiscoveryRoots returns ordered discovery roots for workspace-scoped resources.
// Precedence is left to right: workspace root, additional roots, then the global AGH home.
func WorkspaceDiscoveryRoots(rootDir string, additionalDirs []string, homePaths HomePaths) []WorkspaceDiscoveryRoot {
	roots := make([]WorkspaceDiscoveryRoot, 0, len(additionalDirs)+2)

	if trimmed := strings.TrimSpace(rootDir); trimmed != "" {
		roots = append(roots, WorkspaceDiscoveryRoot{
			Dir:    trimmed,
			Source: WorkspaceDiscoverySourceWorkspace,
		})
	}

	for _, dir := range additionalDirs {
		if trimmed := strings.TrimSpace(dir); trimmed != "" {
			roots = append(roots, WorkspaceDiscoveryRoot{
				Dir:    trimmed,
				Source: WorkspaceDiscoverySourceAdditional,
			})
		}
	}

	if trimmed := strings.TrimSpace(homePaths.HomeDir); trimmed != "" {
		roots = append(roots, WorkspaceDiscoveryRoot{
			Dir:    trimmed,
			Source: WorkspaceDiscoverySourceGlobal,
		})
	}

	return roots
}

// AgentsDir returns the agent-definition directory for this discovery root.
func (r WorkspaceDiscoveryRoot) AgentsDir() string {
	if r.Source == WorkspaceDiscoverySourceGlobal {
		return filepath.Join(r.Dir, AgentsDirName)
	}

	return filepath.Join(r.Dir, DirName, AgentsDirName)
}

// SkillsDir returns the skill-definition directory for this discovery root.
func (r WorkspaceDiscoveryRoot) SkillsDir() string {
	if r.Source == WorkspaceDiscoverySourceGlobal {
		return filepath.Join(r.Dir, SkillsDirName)
	}

	return filepath.Join(r.Dir, DirName, SkillsDirName)
}

// LoopsDir returns the loop-definition directory for this discovery root.
func (r WorkspaceDiscoveryRoot) LoopsDir() string {
	if r.Source == WorkspaceDiscoverySourceGlobal {
		return filepath.Join(r.Dir, LoopsDirName)
	}

	return filepath.Join(r.Dir, DirName, LoopsDirName)
}

// LoadWorkspaceAgentDefs loads workspace-visible agents using root, additional, then global precedence.
func LoadWorkspaceAgentDefs(rootDir string, additionalDirs []string, homePaths HomePaths) ([]AgentDef, error) {
	roots := WorkspaceDiscoveryRoots(rootDir, additionalDirs, homePaths)
	if len(roots) == 0 {
		return nil, nil
	}

	agents := make([]AgentDef, 0)
	seen := make(map[string]struct{})

	for _, root := range roots {
		entries, err := os.ReadDir(root.AgentsDir())
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read agents directory %q: %w", root.AgentsDir(), err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if IsReservedAgentName(entry.Name()) {
				continue
			}

			agentPath := filepath.Join(root.AgentsDir(), entry.Name(), agentDefName)
			agent, err := LoadAgentDefFile(agentPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return nil, err
			}
			target := NormalizeAgentName(entry.Name())
			if agent.Name != target {
				return nil, fmt.Errorf("agent file %q defines name %q, expected %q", agentPath, agent.Name, target)
			}

			if _, ok := seen[agent.Name]; ok {
				continue
			}

			seen[agent.Name] = struct{}{}
			agents = append(agents, agent)
		}
	}

	return agents, nil
}

// ParseAgentDef parses a Markdown file with YAML frontmatter into an AgentDef.
func ParseAgentDef(content []byte) (AgentDef, error) {
	var parsed parsedAgentDef

	body, err := frontmatter.Decode(content, func(data []byte) error {
		return decodeAgentFrontmatter(data, &parsed)
	})
	if err != nil {
		return AgentDef{}, wrapFrontmatterError(err)
	}

	agent := AgentDef{
		Name:            strings.TrimSpace(parsed.Name),
		Provider:        strings.TrimSpace(parsed.Provider),
		Command:         strings.TrimSpace(parsed.Command),
		Model:           strings.TrimSpace(parsed.Model),
		ReasoningEffort: strings.TrimSpace(parsed.ReasoningEffort),
		Tools:           normalizeAgentToolPatterns(parsed.Tools),
		Toolsets:        normalizeAgentToolsetRefs(parsed.Toolsets),
		DenyTools:       normalizeAgentToolPatterns(parsed.DenyTools),
		Permissions:     strings.TrimSpace(parsed.Permissions),
		Skills:          normalizeAgentSkillsConfig(parsed.Skills),
		CategoryPath:    normalizeAgentCategoryPath(parsed.CategoryPath),
		MCPServers:      cloneMCPServers(parsed.MCPServers),
		Prompt:          strings.TrimSpace(body),
	}
	if len(parsed.Hooks) > 0 {
		agent.Hooks = make([]hookspkg.HookDecl, 0, len(parsed.Hooks))
		for idx := range parsed.Hooks {
			raw := &parsed.Hooks[idx]
			decl, err := raw.toHookDecl(hookspkg.HookSourceAgentDefinition, agent.Name)
			if err != nil {
				return AgentDef{}, fmt.Errorf("agent.hooks[%d]: %w", idx, err)
			}
			agent.Hooks = append(agent.Hooks, decl)
		}
	}

	if err := agent.Validate(); err != nil {
		return AgentDef{}, err
	}

	return agent, nil
}

// normalizeAgentCategoryPath trims each segment, preserving casing and order.
func normalizeAgentCategoryPath(path []string) []string {
	if len(path) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(path))
	for _, segment := range path {
		normalized = append(normalized, strings.TrimSpace(segment))
	}
	return normalized
}

func validateAgentCategoryPath(path []string) error {
	for idx, segment := range path {
		switch {
		case segment == "":
			return fmt.Errorf("agent.category_path[%d]: blank segment", idx)
		case segment == "." || segment == "..":
			return fmt.Errorf("agent.category_path[%d]: %q is not a valid segment", idx, segment)
		case strings.ContainsAny(segment, `/\`):
			return fmt.Errorf("agent.category_path[%d]: %q must not contain '/' or '\\'", idx, segment)
		}
	}
	return nil
}

// NormalizeAgentName returns the canonical in-memory agent identity.
func NormalizeAgentName(name string) string {
	return strings.TrimSpace(name)
}

// ValidateAgentName rejects names that could reshape the canonical agent path.
func ValidateAgentName(name string) error {
	trimmed := NormalizeAgentName(name)
	switch {
	case trimmed == "":
		return errors.New("agent name is required")
	case trimmed == "." || trimmed == "..":
		return fmt.Errorf("agent name %q is invalid", trimmed)
	case strings.Contains(trimmed, "/"), strings.Contains(trimmed, `\`):
		return fmt.Errorf("agent name %q is invalid", trimmed)
	default:
		return nil
	}
}

// ValidateAuthoredAgentName rejects names that cannot be materialized in an agent catalog.
func ValidateAuthoredAgentName(name string) error {
	trimmed := NormalizeAgentName(name)
	if err := ValidateAgentName(trimmed); err != nil {
		return err
	}
	if IsReservedAgentName(trimmed) {
		return fmt.Errorf("%w: %q", ErrAgentNameReserved, trimmed)
	}
	return nil
}

func normalizeAgentSkillsConfig(config AgentSkillsConfig) AgentSkillsConfig {
	if len(config.Disabled) == 0 {
		return AgentSkillsConfig{}
	}

	normalized := make([]string, 0, len(config.Disabled))
	seen := make(map[string]struct{}, len(config.Disabled))
	for _, raw := range config.Disabled {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	if len(normalized) == 0 {
		return AgentSkillsConfig{}
	}
	return AgentSkillsConfig{Disabled: normalized}
}

func wrapFrontmatterError(err error) error {
	switch {
	case errors.Is(err, frontmatter.ErrMissing):
		return mappedFrontmatterError{
			message: ErrMissingAgentFrontmatter.Error(),
			causes:  []error{ErrMissingAgentFrontmatter, err},
		}
	case errors.Is(err, frontmatter.ErrUnterminated):
		return mappedFrontmatterError{
			message: ErrUnterminatedAgentFrontmatter.Error(),
			causes:  []error{ErrUnterminatedAgentFrontmatter, err},
		}
	case errors.Is(err, frontmatter.ErrBOM):
		return mappedFrontmatterError{
			message: ErrBOMAgentFrontmatter.Error(),
			causes:  []error{ErrBOMAgentFrontmatter, err},
		}
	default:
		return err
	}
}

func decodeAgentFrontmatter(data []byte, parsed *parsedAgentDef) error {
	if hasEmbeddedTabFrontmatterKey(data) {
		return ErrInvalidAgentFrontmatterKey
	}

	yamlErr := yaml.UnmarshalWithOptions(data, parsed, yaml.Strict())
	if yamlErr == nil {
		return nil
	}

	var parsedTOML parsedAgentDef
	meta, tomlErr := toml.Decode(string(data), &parsedTOML)
	if tomlErr != nil {
		return fmt.Errorf(
			"decode agent frontmatter: %w",
			errors.Join(
				fmt.Errorf("yaml: %w", yamlErr),
				fmt.Errorf("toml: %w", tomlErr),
			),
		)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return fmt.Errorf("decode agent frontmatter: unknown field %q", undecoded[0].String())
	}
	*parsed = parsedTOML
	return nil
}

func hasEmbeddedTabFrontmatterKey(data []byte) bool {
	lines := strings.SplitSeq(string(data), "\n")
	for line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.Contains(key, "\t") {
			return true
		}
	}
	return false
}

type mappedFrontmatterError struct {
	message string
	causes  []error
}

func (e mappedFrontmatterError) Error() string {
	return e.message
}

func (e mappedFrontmatterError) Unwrap() []error {
	return e.causes
}

func mergeAgentMCPSidecar(dir string, agent *AgentDef) error {
	if agent == nil {
		return errors.New("agent is required")
	}

	servers, err := LoadMCPServersJSONFile(filepath.Join(strings.TrimSpace(dir), MCPJSONName))
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}

	agent.MCPServers = OverrideMCPServers(agent.MCPServers, servers)
	return nil
}
