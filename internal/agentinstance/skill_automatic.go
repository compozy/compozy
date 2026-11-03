package agentinstance

import (
	"path"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/frontmatter"
	yaml "gopkg.in/yaml.v3"
)

// SkillAutomaticContents inventories declarations for review; it never executes them.
type SkillAutomaticContents struct{ Hooks, MCPServers, Scripts int }

func (draft SkillDraft) AutomaticContents() (SkillAutomaticContents, error) {
	var result SkillAutomaticContents
	for _, file := range draft.Files {
		if strings.HasPrefix(file.Path, "scripts/") {
			result.Scripts++
		}
		switch path.Base(file.Path) {
		case compozyconfig.MCPJSONName:
			servers, err := compozyconfig.ParseMCPServersJSON([]byte(file.Content), file.Path)
			if err != nil {
				return result, err
			}
			result.MCPServers += len(servers)
		case skillDefinitionFile:
			parts, err := frontmatter.Split([]byte(file.Content))
			if err != nil {
				return result, err
			}
			var metadata struct {
				Metadata struct {
					Compozy struct {
						Hooks      []any `yaml:"hooks"`
						MCPServers []any `yaml:"mcp_servers"`
					} `yaml:"compozy"`
				} `yaml:"metadata"`
			}
			if err := yaml.Unmarshal(parts.Metadata, &metadata); err != nil {
				return result, err
			}
			result.Hooks += len(metadata.Metadata.Compozy.Hooks)
			result.MCPServers += len(metadata.Metadata.Compozy.MCPServers)
		}
	}
	return result, nil
}
