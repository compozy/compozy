package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	bridgesBridgesKey = "bridges"
	bridgesHealthKey  = "health"
)

var bridgeTools = []toolspkg.Descriptor{
	bridgeListDescriptor(),
	nativeDescriptor(
		toolspkg.ToolIDBridgesStatus,
		"bridges_status",
		"Bridges Status",
		"Read bridge status and health without bridge credentials or secret bindings.",
		bridgesStatusInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBridges},
		[]string{bridgesBridgesKey, descriptorKeywordStatus, bridgesHealthKey},
		[]string{"bridge status", "bridge health"},
	),
}

func bridgeDescriptors() []toolspkg.Descriptor {
	return bridgeTools
}

func bridgeListDescriptor() toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDBridgesList,
		"bridges_list",
		"Bridges List",
		"List one bounded, workspace-safe bridge catalog page without credentials or secret bindings.",
		bridgesListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDBridges},
		[]string{bridgesBridgesKey, "list"},
		[]string{"bridge list", "bridge instances"},
	)
	descriptor.OutputSchema = json.RawMessage(bridgesListOutputSchema)
	return descriptor
}

const bridgesListInputSchema = `{
	"type":"object",
	"properties":{
		"workspace_id":{"type":"string"},
		"scope":{"type":"string","enum":["all","global","workspace"]},
		"q":{"type":"string"},
		"platform":{"type":"string"},
		"status":{"type":"string","enum":["disabled","starting","ready","degraded","auth_required","error"]},
		"sort":{"type":"string","enum":["name"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":200}
	},
	"additionalProperties":false
}`

const bridgesListOutputSchema = `{
	"type":"object",
	"required":["bridges","bridge_health","facets","page","redacted"],
	"properties":{
		"bridges":{"type":"array","items":{"type":"object"}},
		"bridge_health":{"type":"object","additionalProperties":{"type":"object"}},
		"facets":{"type":"object"},
		"page":{
			"type":"object",
			"required":["has_more","total","limit"],
			"properties":{
				"next_cursor":{"type":"string"},
				"has_more":{"type":"boolean"},
				"total":{"type":"integer","minimum":0},
				"limit":{"type":"integer","minimum":1,"maximum":200}
			},
			"additionalProperties":false
		},
		"redacted":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const bridgesStatusInputSchema = `{
	"type":"object",
	"properties":{
		"bridge_id":{"type":"string"},
		"workspace_id":{"type":"string"},
		"scope":{"type":"string","enum":["all","global","workspace"]},
		"q":{"type":"string"},
		"platform":{"type":"string"},
		"status":{"type":"string","enum":["disabled","starting","ready","degraded","auth_required","error"]},
		"sort":{"type":"string","enum":["name"]},
		"cursor":{"type":"string"},
		"limit":{"type":"integer","minimum":1,"maximum":200}
	},
	"additionalProperties":false
}`
