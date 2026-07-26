package skills

import (
	"log/slog"
	"sort"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

// MCPResolver collects and resolves MCP server declarations from enabled skills.
type MCPResolver struct {
	allowedMarketplace []string
	logger             *slog.Logger
}

// NewMCPResolver constructs an MCPResolver from skills config and logger settings.
func NewMCPResolver(cfg aghconfig.SkillsConfig, logger *slog.Logger) *MCPResolver {
	if logger == nil {
		logger = slog.Default()
	}

	return &MCPResolver{
		allowedMarketplace: cloneStrings(cfg.AllowedMarketplaceMCP),
		logger:             logger,
	}
}

// Resolve returns MCP servers from enabled skills after trust-tier filtering.
// When multiple declarations share the same trimmed server name, the later
// skill in source-precedence order replaces the earlier one ("last wins").
// The caller then passes the result through aghconfig.MergeMCPServers, which
// keeps the first server at each final position. Combined together, skill-local
// duplicates are resolved last-wins before config-vs-skill merge applies its
// first-wins behavior.
func (mr *MCPResolver) Resolve(skills []*Skill) []aghconfig.MCPServer {
	if len(skills) == 0 {
		return nil
	}

	ordered := orderSkillsBySource(skills)
	allowedMarketplace := marketplaceAllowlist(mr.allowedMarketplace)

	resolved := make([]aghconfig.MCPServer, 0)
	index := make(map[string]int)
	origins := make([]mcpOrigin, 0)

	for _, skill := range ordered {
		if skill == nil || !skill.Enabled || len(skill.MCPServers) == 0 {
			continue
		}
		for _, server := range skill.MCPServers {
			if !marketplaceSkillAllowed(skill, allowedMarketplace) {
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
		return left.Source < right.Source
	})
	return ordered
}

func marketplaceAllowlist(values []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}

	return allowed
}

func marketplaceSkillAllowed(skill *Skill, allowedMarketplace map[string]struct{}) bool {
	if skill == nil {
		return false
	}

	switch skill.Source {
	case SourceBundled, SourceUser, SourceAdditional, SourceWorkspace:
		return true
	case SourceMarketplace:
		for _, key := range marketplaceConsentKeys(skill) {
			if _, ok := allowedMarketplace[key]; ok {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func marketplaceConsentKeys(skill *Skill) []string {
	if skill == nil || skill.Provenance == nil {
		return nil
	}

	provenance := skill.Provenance
	keys := make([]string, 0, 3)
	if slug := strings.TrimSpace(provenance.Slug); slug != "" {
		keys = append(keys, slug)
		if registry := strings.TrimSpace(provenance.Registry); registry != "" {
			keys = append(keys, registry+":"+slug)
		}
	}
	if hash := strings.TrimSpace(provenance.Hash); hash != "" {
		keys = append(keys, hash)
	}

	return keys
}

func toConfigMCPServer(decl MCPServerDecl) aghconfig.MCPServer {
	return aghconfig.MCPServer{
		Name:      decl.Name,
		Command:   decl.Command,
		Args:      append([]string(nil), decl.Args...),
		Env:       cloneStringMap(decl.Env),
		SecretEnv: cloneStringMap(decl.SecretEnv),
	}
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}

	return append([]string(nil), values...)
}
