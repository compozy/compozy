package agents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	runtimeagent "github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
)

const (
	// ReservedAgentName is the only disallowed reusable agent slug in v1.
	ReservedAgentName = "compozy"
	// ReservedMCPServerName is the host-owned MCP server name that agents may not override.
	ReservedMCPServerName = "compozy"
	reasoningEffortLow    = "low"
	reasoningEffortMedium = "medium"
	reasoningEffortHigh   = "high"
	reasoningEffortXHigh  = "xhigh"
)

const (
	agentDirName   = "agents"
	agentFileName  = "AGENT.md"
	agentMCPConfig = "mcp.json"
)

var (
	slugPattern               = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	envPlaceholderPattern     = regexp.MustCompile(`\$\{([^}]+)\}`)
	unsupportedMetadataFields = []string{"extends", "uses", "skills", "memory"}
)

var (
	// ErrAgentNotFound indicates that no resolved agent matched the requested name.
	ErrAgentNotFound = errors.New("agent not found")
	// ErrInvalidAgentName indicates that an agent directory name failed slug validation.
	ErrInvalidAgentName = errors.New("invalid agent name")
	// ErrReservedAgentName indicates that an agent directory uses a reserved slug.
	ErrReservedAgentName = errors.New("reserved agent name")
	// ErrMissingAgentDefinition indicates that an agent directory does not contain `AGENT.md`.
	ErrMissingAgentDefinition = errors.New("missing AGENT.md")
	// ErrMalformedFrontmatter indicates that `AGENT.md` frontmatter could not be parsed.
	ErrMalformedFrontmatter = errors.New("malformed AGENT.md front matter")
	// ErrUnsupportedMetadataField indicates that `AGENT.md` declares a deferred field.
	ErrUnsupportedMetadataField = errors.New("unsupported agent metadata field")
	// ErrInvalidRuntimeDefaults indicates that agent runtime defaults are invalid.
	ErrInvalidRuntimeDefaults = errors.New("invalid agent runtime defaults")
	// ErrMalformedMCPConfig indicates that `mcp.json` is invalid.
	ErrMalformedMCPConfig = errors.New("malformed mcp.json")
	// ErrMissingEnvironmentVariable indicates that a placeholder referenced an unset environment variable.
	ErrMissingEnvironmentVariable = errors.New("missing environment variable")
	// ErrReservedMCPServerName indicates that `mcp.json` attempted to declare the host-owned server.
	ErrReservedMCPServerName = errors.New("reserved MCP server name")
)

// Scope identifies where an agent definition was discovered.
type Scope string

const (
	// ScopeWorkspace identifies agents discovered from `.compozy/agents`.
	ScopeWorkspace Scope = "workspace"
	// ScopeGlobal identifies agents discovered from `~/.compozy/agents`.
	ScopeGlobal Scope = "global"
)

// Source describes the filesystem origin of a resolved or invalid agent.
type Source struct {
	Scope          Scope
	RootDir        string
	Dir            string
	DefinitionPath string
	MCPConfigPath  string
}

// Metadata contains the human-facing fields from `AGENT.md`.
type Metadata struct {
	Title       string
	Description string
}

// RuntimeDefaults contains the runtime defaults declared in `AGENT.md`.
type RuntimeDefaults struct {
	IDE             string
	Model           string
	ReasoningEffort string
	AccessMode      string
}

// MCPServer describes one agent-local MCP server declaration.
type MCPServer struct {
	Name    string
	Command string
	Args    []string
	Env     map[string]string
}

// MCPConfig contains the resolved contents of `mcp.json`.
type MCPConfig struct {
	Path    string
	Servers []MCPServer
}

// ResolvedAgent is the canonical reusable agent definition consumed by later tasks.
type ResolvedAgent struct {
	Name     string
	Metadata Metadata
	Runtime  RuntimeDefaults
	Prompt   string
	Source   Source
	MCP      *MCPConfig
}

// Problem records a non-fatal discovery or validation failure for one agent directory.
type Problem struct {
	Name   string
	Source Source
	Err    error
}

// Error implements the error interface.
func (p Problem) Error() string {
	if strings.TrimSpace(p.Name) == "" {
		return p.Err.Error()
	}
	return fmt.Sprintf("%s (%s): %v", p.Name, p.Source.Scope, p.Err)
}

// Unwrap exposes the underlying validation failure.
func (p Problem) Unwrap() error {
	return p.Err
}

// Catalog contains all successfully resolved agents plus non-fatal per-agent problems.
type Catalog struct {
	Agents   []ResolvedAgent
	Problems []Problem
}

