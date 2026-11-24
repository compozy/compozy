package builtin

import (
	"strings"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func controllerComputerDescriptors() []toolspkg.Descriptor {
	return []toolspkg.Descriptor{
		controllerComputerDescriptor(
			toolspkg.ToolIDComputerList,
			"List computers",
			"List added computers on this controller and its local execution settings.",
			emptyInputSchema,
			`{"type":"object","required":["computers","local"],"properties":{"computers":{"type":"array","items":`+computerStatusOutput+`},"local":{"type":"object","required":["run_agents","load_balancing","load_balancing_weight"],"properties":{"run_agents":{"type":"boolean"},"load_balancing":{"type":"boolean"},"load_balancing_weight":{"type":"integer"}}}}}`,
			toolspkg.RiskRead,
		),
		controllerComputerDescriptor(
			toolspkg.ToolIDComputerStatus,
			"Inspect a computer",
			"Read a computer's routes, scopes and current connection status.",
			`{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1}}}`,
			computerStatusOutput,
			toolspkg.RiskRead,
		),
		controllerComputerDescriptor(
			toolspkg.ToolIDComputerAdd,
			"Add a computer",
			"Start an SSH setup or add a computer already linked to the account.",
			computerAddInput,
			`{"oneOf":[`+computerStatusOutput+`,`+computerSetupOutput+`]}`,
			toolspkg.RiskOpenWorld,
		),
		controllerComputerDescriptor(
			toolspkg.ToolIDComputerRemove,
			"Remove a computer",
			"Remove a computer from this controller and optionally unlink it from the account.",
			`{"type":"object","additionalProperties":false,"required":["id"],"properties":{"id":{"type":"string","minLength":1},"unlink":{"type":"boolean"}}}`,
			`{"type":"object","additionalProperties":false,"required":["removed"],"properties":{"removed":{"type":"boolean"}}}`,
			toolspkg.RiskDestructive,
		),
		controllerComputerDescriptor(
			toolspkg.ToolIDComputerSetupStatus,
			"Read computer setup",
			"Read SSH setup progress or wait for a change for up to 30 seconds.",
			`{"type":"object","additionalProperties":false,"required":["setup_id"],"properties":{"setup_id":{"type":"string","minLength":1},"wait_ms":{"type":"integer","minimum":0,"maximum":30000}}}`,
			computerSetupOutput,
			toolspkg.RiskRead,
		),
		controllerComputerDescriptor(
			toolspkg.ToolIDComputerSetupAnswer,
			"Answer SSH host trust",
			"Accept or reject the SSH host key shown by computer setup. The operator enters secrets in the app or CLI.",
			`{"type":"object","additionalProperties":false,"required":["setup_id","accept"],"properties":{"setup_id":{"type":"string","minLength":1},"accept":{"type":"boolean"}}}`,
			emptyInputSchema,
			toolspkg.RiskMutating,
		),
		controllerComputerDescriptor(
			toolspkg.ToolIDAccountStatus,
			"Read account status",
			"Read the local sign-in state, linked device and plan.",
			emptyInputSchema,
			accountStatusOutput,
			toolspkg.RiskRead,
		),
	}
}

func controllerComputerDescriptor(
	id toolspkg.ToolID,
	title, description, input, output string,
	risk toolspkg.RiskClass,
) toolspkg.Descriptor {
	descriptor := nativeDescriptor(id, strings.TrimPrefix(id.String(), "compozy__"), title, description, input, risk,
		risk == toolspkg.RiskRead, risk == toolspkg.RiskDestructive, risk == toolspkg.RiskOpenWorld,
		nil, []string{"computer", "remote access"}, []string{title})
	descriptor.OutputSchema = []byte(output)
	return descriptor
}

const computerAddInput = `{"type":"object","additionalProperties":false,"required":["kind"],"properties":{
	"kind":{"enum":["ssh","account"]},"id":{"type":"string","minLength":1},"host":{"type":"string","minLength":1},
	"user":{"type":"string"},"port":{"type":"integer","minimum":1,"maximum":65535},"name":{"type":"string"},
	"trust_host_fingerprint":{"type":"string"}},"oneOf":[
		{"properties":{"kind":{"const":"ssh"}},"required":["host"]},
		{"properties":{"kind":{"const":"account"}},"required":["id"]}]}`

const computerStatusOutput = `{"type":"object",
	"required":["id","name","status","enabled","load_balancing_weight","routes","scopes"],
	"properties":{"id":{"type":"string"},"name":{"type":"string"},"status":{"type":"string"},"enabled":{"type":"boolean"},
	"load_balancing_weight":{"type":"integer"},"routes":{"type":"array","items":{"type":"object"}},
	"scopes":{"type":["array","null"],"items":{"type":"string"}}}}`

const computerSetupOutput = `{"type":"object","required":["setup_id","step","state","prompt","error","computer_id"],
	"properties":{"setup_id":{"type":"string"},"step":{"type":"string"},"state":{"type":"string"},
	"prompt":{"type":["object","null"]},"prompt_kind":{"type":"string"},"error":{"type":["object","null"]},
	"computer_id":{"type":["string","null"]}}}`

const accountStatusOutput = `{"type":"object",
	"required":["remote_computers_used","remote_computers_limit","cloud_routes_used","cloud_routes_limit","device"],
	"properties":{
	"state":{"type":"string"},"plan":{"type":"string"},"email":{"type":"string"},
	"remote_computers_used":{"type":"integer"},"remote_computers_limit":{"type":["integer","null"]},
	"cloud_routes_used":{"type":"integer"},"cloud_routes_limit":{"type":"integer"},
	"device":{"type":["object","null"]}}}`
