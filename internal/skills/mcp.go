package skills

import (
	"log/slog"
	"sort"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

// MCPResolver collects and resolves MCP server declarations from enabled skills.
type MCPResolver struct {
	logger *slog.Logger
}

// NewMCPResolver constructs an MCPResolver with logger settings.
func NewMCPResolver(logger *slog.Logger) *MCPResolver {
	if logger == nil {
		logger = slog.Default()
	}

	return &MCPResolver{
		logger: logger,
	}
}

// Resolve returns MCP servers from enabled skills after trust-tier filtering.
// When multiple declarations share the same trimmed server name, the later
// skill in source-precedence order replaces the earlier one ("last wins").
// The caller then passes the result through compozyconfig.MergeMCPServers, which
// keeps the first server at each final position. Combined together, skill-local
// duplicates are resolved last-wins before config-vs-skill merge applies its
// first-wins behavior.
func (mr *MCPResolver) Resolve(skills []*Skill) []compozyconfig.MCPServer {
	if len(skills) == 0 {
		return nil
	}

	ordered := orderSkillsBySource(skills)

	resolved := make([]compozyconfig.MCPServer, 0)
	index := make(map[string]int)
	origins := make([]mcpOrigin, 0)

	for _, skill := range ordered {
		if skill == nil || !skill.Enabled || len(skill.MCPServers) == 0 {
			continue
		}
		for _, server := range skill.MCPServers {
			if !skillMCPAllowed(skill) {
				mr.logger.Warn(
					"blocked MCP server",
					"skill_name", skill.Meta.Name,
					"mcp_server", server.Name,
					"source", skillSourceName(skill.Source),
				)
				continue
			}

			resolvedServer := toConfigMCPServer(server)
			resolvedServer.Name = strings.TrimSpace(resolvedServer.Name)
			name := resolvedServer.Name
			if idx, ok := index[name]; ok && name != "" {
				resolved[idx] = resolvedServer
				origins[idx] = mcpOrigin{
					skillName: skill.Meta.Name,
					source:    skill.Source,
				}
				continue
			}

			resolved = append(resolved, resolvedServer)
			origins = append(origins, mcpOrigin{
				skillName: skill.Meta.Name,
				source:    skill.Source,
			})
			if name != "" {
				index[name] = len(resolved) - 1
			}
		}
	}

	for i, server := range resolved {
		mr.logger.Info(
			"resolved MCP server",
			"skill_name", origins[i].skillName,
			"mcp_server", server.Name,
			"source", skillSourceName(origins[i].source),
		)
	}

	if len(resolved) == 0 {
		return nil
	}

	return resolved
}

type mcpOrigin struct {
	skillName string
	source    SkillSource
}

func orderSkillsBySource(skills []*Skill) []*Skill {
	ordered := append([]*Skill(nil), skills...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i]
		right := ordered[j]
		if left == nil || right == nil {
			return left != nil
		}
		return SkillPrecedenceRank(left.Source) < SkillPrecedenceRank(right.Source)
	})
	return ordered
}

func skillMCPAllowed(skill *Skill) bool {
	if skill == nil {
		return false
	}
	switch skill.Source {
	case SourceBundled, SourceUser, SourceAdditional, SourceWorkspace, SourceProfile, SourceWorkspaceProfile:
		return true
	default:
		return false
	}
}

func toConfigMCPServer(decl MCPServerDecl) compozyconfig.MCPServer {
	return compozyconfig.MCPServer{
		Name:      decl.Name,
		Command:   decl.Command,
		Args:      append([]string(nil), decl.Args...),
		Env:       cloneStringMap(decl.Env),
		SecretEnv: cloneStringMap(decl.SecretEnv),
	}
}