// Resolve returns one agent from the catalog or the matching validation problem.
func (c Catalog) Resolve(name string) (ResolvedAgent, error) {
	normalized := strings.TrimSpace(name)
	for idx := range c.Agents {
		resolved := c.Agents[idx]
		if resolved.Name == normalized {
			return resolved, nil
		}
	}
	for _, problem := range c.Problems {
		if problem.Name == normalized {
			return ResolvedAgent{}, fmt.Errorf("resolve agent %q: %w", normalized, problem)
		}
	}
	return ResolvedAgent{}, fmt.Errorf("%w: %q", ErrAgentNotFound, normalized)
}

// Option configures a Registry.
type Option func(*Registry)

// WithHomeDir overrides how the registry resolves the global agent root.
func WithHomeDir(fn func() (string, error)) Option {
	return func(r *Registry) {
		if fn != nil {
			r.homeDir = fn
		}
	}
}

// WithLookupEnv overrides how environment variables are resolved during placeholder expansion.
func WithLookupEnv(fn func(string) (string, bool)) Option {
	return func(r *Registry) {
		if fn != nil {
			r.lookupEnv = fn
		}
	}
}

// Registry discovers, parses, validates, and resolves reusable agents.
type Registry struct {
	homeDir   func() (string, error)
	lookupEnv func(string) (string, bool)
}

// New constructs a reusable agent registry with optional test hooks.
func New(opts ...Option) *Registry {
	registry := &Registry{
		homeDir:   os.UserHomeDir,
		lookupEnv: os.LookupEnv,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(registry)
		}
	}
	return registry
}

// Discover scans workspace and global scopes, applying whole-directory workspace overrides.
func (r *Registry) Discover(ctx context.Context, workspaceRoot string) (Catalog, error) {
	if err := context.Cause(ctx); err != nil {
		return Catalog{}, fmt.Errorf("discover agents: %w", err)
	}

	var (
		globalCandidates map[string]agentCandidate
		globalProblems   []Problem
	)
	globalRoot, err := r.globalAgentsRoot()
	if err == nil {
		globalCandidates, globalProblems, err = scanScope(ctx, ScopeGlobal, globalRoot)
		if err != nil {
			return Catalog{}, err
		}
	}
	workspaceCandidates, workspaceProblems, err := scanScope(
		ctx,
		ScopeWorkspace,
		filepath.Join(model.CompozyDir(workspaceRoot), agentDirName),
	)
	if err != nil {
		return Catalog{}, err
	}

	selected := make(map[string]agentCandidate, len(globalCandidates)+len(workspaceCandidates))
	for name, candidate := range globalCandidates {
		selected[name] = candidate
	}
	for name, candidate := range workspaceCandidates {
		selected[name] = candidate
	}

	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)

	catalog := Catalog{
		Problems: append(globalProblems, workspaceProblems...),
	}
	for _, name := range names {
		if err := context.Cause(ctx); err != nil {
			return Catalog{}, fmt.Errorf("discover agents: %w", err)
		}

		resolved, loadErr := r.loadAgent(selected[name])
		if loadErr != nil {
			catalog.Problems = append(catalog.Problems, Problem{
				Name:   name,
				Source: selected[name].Source,
				Err:    loadErr,
			})
			continue
		}
		catalog.Agents = append(catalog.Agents, resolved)
	}

	sort.Slice(catalog.Problems, func(i, j int) bool {
		if catalog.Problems[i].Name != catalog.Problems[j].Name {
			return catalog.Problems[i].Name < catalog.Problems[j].Name
		}
		if catalog.Problems[i].Source.Scope != catalog.Problems[j].Source.Scope {
			return catalog.Problems[i].Source.Scope < catalog.Problems[j].Source.Scope
		}
		return catalog.Problems[i].Source.Dir < catalog.Problems[j].Source.Dir
	})

	return catalog, nil
}

// Resolve discovers the current catalog and returns one resolved agent.
func (r *Registry) Resolve(ctx context.Context, workspaceRoot, name string) (ResolvedAgent, error) {
	catalog, err := r.Discover(ctx, workspaceRoot)
	if err != nil {
		return ResolvedAgent{}, err
	}
	return catalog.Resolve(name)
}

type agentCandidate struct {
	Name   string
	Source Source
}

