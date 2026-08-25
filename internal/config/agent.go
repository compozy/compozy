package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
	"github.com/compozy/compozy/internal/frontmatter"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/goccy/go-yaml"
)

var resourceProfileNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// AgentDef is the parsed representation of an AGENT.md file.
type AgentDef struct {
	Name                string               `json:"name"                       yaml:"name"                       toml:"name"`
	Provider            string               `json:"provider,omitempty"         yaml:"provider"                   toml:"provider"`
	Command             string               `json:"command,omitempty"          yaml:"command,omitempty"          toml:"command,omitempty"`
	Model               string               `json:"model,omitempty"            yaml:"model,omitempty"            toml:"model,omitempty"`
	ReasoningEffort     string               `json:"reasoning_effort,omitempty" yaml:"reasoning_effort,omitempty" toml:"reasoning_effort,omitempty"`
	Tools               []string             `json:"tools,omitempty"            yaml:"tools,omitempty"            toml:"tools,omitempty"`
	Toolsets            []string             `json:"toolsets,omitempty"         yaml:"toolsets,omitempty"         toml:"toolsets,omitempty"`
	DenyTools           []string             `json:"deny_tools,omitempty"       yaml:"deny_tools,omitempty"       toml:"deny_tools,omitempty"`
	Permissions         string               `json:"permissions,omitempty"      yaml:"permissions,omitempty"      toml:"permissions,omitempty"`
	Skills              AgentSkillsConfig    `json:"skills,omitzero"            yaml:"skills,omitempty"           toml:"skills,omitempty"`
	CategoryPath        []string             `json:"category_path,omitempty"    yaml:"category_path,omitempty"    toml:"category_path,omitempty"`
	MCPServers          []MCPServer          `json:"mcp_servers,omitempty"      yaml:"mcp_servers,omitempty"      toml:"mcp_servers,omitempty"`
	Hooks               []hookspkg.HookDecl  `json:"hooks,omitempty"            yaml:"hooks,omitempty"            toml:"hooks,omitempty"`
	Capabilities        *CapabilityCatalog   `json:"capabilities,omitempty"     yaml:"-"                          toml:"-"`
	Prompt              string               `json:"prompt,omitempty"           yaml:"-"`
	SourcePath          string               `json:"-"                          yaml:"-"                          toml:"-"`
	SourceLayer         string               `json:"-"                          yaml:"-"                          toml:"-"`
	ShadowedDefinitions []AgentDefinitionRef `json:"-"                          yaml:"-"                          toml:"-"`
}

// AgentDefinitionRef identifies one lower-precedence definition hidden by a winner.
type AgentDefinitionRef struct {
	Layer string
	Path  string
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
	agentLayerUnknown = "unknown"

	// WorkspaceDiscoverySourceWorkspace marks the primary workspace root.
	WorkspaceDiscoverySourceWorkspace WorkspaceDiscoverySource = "workspace"
	// WorkspaceDiscoverySourceAdditional marks an additional workspace root.
	WorkspaceDiscoverySourceAdditional WorkspaceDiscoverySource = "additional"
	// WorkspaceDiscoverySourceGlobal marks the global Compozy home root.
	WorkspaceDiscoverySourceGlobal WorkspaceDiscoverySource = "global"
	// WorkspaceDiscoverySourceProfile marks the active profile's personal root.
	WorkspaceDiscoverySourceProfile WorkspaceDiscoverySource = "profile"
	// WorkspaceDiscoverySourceWorkspaceProfile marks the project root bound to the active profile name.
	WorkspaceDiscoverySourceWorkspaceProfile WorkspaceDiscoverySource = "workspace_profile"
)

// WorkspaceDiscoveryRoot describes a filesystem root participating in multi-root resource discovery.
type WorkspaceDiscoveryRoot struct {
	Dir             string
	Source          WorkspaceDiscoverySource
	ProfileID       string
	WorkspaceID     string
	ResourceScopeID string
	OperatorHomeDir string
}

