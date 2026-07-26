package builtin

import toolspkg "github.com/compozy/agh/internal/tools"

const (
	skillsSkillsKey = "skills"
)

const (
	skillsCatalogKey = "catalog"
)

var skillTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDSkillList,
		"skill_list",
		"Skill List",
		"List skills through the existing skill registry.",
		skillListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{skillsSkillsKey, skillsCatalogKey},
		[]string{"available skills", "skill registry"},
	),
	nativeDescriptor(
		toolspkg.ToolIDSkillSearch,
		"skill_search",
		"Skill Search",
		"Search skills through the existing skill registry.",
		skillSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{skillsSkillsKey, skillsCatalogKey},
		[]string{"find skills", "skill registry search"},
	),
	nativeDescriptor(
		toolspkg.ToolIDSkillView,
		"skill_view",
		"Skill View",
		"Read one skill body or one resource file through the existing skill registry.",
		skillViewInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{skillsSkillsKey, skillsCatalogKey, "content"},
		[]string{"skill body", "skill instructions"},
	),
}

func skillDescriptors() []toolspkg.Descriptor {
	return skillTools
}

const skillListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace_id":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const skillSearchInputSchema = `{
	"type":"object",
	"required":["query"],
	"properties":{
		"query":{"type":"string"},
		"workspace_id":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const skillViewInputSchema = `{
	"type":"object",
	"required":["name"],
	"properties":{
		"name":{"type":"string"},
		"workspace_id":{"type":"string"},
		"file":{"type":"string"}
	},
	"additionalProperties":false
}`
