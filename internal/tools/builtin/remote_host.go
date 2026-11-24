package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var singleResultRemoteHost = map[toolspkg.ToolID]SingleResultReason{
	toolspkg.ToolIDRemoteStatus:        SingleResultMinimal,
	toolspkg.ToolIDComputerSetupCancel: SingleResultMinimal,
	toolspkg.ToolIDComputerSetupAnswer: SingleResultMinimal,
	toolspkg.ToolIDComputerSetupStatus: SingleResultMinimal,
	toolspkg.ToolIDComputerList:        SingleResultMinimal,
	toolspkg.ToolIDComputerStatus:      SingleResultMinimal,
	toolspkg.ToolIDComputerAdd:         SingleResultMinimal,
	toolspkg.ToolIDComputerRemove:      SingleResultMinimal,
	toolspkg.ToolIDAccountStatus:       SingleResultMinimal,
}

func remoteHostDescriptors() []toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		toolspkg.ToolIDRemoteStatus,
		"remote_status",
		"Remote status",
		"Read remote access and hosted-route readiness for this computer.",
		`{"type":"object","additionalProperties":false}`,
		toolspkg.RiskRead,
		true,
		false,
		false,
		nil,
		[]string{"remote access", "computer", "listeners"},
		[]string{"remote status", "is remote access enabled"},
	)
	descriptor.OutputSchema = []byte(remoteStatusOutput)
	cancel := nativeDescriptor(
		toolspkg.ToolIDComputerSetupCancel,
		"computer_setup_cancel",
		"Cancel computer setup",
		"Stop an unlinked SSH setup on this controller and release its slot.",
		`{"type":"object","additionalProperties":false,"required":["setup_id"],"properties":{"setup_id":{"type":"string","minLength":1}}}`,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		nil,
		[]string{"ssh", "computer", "setup"},
		[]string{"cancel ssh computer setup", "release free computer slot"},
	)
	cancel.OutputSchema = []byte(`{"type":"object","additionalProperties":false,"required":["setup_id","state"],
		"properties":{"setup_id":{"type":"string"},"state":{"const":"cancelled"}}}`) //nolint:misspell // Public state.
	return append([]toolspkg.Descriptor{descriptor, cancel}, controllerComputerDescriptors()...)
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
			"properties":{
				"enabled":{"type":"boolean"},
				"state":{"type":"string"},
				"hostname":{"type":["string","null"]},
				"diagnosis":{"type":"string"}
			}},
		"entitlement":{"type":["object","null"],"required":["valid_until","plan"],"additionalProperties":false,
			"properties":{"valid_until":{"type":"string","format":"date-time"},"plan":{"type":"string"}}},
		"control_link":{"type":"string"},
		"require_device_approval":{"type":"boolean"}
	}
}`
