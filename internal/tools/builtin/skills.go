package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	skillsSkillsKey = "skills"
)

const (
	skillsCatalogKey = "catalog"
)

var skillTools = []toolspkg.Descriptor{
	skillDescriptor(nativeDescriptor(
		toolspkg.ToolIDSkillList,
		"skill_list",
		"Skill List",
		"List skills with source origin through the existing skill registry.",
		skillListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{skillsSkillsKey, skillsCatalogKey},
		[]string{"available skills", "skill registry"},
	), skillCatalogOutputSchema),
	skillDescriptor(nativeDescriptor(
		toolspkg.ToolIDSkillSearch,
		"skill_search",
		"Skill Search",
		"Search skills with source origin through the existing skill registry.",
		skillSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{skillsSkillsKey, skillsCatalogKey},
		[]string{"find skills", "skill registry search"},
	), skillCatalogOutputSchema),
	skillDescriptor(nativeDescriptor(
		toolspkg.ToolIDSkillView,
		"skill_view",
		"Skill View",
		"Read one skill body or resource with source origin and exposure health.",
		skillViewInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{skillsSkillsKey, skillsCatalogKey, "content"},
		[]string{"skill body", "skill instructions"},
	), skillViewOutputSchema),
}

func skillDescriptors() []toolspkg.Descriptor {
	descriptors := make([]toolspkg.Descriptor, 0, len(skillTools))
	for _, descriptor := range skillTools {
		descriptors = append(descriptors, cloneDescriptor(descriptor))
	}
	return descriptors
}

func skillDescriptor(descriptor toolspkg.Descriptor, outputSchema string) toolspkg.Descriptor {
	descriptor.OutputSchema = json.RawMessage(outputSchema)
	return descriptor
}

const skillListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const skillSearchInputSchema = `{
	"type":"object",
	"required":["query"],
	"properties":{
		"query":{"type":"string"},
		"workspace":{"type":"string"},
		"limit":{"type":"integer"}
	},
	"additionalProperties":false
}`

const skillViewInputSchema = `{
	"type":"object",
	"properties":{
		"name":{"type":"string","description":"Skill name. Provide exactly one of name or command_id."},
		"command_id":{
			"type":"string",
			"description":"Source-qualified session command id. Provide exactly one of command_id or name."
		},
		"workspace":{"type":"string"},
		"file":{
			"type":"string",
			"description":"Skill-relative resource path exactly as listed by the skill, for example references/terminal.md."
		}
	},
	"not":{"required":["name","command_id"]},
	"additionalProperties":false
}`

const skillCatalogOutputSchema = `{
	"type":"object",
	"required":["skills"],
	"properties":{
		"skills":{
			"type":"array",
			"items":{
				"type":"object",
				"required":["name","source","origin","owner_scope"],
				"properties":{
					"name":{"type":"string"},
					"source":{"type":"string"},
					"origin":{"type":"string"},
					"owner_scope":{"type":"string","enum":["user","workspace","profile","workspace_profile"]},
					"owner_id":{"type":"string"}
				}
			}
		}
	},
	"additionalProperties":false
}`

const skillViewOutputSchema = `{
	"type":"object",
	"required":["skill","content"],
	"properties":{
		"skill":{
			"type":"object",
			"required":["name","source","origin","owner_scope","exposures"],
			"properties":{
				"name":{"type":"string"},
				"source":{"type":"string"},
				"origin":{"type":"string"},
				"owner_scope":{"type":"string","enum":["user","workspace","profile","workspace_profile"]},
				"owner_id":{"type":"string"},
				"exposures":{
					"type":"array",
					"items":{
						"type":"object",
						"required":["target","path","status"],
						"properties":{
							"target":{"type":"string"},
							"path":{"type":"string"},
							"status":{"type":"string","enum":["healthy","missing","broken","foreign_conflict"]}
						}
					}
				}
			}
		},
		"content":{"type":"string"},
		"command_id":{"type":"string"},
		"file":{"type":"string"}
	},
	"additionalProperties":false
}`
