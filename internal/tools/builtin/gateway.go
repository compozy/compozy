package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func gatewayDescriptors() []toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDGateway,
		"gateway",
		"Gateway",
		"Inspect gateway status or audit results in every permission mode. Management actions are accepted only "+
			"for sessions whose effective permission mode is approve-all.",
		gatewayInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDGateway},
		[]string{"gateway", "remote access", "exposure", descriptorKeywordAudit, "devices", "providers"},
		[]string{"gateway status", "gateway audit", "repair remote exposure", "manage paired devices"},
	)
	descriptor.OutputSchema = []byte(gatewayOutputSchema)
	return []toolspkg.Descriptor{descriptor}
}

const gatewayInputSchema = `{
	"type":"object",
	"required":["action"],
	"properties":{
		"action":{"type":"string","enum":[
			"status","audit","device_list","surface_set","provider_enable","provider_disable",
			"device_rename","device_revoke","ingress_bind","ingress_unbind"
		]},
		"surface":{"type":"string","enum":["operator_ui","webhook_ingress"]},
		"tier":{"type":"string","enum":["private","public"]},
		"desired":{"type":"string","enum":["enabled","disabled"]},
		"expected_generation":{"type":"integer","minimum":0},
		"consent":{"type":"boolean"},
		"provider":{"type":"string"},
		"install_source":{"type":"string"},
		"digest_confirmed":{"type":"string"},
		"device_id":{"type":"string"},
		"name":{"type":"string"},
		"subject_kind":{"type":"string","enum":["webhook_trigger","bridge_instance"]},
		"subject_id":{"type":"string"},
		"confirmed":{"type":"boolean"}
	},
	"additionalProperties":false
}`

const gatewayOutputSchema = `{
	"type":"object",
	"required":["action","redacted"],
	"properties":{
		"action":{"type":"string","enum":[
			"status","audit","device_list","surface_set","provider_enable","provider_disable",
			"device_rename","device_revoke","ingress_bind","ingress_unbind"
		]},
		"redacted":{"type":"boolean"},
		"status":{"type":"object"},
		"audit":{"type":"object"},
		"devices":{"type":"array","items":{"type":"object"}},
		"device":{"type":"object"},
		"revoke":{"type":"object"},
		"binding":{"type":"object"},
		"unbind":{"type":"object"}
	},
	"additionalProperties":false
}`
