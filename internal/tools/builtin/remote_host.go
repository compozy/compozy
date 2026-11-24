package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

func remoteHostDescriptors() []toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDRemoteStatus, "remote_status", "Remote status",
		"Read remote access status for this computer.", `{"type":"object","additionalProperties":false}`,
		toolspkg.RiskRead, true, false, true, nil,
		[]string{"remote access", "computer", "listeners"}, []string{"remote status", "is remote access enabled"},
	)
	descriptor.OutputSchema = []byte(remoteStatusOutput)
	return []toolspkg.Descriptor{descriptor}
}

const remoteStatusOutput = `{
	"type":"object","additionalProperties":false,
	"required":["computer_id","enabled","listener","direct","cloud_route",
		"entitlement","control_link","require_device_approval"],
	"properties":{
		"computer_id":{"type":"string"},
		"enabled":{"type":"boolean"},
		"listener":{"type":"string"},
		"direct":{"type":"object","required":["enabled"],"additionalProperties":false,
			"properties":{"enabled":{"type":"boolean"},"addresses":{"type":"array","items":{"type":"string"}}}},
		"cloud_route":{"type":"object","required":["enabled","state","hostname"],"additionalProperties":false,
			"properties":{"enabled":{"type":"boolean"},"state":{"type":"string"},"hostname":{"type":["string","null"]}}},
		"entitlement":{"type":["object","null"],"required":["valid_until","plan"],"additionalProperties":false,
			"properties":{"valid_until":{"type":"string","format":"date-time"},"plan":{"type":"string"}}},
		"control_link":{"type":"string"},
		"require_device_approval":{"type":"boolean"}
	}
}`