type frontmatterFields struct {
	Title           string `yaml:"title"`
	Description     string `yaml:"description"`
	IDE             string `yaml:"ide"`
	Model           string `yaml:"model"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	AccessMode      string `yaml:"access_mode"`
}

type rawMCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

func scanScope(
	ctx context.Context,
	scope Scope,
	root string,
) (map[string]agentCandidate, []Problem, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("read %s agent root %s: %w", scope, root, err)
	}

	candidates := make(map[string]agentCandidate, len(entries))
	problems := make([]Problem, 0)
	for _, entry := range entries {
		if err := context.Cause(ctx); err != nil {
			return nil, nil, fmt.Errorf("discover agents: %w", err)
		}
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		source := Source{
			Scope:          scope,
			RootDir:        root,
			Dir:            filepath.Join(root, name),
			DefinitionPath: filepath.Join(root, name, agentFileName),
			MCPConfigPath:  filepath.Join(root, name, agentMCPConfig),
		}
		if err := validateAgentName(name); err != nil {
			problems = append(problems, Problem{Name: name, Source: source, Err: err})
			continue
		}

		candidates[name] = agentCandidate{Name: name, Source: source}
	}
	return candidates, problems, nil
}

func (r *Registry) loadAgent(candidate agentCandidate) (ResolvedAgent, error) {
	content, err := os.ReadFile(candidate.Source.DefinitionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ResolvedAgent{}, fmt.Errorf("%w: %s", ErrMissingAgentDefinition, candidate.Source.DefinitionPath)
		}
		return ResolvedAgent{}, fmt.Errorf("read %s: %w", candidate.Source.DefinitionPath, err)
	}

	metadata, runtime, prompt, err := parseAgentDefinition(candidate.Source.DefinitionPath, string(content))
	if err != nil {
		return ResolvedAgent{}, err
	}

	mcpConfig, err := r.loadMCP(candidate.Source)
	if err != nil {
		return ResolvedAgent{}, err
	}

	return ResolvedAgent{
		Name:     candidate.Name,
		Metadata: metadata,
		Runtime:  runtime,
		Prompt:   prompt,
		Source:   candidate.Source,
		MCP:      mcpConfig,
	}, nil
}

func parseAgentDefinition(path string, content string) (Metadata, RuntimeDefaults, string, error) {
	rawFields := map[string]any{}
	if _, err := frontmatter.Parse(content, &rawFields); err != nil {
		return Metadata{}, RuntimeDefaults{}, "", fmt.Errorf("%w: %s: %v", ErrMalformedFrontmatter, path, err)
	}
	if err := validateUnsupportedFields(path, rawFields); err != nil {
		return Metadata{}, RuntimeDefaults{}, "", err
	}

	var parsed frontmatterFields
	body, err := frontmatter.Parse(content, &parsed)
	if err != nil {
		return Metadata{}, RuntimeDefaults{}, "", fmt.Errorf("%w: %s: %v", ErrMalformedFrontmatter, path, err)
	}

	metadata := Metadata{
		Title:       strings.TrimSpace(parsed.Title),
		Description: strings.TrimSpace(parsed.Description),
	}
	runtime := RuntimeDefaults{
		IDE:             strings.TrimSpace(parsed.IDE),
		Model:           strings.TrimSpace(parsed.Model),
		ReasoningEffort: strings.TrimSpace(parsed.ReasoningEffort),
		AccessMode:      strings.TrimSpace(parsed.AccessMode),
	}
	if err := validateRuntimeDefaults(path, runtime); err != nil {
		return Metadata{}, RuntimeDefaults{}, "", err
	}
	if strings.TrimSpace(runtime.IDE) != "" && strings.TrimSpace(runtime.Model) == "" {
		modelName, err := runtimeagent.ResolveRuntimeModel(runtime.IDE, "")
		if err != nil {
			return Metadata{}, RuntimeDefaults{}, "", fmt.Errorf(
				"%w: %s ide %q is not supported",
				ErrInvalidRuntimeDefaults,
				path,
				runtime.IDE,
			)
		}
		runtime.Model = strings.TrimSpace(modelName)
	}

	return metadata, runtime, body, nil
}

func validateUnsupportedFields(path string, rawFields map[string]any) error {
	for _, field := range unsupportedMetadataFields {
		if _, found := rawFields[field]; found {
			return fmt.Errorf("%w: %s declares %q", ErrUnsupportedMetadataField, path, field)
		}
	}
	return nil
}

func validateRuntimeDefaults(path string, runtime RuntimeDefaults) error {
	if runtime.IDE != "" {
		if _, err := runtimeagent.DriverCatalogEntryForIDE(runtime.IDE); err != nil {
			return fmt.Errorf("%w: %s ide %q is not supported", ErrInvalidRuntimeDefaults, path, runtime.IDE)
		}
	}
	if runtime.ReasoningEffort != "" {
		switch runtime.ReasoningEffort {
		case reasoningEffortLow, reasoningEffortMedium, reasoningEffortHigh, reasoningEffortXHigh:
		default:
			return fmt.Errorf(
				"%w: %s reasoning_effort must be one of low, medium, high, xhigh (got %q)",
				ErrInvalidRuntimeDefaults,
				path,
				runtime.ReasoningEffort,
			)
		}
	}
	if runtime.AccessMode != "" {
		switch runtime.AccessMode {
		case model.AccessModeDefault, model.AccessModeFull:
		default:
			return fmt.Errorf(
				"%w: %s access_mode must be %q or %q (got %q)",
				ErrInvalidRuntimeDefaults,
				path,
				model.AccessModeDefault,
				model.AccessModeFull,
				runtime.AccessMode,
			)
		}
	}
	return nil
}

func (r *Registry) loadMCP(source Source) (*MCPConfig, error) {
	content, err := os.ReadFile(source.MCPConfigPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", source.MCPConfigPath, err)
	}

	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrMalformedMCPConfig, source.MCPConfigPath, err)
	}

	rawServers, ok := document["mcpServers"]
	if !ok {
		return nil, fmt.Errorf(
			"%w: %s is missing top-level \"mcpServers\"",
			ErrMalformedMCPConfig,
			source.MCPConfigPath,
		)
	}

	serversByName := map[string]rawMCPServer{}
	if err := json.Unmarshal(rawServers, &serversByName); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrMalformedMCPConfig, source.MCPConfigPath, err)
	}

	names := make([]string, 0, len(serversByName))
	for name := range serversByName {
		names = append(names, name)
	}
	sort.Strings(names)

	servers := make([]MCPServer, 0, len(names))
	for _, name := range names {
		if name == ReservedMCPServerName {
			return nil, fmt.Errorf("%w: %s declares %q", ErrReservedMCPServerName, source.MCPConfigPath, name)
		}

		rawServer := serversByName[name]
		server, err := r.resolveMCPServer(source, name, rawServer)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return &MCPConfig{
		Path:    source.MCPConfigPath,
		Servers: servers,
	}, nil
}

func (r *Registry) resolveMCPServer(source Source, name string, rawServer rawMCPServer) (MCPServer, error) {
	command, err := r.expandPlaceholders(source.MCPConfigPath, rawServer.Command)
	if err != nil {
		return MCPServer{}, err
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return MCPServer{}, fmt.Errorf(
			"%w: %s server %q command is required",
			ErrMalformedMCPConfig,
			source.MCPConfigPath,
			name,
		)
	}
	command = resolveCommandPath(command, source.Dir)

	args := make([]string, 0, len(rawServer.Args))
	for _, arg := range rawServer.Args {
		expanded, err := r.expandPlaceholders(source.MCPConfigPath, arg)
		if err != nil {
			return MCPServer{}, err
		}
		args = append(args, expanded)
	}

	var env map[string]string
	if len(rawServer.Env) > 0 {
		env = make(map[string]string, len(rawServer.Env))
		keys := make([]string, 0, len(rawServer.Env))
		for key := range rawServer.Env {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			expanded, err := r.expandPlaceholders(source.MCPConfigPath, rawServer.Env[key])
			if err != nil {
				return MCPServer{}, err
			}
			env[key] = expanded
		}
	}

	return MCPServer{
		Name:    name,
		Command: command,
		Args:    args,
		Env:     env,
	}, nil
}

func (r *Registry) expandPlaceholders(path, value string) (string, error) {
	matches := envPlaceholderPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return value, nil
	}

	var builder strings.Builder
	last := 0
	for _, match := range matches {
		variable := value[match[2]:match[3]]
		resolved, ok := r.lookupEnv(variable)
		if !ok {
			return "", fmt.Errorf("%w: %s references %q", ErrMissingEnvironmentVariable, path, variable)
		}
		builder.WriteString(value[last:match[0]])
		builder.WriteString(resolved)
		last = match[1]
	}
	builder.WriteString(value[last:])
	return builder.String(), nil
}

func validateAgentName(name string) error {
	switch {
	case name == ReservedAgentName:
		return fmt.Errorf("%w: %q", ErrReservedAgentName, name)
	case !slugPattern.MatchString(name):
		return fmt.Errorf("%w: %q must match %s", ErrInvalidAgentName, name, slugPattern.String())
	default:
		return nil
	}
}

func resolveCommandPath(command, dir string) string {
	if filepath.IsAbs(command) || !looksLikePath(command) {
		return command
	}
	return filepath.Clean(filepath.Join(dir, command))
}

func looksLikePath(command string) bool {
	return strings.HasPrefix(command, ".") ||
		strings.Contains(command, "/") ||
		strings.Contains(command, `\`)
}

func (r *Registry) globalAgentsRoot() (string, error) {
	home, err := r.homeDir()
	if err != nil {
		return "", fmt.Errorf("resolve global agents root: %w", err)
	}
	return filepath.Join(home, model.WorkflowRootDirName, agentDirName), nil
}
