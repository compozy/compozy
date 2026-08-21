package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var profileTools = []toolspkg.Descriptor{
	func() toolspkg.Descriptor {
		descriptor := nativeDescriptor(
			toolspkg.ToolIDProfileList,
			"profile_list",
			"Profile List",
			"List profiles and mark the profile bound to the current session, or the permanent default outside a bound session.",
			emptyInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
			[]string{"profiles", descriptorKeywordCatalog, descriptorKeywordStatus},
			[]string{"profile list", "available profiles", "current profile"},
		)
		descriptor.OutputSchema = []byte(profileListOutputSchema)
		return descriptor
	}(),
	func() toolspkg.Descriptor {
		descriptor := nativeDescriptor(
			toolspkg.ToolIDProfileCurrent,
			"profile_current",
			"Profile Current",
			"Read the immutable profile and workspace bound to the current session, "+
				"or the permanent default outside a bound session.",
			emptyInputSchema,
			toolspkg.RiskRead,
			true,
			false,
			false,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
			[]string{"profiles", descriptorKeywordStatus, "session"},
			[]string{"current profile", "session profile", "active profile"},
		)
		descriptor.OutputSchema = []byte(profileCurrentOutputSchema)
		return descriptor
	}(),
}

func profileDescriptors() []toolspkg.Descriptor { return profileTools }

const profileListOutputSchema = `{
	"type":"object",
	"required":["profiles"],
	"properties":{"profiles":{"type":"array","items":{
		"type":"object",
		"required":["name","state","current","work_items","needs_setup","credential_requirements"],
		"properties":{
			"name":{"type":"string"},
			"state":{"type":"string","enum":["active","archived"]},
			"current":{"type":"boolean"},
			"work_items":{"type":"integer"},
			"needs_setup":{"type":"boolean"},
			"credential_requirements":{"type":"array","items":{
				"type":"object",
				"required":["provider","slot","source_extension","missing"],
				"properties":{
					"provider":{"type":"string"},
					"slot":{"type":"string"},
					"source_extension":{"type":"string"},
					"missing":{"type":"boolean"}
				},
				"additionalProperties":false
			}}
		},
		"additionalProperties":false
	}}},
	"additionalProperties":false
}`

const profileCurrentOutputSchema = `{
	"type":"object",
	"required":["profile","source"],
	"properties":{
		"profile":{"type":"string"},
		"source":{"type":"string","enum":["session","default"]},
		"workspace":{"type":"string"}
	},
	"additionalProperties":false
}`
