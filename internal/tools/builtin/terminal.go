package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	terminalObserveCapability = "terminal.observe"
	terminalExecCapability    = "terminal.exec"
	terminalTag               = "terminal"
)

type terminalDescriptorSpec struct {
	id           toolspkg.ToolID
	nativeName   string
	title        string
	description  string
	inputSchema  string
	outputSchema string
	risk         toolspkg.RiskClass
	readOnly     bool
	destructive  bool
	openWorld    bool
	concurrent   bool
	capability   string
}

var terminalToolSpecs = []terminalDescriptorSpec{
	{id: toolspkg.ToolIDTerminalExec, nativeName: "terminal_exec", title: "Terminal Exec", description: "Execute one supervised command with bounded output and honest yield semantics.", inputSchema: terminalExecInputSchema, outputSchema: terminalExecOutputSchema, risk: toolspkg.RiskDestructive, destructive: true, openWorld: true, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalOpen, nativeName: "terminal_open", title: "Terminal Open", description: "Open one visible interactive terminal in the acting workspace.", inputSchema: terminalOpenInputSchema, outputSchema: terminalIDOutputSchema, risk: toolspkg.RiskMutating, openWorld: true, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalWrite, nativeName: "terminal_write", title: "Terminal Write", description: "Deliver approved input to a terminal controlled by the acting agent.", inputSchema: terminalWriteInputSchema, outputSchema: terminalWriteOutputSchema, risk: toolspkg.RiskDestructive, destructive: true, openWorld: true, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalRead, nativeName: "terminal_read", title: "Terminal Read", description: "Read bounded untrusted terminal output.", inputSchema: terminalReadInputSchema, outputSchema: terminalReadOutputSchema, risk: toolspkg.RiskRead, readOnly: true, concurrent: true, capability: terminalObserveCapability},
	{id: toolspkg.ToolIDTerminalWait, nativeName: "terminal_wait", title: "Terminal Wait", description: "Wait for an observable terminal condition without guessing completion.", inputSchema: terminalWaitInputSchema, outputSchema: terminalWaitOutputSchema, risk: toolspkg.RiskRead, readOnly: true, concurrent: true, capability: terminalObserveCapability},
	{id: toolspkg.ToolIDTerminalSignal, nativeName: "terminal_signal", title: "Terminal Signal", description: "Signal a terminal owned by the acting run.", inputSchema: terminalSignalInputSchema, outputSchema: deliveredOutputSchema, risk: toolspkg.RiskMutating, openWorld: true, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalClose, nativeName: "terminal_close", title: "Terminal Close", description: "Close a terminal owned by the acting run.", inputSchema: terminalIDInputSchema, outputSchema: terminalCloseOutputSchema, risk: toolspkg.RiskDestructive, destructive: true, openWorld: true, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalList, nativeName: "terminal_list", title: "Terminal List", description: "List terminals visible to the acting profile.", inputSchema: emptyInputSchema, outputSchema: terminalListOutputSchema, risk: toolspkg.RiskRead, readOnly: true, concurrent: true, capability: terminalObserveCapability},
	{id: toolspkg.ToolIDTerminalRequestInput, nativeName: "terminal_request_input", title: "Terminal Request Input", description: "Request human input without exposing the answer to the agent.", inputSchema: terminalRequestInputSchema, outputSchema: terminalInputOutcomeSchema, risk: toolspkg.RiskMutating, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalYield, nativeName: "terminal_yield", title: "Terminal Yield", description: "Yield terminal control to the operator.", inputSchema: terminalYieldInputSchema, outputSchema: terminalLeaseOutputSchema, risk: toolspkg.RiskMutating, capability: terminalExecCapability},
	{id: toolspkg.ToolIDTerminalClaim, nativeName: "terminal_claim", title: "Terminal Claim", description: "Claim an available terminal without displacing a human controller.", inputSchema: terminalIDInputSchema, outputSchema: terminalLeaseOutputSchema, risk: toolspkg.RiskMutating, capability: terminalExecCapability},
}