// AgentLayerName returns the public layer label for a discovery source.
func AgentLayerName(source WorkspaceDiscoverySource) string {
	switch source {
	case WorkspaceDiscoverySourceGlobal:
		return string(WriteScopeUser)
	case WorkspaceDiscoverySourceProfile:
		return "profile"
	case WorkspaceDiscoverySourceAdditional:
		return string(WorkspaceDiscoverySourceAdditional)
	case WorkspaceDiscoverySourceWorkspace:
		return "project"
	case WorkspaceDiscoverySourceWorkspaceProfile:
		return "project_profile"
	default:
		return agentLayerUnknown
	}
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

// LoadAgentDef loads an AGENT.md file from the configured Compozy home directory.
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
func LoadAgentDefFile(path string) (agent AgentDef, err error) {
	agentPath := filepath.Clean(path)
	agentDirPath := filepath.Dir(agentPath)
	directory, err := fileutil.OpenDirectory(agentDirPath)
	if err != nil {
		return AgentDef{}, fmt.Errorf("open agent directory %q: %w", agentDirPath, err)
	}
	defer func() {
		if closeErr := directory.Close(); closeErr != nil {
			agent = AgentDef{}
			err = errors.Join(err, fmt.Errorf("close agent directory %q: %w", agentDirPath, closeErr))
		}
	}()

	return loadAgentDefFromDirectory(directory, filepath.Base(agentPath), agentPath)
}

func loadAgentDefFromDirectory(
	directory *fileutil.Directory,
	agentFileName string,
	agentPath string,
) (AgentDef, error) {
	contents, _, err := directory.ReadRegularFile(agentFileName)
	if err != nil {
		return AgentDef{}, fmt.Errorf("read agent file %q: %w", agentPath, err)
	}

	agent, err := ParseAgentDef(contents)
	if err != nil {
		return AgentDef{}, fmt.Errorf("parse agent file %q: %w", agentPath, err)
	}
	if err := mergeAgentMCPSidecarFromDirectory(
		directory,
		filepath.Join(filepath.Dir(agentPath), MCPJSONName),
		&agent,
	); err != nil {
		return AgentDef{}, fmt.Errorf("load agent file %q MCP JSON: %w", agentPath, err)
	}
	capabilities, err := loadAgentCapabilitiesFromDirectory(directory, filepath.Dir(agentPath))
	if err != nil {
		return AgentDef{}, fmt.Errorf("load agent file %q capability catalog: %w", agentPath, err)
	}
	agent.Capabilities = capabilities
	if err := agent.Validate(); err != nil {
		return AgentDef{}, fmt.Errorf("validate agent file %q: %w", agentPath, err)
	}
	agent.SourcePath = agentPath

	return agent, nil
}

// WorkspaceDiscoveryRoots returns ordered discovery roots for workspace-visible resources.
// A non-empty profile name inserts the two profile-bound roots at their precedence slots.
func WorkspaceDiscoveryRoots(
	rootDir string,
	additionalDirs []string,
	homePaths HomePaths,
	profileName string,
) []WorkspaceDiscoveryRoot {
	profileName = strings.TrimSpace(profileName)
	roots := make([]WorkspaceDiscoveryRoot, 0, len(additionalDirs)+4)

	if trimmed := strings.TrimSpace(rootDir); trimmed != "" {
		if profileName != "" {
			roots = append(roots, WorkspaceDiscoveryRoot{
				Dir:    filepath.Join(trimmed, DirName, ProfilesDirName, profileName),
				Source: WorkspaceDiscoverySourceWorkspaceProfile,
			})
		}
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

	if profileName != "" && strings.TrimSpace(homePaths.ProfilesDir) != "" {
		roots = append(roots, WorkspaceDiscoveryRoot{
			Dir:    filepath.Join(homePaths.ProfilesDir, profileName),
			Source: WorkspaceDiscoverySourceProfile,
		})
	}

	if trimmed := strings.TrimSpace(homePaths.HomeDir); trimmed != "" {
		roots = append(roots, WorkspaceDiscoveryRoot{
			Dir:             trimmed,
			Source:          WorkspaceDiscoverySourceGlobal,
			OperatorHomeDir: strings.TrimSpace(homePaths.OperatorHomeDir),
		})
	}

	return roots
}

// ValidateResourceProfileName validates a profile name used in discovery paths.
func ValidateResourceProfileName(name string) error {
	trimmed := strings.TrimSpace(name)
	if !resourceProfileNamePattern.MatchString(trimmed) {
		return fmt.Errorf(
			"config: resource profile name %q must match %s",
			trimmed,
			resourceProfileNamePattern.String(),
		)
	}
	return nil
}

// AgentsDir returns the agent-definition directory for this discovery root.
func (r WorkspaceDiscoveryRoot) AgentsDir() string {
	if r.usesHomeResourceLayout() {
		return filepath.Join(r.Dir, AgentsDirName)
	}

	return filepath.Join(r.Dir, DirName, AgentsDirName)
}

// LoopsDir returns the loop-definition directory for this discovery root.
func (r WorkspaceDiscoveryRoot) LoopsDir() string {
	if r.usesHomeResourceLayout() {
		return filepath.Join(r.Dir, LoopsDirName)
	}

	return filepath.Join(r.Dir, DirName, LoopsDirName)
}

func (r WorkspaceDiscoveryRoot) usesHomeResourceLayout() bool {
	switch r.Source {
	case WorkspaceDiscoverySourceGlobal, WorkspaceDiscoverySourceProfile, WorkspaceDiscoverySourceWorkspaceProfile:
		return true
	default:
		return false
	}
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

func mergeAgentMCPSidecarFromDirectory(directory *fileutil.Directory, path string, agent *AgentDef) error {
	if agent == nil {
		return errors.New("agent is required")
	}

	servers, err := loadMCPServersJSONFromDirectory(directory, path)
	if err != nil {
		return err
	}
	if len(servers) == 0 {
		return nil
	}

	agent.MCPServers = OverrideMCPServers(agent.MCPServers, servers)
	return nil
}
