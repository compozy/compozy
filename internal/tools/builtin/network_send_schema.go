package builtin

import networkpkg "github.com/compozy/agh/internal/network"

const (
	networkSendSchemaDescription = "For say, capability, receipt, and trace, surface is required. " +
		"surface=thread requires thread_id; the first valid send creates the public thread. " +
		"surface=direct requires an existing direct_id from network_direct_resolve. " +
		"capability, receipt, and trace require work_id. " +
		"capability and lifecycle-bearing say also require to. " +
		"greet and whois omit conversation and work fields."
	networkSendSessionDescription = "Local source session. " +
		"The hosted native surface supplies the bound session when omitted."
	networkSendSurfaceDescription = "Conversation surface; required for say, capability, receipt, and trace. " +
		"Omit for greet and whois."
	networkSendThreadDescription = "Required with surface=thread. Use a fresh valid ID. " +
		"The first valid send creates the public thread; later sends reuse the same ID."
	networkSendDirectDescription = "Required with surface=direct. " +
		"Resolve the existing room first with network_direct_resolve."
	networkSendWorkDescription = "Required for capability, receipt, and trace; " +
		"optional for lifecycle-bearing say messages in the same conversation."
	networkSendTargetDescription = "Target peer. Required for capability and for say carrying work_id."
	networkSendBodyDescription   = "Kind-specific object. say requires a non-empty text field; " +
		"other kinds follow the AGH Network message body rules."
)

const networkSendInputSchema = `{
	"type":"object",
	"description":"` + networkSendSchemaDescription + `",
	"required":["workspace_id","channel","kind","body"],
	"properties":{
		"workspace_id":{"type":"string"},
		"session_id":{
			"type":"string",
			"description":"` + networkSendSessionDescription + `"
		},
		"channel":{"type":"string"},
		"kind":{
			"type":"string",
			"enum":["greet","whois","say","capability","receipt","trace"]
		},
		"surface":{
			"type":"string",
			"enum":["thread","direct"],
			"description":"` + networkSendSurfaceDescription + `"
		},
		"thread_id":{
			"type":"string",
			"pattern":"` + networkpkg.ThreadIDPattern + `",
			"description":"` + networkSendThreadDescription + `"
		},
		"direct_id":{
			"type":"string",
			"pattern":"` + networkpkg.DirectIDPattern + `",
			"description":"` + networkSendDirectDescription + `"
		},
		"work_id":{"type":"string","description":"` + networkSendWorkDescription + `"},
		"to":{"type":"string","description":"` + networkSendTargetDescription + `"},
		"mentions":{"type":"array","items":{"type":"string"}},
		"body":{"type":"object","description":"` + networkSendBodyDescription + `"},
		"reply_to":{"type":"string"},
		"trace_id":{"type":"string"},
		"causation_id":{"type":"string"},
		"expires_at":{"type":"integer"},
		"id":{"type":"string"},
		"ext":{"type":"object"}
	},
	"additionalProperties":false
}`