func terminalDescriptors() []toolspkg.Descriptor {
	descriptors := make([]toolspkg.Descriptor, 0, len(terminalToolSpecs))
	for _, spec := range terminalToolSpecs {
		descriptor := nativeDescriptor(
			spec.id, spec.nativeName, spec.title, spec.description, spec.inputSchema,
			spec.risk, spec.readOnly, spec.destructive, spec.openWorld,
			[]toolspkg.ToolsetID{toolspkg.ToolsetIDTerminal}, []string{terminalTag},
			[]string{spec.title, spec.description},
		)
		descriptor.OutputSchema = json.RawMessage(spec.outputSchema)
		descriptor.ConcurrencySafe = spec.concurrent
		descriptors = append(descriptors, withRequiredCapabilities(descriptor, spec.capability))
	}
	return descriptors
}

func terminalToolset() toolspkg.Toolset {
	tools := make([]string, 0, len(terminalToolSpecs))
	for _, spec := range terminalToolSpecs {
		tools = append(tools, spec.id.String())
	}
	return toolspkg.Toolset{ID: toolspkg.ToolsetIDTerminal, Tools: tools}
}

const terminalIDPropertySchema = `{"type":"string","pattern":"^term-[0-9a-f]{12}$"}`
const terminalIDInputSchema = `{"type":"object","required":["terminal_id"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `}}`
const terminalIDOutputSchema = `{"type":"object","required":["terminal_id"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `}}`
const terminalOpenInputSchema = `{"type":"object","required":["title"],"additionalProperties":false,"properties":{"cwd":{"type":"string"},"shell":{"type":"string"},"cols":{"type":"integer","minimum":20,"maximum":2000},"rows":{"type":"integer","minimum":5,"maximum":1000},"title":{"type":"string","minLength":1,"maxLength":256}}}`
const terminalExecInputSchema = `{"type":"object","required":["command"],"additionalProperties":false,"properties":{"command":{"type":"string","minLength":1},"args":{"type":"array","items":{"type":"string"}},"cwd":{"type":"string"},"env":{"type":"object","additionalProperties":{"type":"string"}},"yield_ms":{"type":"integer","minimum":250,"maximum":30000},"visible":{"type":"boolean"},"output":{"type":"object","additionalProperties":false,"properties":{"max_bytes":{"type":"integer","minimum":1},"strategy":{"enum":["tail","head_tail"]},"grep":{"type":"string"}}}}}`
const terminalExecOutputSchema = `{"type":"object","required":["untrusted","command_id","terminal_id"],"properties":{"exit_code":{"type":["integer","null"]},"signal":{"type":["string","null"]},"output":{"type":"string"},"truncated":{"type":"boolean"},"untrusted":{"const":true},"duration_ms":{"type":"integer"},"command_id":{"type":"string"},"still_running":{"type":"boolean"},"terminal_id":{"type":["string","null"]},"spill":{"type":"object"}}}`
const terminalWriteInputSchema = `{"type":"object","required":["terminal_id"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `,"data":{"type":"string"}}}`
const terminalWriteOutputSchema = `{"type":"object","required":["accepted","lease_state"],"properties":{"accepted":{"type":"boolean"},"lease_state":{"enum":["agent_owned","human_owned","available"]},"content":{"type":"string"},"seq":{"type":"integer"},"busy":{"type":"boolean"}}}`
const terminalReadInputSchema = `{"type":"object","required":["terminal_id","view"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `,"view":{"enum":["screen","tail","lines"]},"max_bytes":{"type":"integer","minimum":1},"since_seq":{"type":"integer","minimum":0},"from":{"type":"integer","minimum":0},"to":{"type":"integer","minimum":0},"grep":{"type":"string"}}}`
const terminalReadOutputSchema = `{"type":"object","required":["content","seq","truncated","busy","untrusted"],"properties":{"content":{"type":"string"},"seq":{"type":"integer"},"truncated":{"type":"boolean"},"busy":{"type":"boolean"},"untrusted":{"const":true}}}`
const terminalWaitInputSchema = `{"type":"object","required":["terminal_id","until"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `,"until":{"enum":["exit","idle","match"]},"pattern":{"type":"string"},"timeout_ms":{"type":"integer","minimum":0,"maximum":60000}}}`
const terminalWaitOutputSchema = `{"type":"object","required":["reason","screen","untrusted"],"properties":{"reason":{"enum":["exit","match","idle","timeout","still_running","stalled"]},"exit_code":{"type":"integer"},"screen":{"type":"string"},"untrusted":{"const":true}}}`
const terminalSignalInputSchema = `{"type":"object","required":["terminal_id","signal"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `,"signal":{"enum":["INT","TERM","KILL","HUP"]}}}`
const deliveredOutputSchema = `{"type":"object","required":["delivered"],"additionalProperties":false,"properties":{"delivered":{"type":"boolean"}}}`
const terminalCloseOutputSchema = `{"type":"object","required":["exit"],"additionalProperties":false,"properties":{"exit":{"type":"object","required":["cause","at"],"additionalProperties":false,"properties":{"cause":{"type":"string"},"code":{"type":["integer","null"]},"signal":{"type":["string","null"]},"at":{"type":"string","format":"date-time"}}}}}`
const terminalListOutputSchema = `{"type":"object","required":["terminals"],"additionalProperties":false,"properties":{"terminals":{"type":"array","items":{"type":"object","required":["id","workspace_id","profile_id","profile_name","title","shell","cwd","mode","state","controller","lease","viewers","bound_run","capabilities","created_at"],"additionalProperties":false,"properties":{"id":{"type":"string"},"workspace_id":{"type":"string"},"profile_id":{"type":"string"},"profile_name":{"type":"string"},"title":{"type":"string"},"shell":{"type":"string"},"cwd":{"type":"string"},"mode":{"enum":["pty","pipe"]},"state":{"type":"string"},"controller":{"type":["object","null"],"required":["kind","id"],"additionalProperties":false,"properties":{"kind":{"type":"string"},"id":{"type":"string"}}},"lease":{"enum":["agent_owned","human_owned","available"]},"viewers":{"type":"integer"},"bound_run":{"type":["object","null"],"required":["session_id","run_id"],"additionalProperties":false,"properties":{"session_id":{"type":"string"},"run_id":{"type":"string"}}},"capabilities":{"type":"object","required":["interactive"],"additionalProperties":false,"properties":{"interactive":{"type":"boolean"}}},"created_at":{"type":"string","format":"date-time"},"exit":{"type":"object","required":["cause","at"],"additionalProperties":false,"properties":{"cause":{"type":"string"},"code":{"type":"integer"},"signal":{"type":"string"},"at":{"type":"string","format":"date-time"}}}}}}}}`
const terminalRequestInputSchema = `{"type":"object","required":["terminal_id","reason","prompt_excerpt"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `,"reason":{"type":"string"},"prompt_excerpt":{"type":"string"},"redact":{"type":"boolean"}}}`
const terminalInputOutcomeSchema = `{"type":"object","required":["outcome","redacted","length"],"additionalProperties":false,"properties":{"outcome":{"enum":["answered","rejected","superseded","expired"]},"redacted":{"type":"boolean"},"length":{"type":"integer"}}}`
const terminalYieldInputSchema = `{"type":"object","required":["terminal_id","reason"],"additionalProperties":false,"properties":{"terminal_id":` + terminalIDPropertySchema + `,"reason":{"type":"string"}}}`
const terminalLeaseOutputSchema = `{"type":"object","required":["lease_state"],"additionalProperties":false,"properties":{"granted":{"type":"boolean"},"lease_state":{"enum":["agent_owned","human_owned","available"]},"controller":{"type":["object","null"],"additionalProperties":false,"required":["kind","id","profile_id"],"properties":{"kind":{"type":"string"},"id":{"type":"string"},"profile_id":{"type":"string"},"session_id":{"type":"string"},"run_id":{"type":"string"},"generation":{"type":"integer"}}}}}`
