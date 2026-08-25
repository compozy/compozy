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
	{toolspkg.ToolIDTerminalExec, "terminal_exec", "Terminal Exec", "Execute one supervised command with bounded output and honest yield semantics.", terminalExecInputSchema, terminalExecOutputSchema, toolspkg.RiskDestructive, false, true, true, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalOpen, "terminal_open", "Terminal Open", "Open one visible interactive terminal in the acting workspace.", terminalOpenInputSchema, terminalIDOutputSchema, toolspkg.RiskMutating, false, false, true, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalWrite, "terminal_write", "Terminal Write", "Deliver approved input to a terminal controlled by the acting agent.", terminalWriteInputSchema, terminalWriteOutputSchema, toolspkg.RiskDestructive, false, true, true, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalRead, "terminal_read", "Terminal Read", "Read bounded untrusted terminal output.", terminalReadInputSchema, terminalReadOutputSchema, toolspkg.RiskRead, true, false, false, true, terminalObserveCapability},
	{toolspkg.ToolIDTerminalWait, "terminal_wait", "Terminal Wait", "Wait for an observable terminal condition without guessing completion.", terminalWaitInputSchema, terminalWaitOutputSchema, toolspkg.RiskRead, true, false, false, true, terminalObserveCapability},
	{toolspkg.ToolIDTerminalSignal, "terminal_signal", "Terminal Signal", "Signal a terminal owned by the acting run.", terminalSignalInputSchema, deliveredOutputSchema, toolspkg.RiskMutating, false, false, true, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalClose, "terminal_close", "Terminal Close", "Close a terminal owned by the acting run.", terminalIDInputSchema, terminalCloseOutputSchema, toolspkg.RiskDestructive, false, true, true, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalList, "terminal_list", "Terminal List", "List terminals visible to the acting profile.", emptyInputSchema, terminalListOutputSchema, toolspkg.RiskRead, true, false, false, true, terminalObserveCapability},
	{toolspkg.ToolIDTerminalRequestInput, "terminal_request_input", "Terminal Request Input", "Request human input without exposing the answer to the agent.", terminalRequestInputSchema, terminalInputOutcomeSchema, toolspkg.RiskMutating, false, false, false, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalYield, "terminal_yield", "Terminal Yield", "Yield terminal control to the operator.", terminalYieldInputSchema, terminalLeaseOutputSchema, toolspkg.RiskMutating, false, false, false, false, terminalExecCapability},
	{toolspkg.ToolIDTerminalClaim, "terminal_claim", "Terminal Claim", "Claim an available terminal without displacing a human controller.", terminalIDInputSchema, terminalLeaseOutputSchema, toolspkg.RiskMutating, false, false, false, false, terminalExecCapability},
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

const terminalIDInputSchema = `{"type":"object","required":["terminal_id"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string","pattern":"^term-[0-9a-f]{12}$"}}}`
const terminalIDOutputSchema = `{"type":"object","required":["terminal_id"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"}}}`
const terminalOpenInputSchema = `{"type":"object","required":["title"],"additionalProperties":false,"properties":{"cwd":{"type":"string"},"shell":{"type":"string"},"cols":{"type":"integer","minimum":20,"maximum":2000},"rows":{"type":"integer","minimum":5,"maximum":1000},"title":{"type":"string","minLength":1,"maxLength":256}}}`
const terminalExecInputSchema = `{"type":"object","required":["command"],"additionalProperties":false,"properties":{"command":{"type":"string","minLength":1},"args":{"type":"array","items":{"type":"string"}},"cwd":{"type":"string"},"env":{"type":"object","additionalProperties":{"type":"string"}},"yield_ms":{"type":"integer","minimum":250,"maximum":30000},"visible":{"type":"boolean"},"output":{"type":"object","additionalProperties":false,"properties":{"max_bytes":{"type":"integer","minimum":1},"strategy":{"enum":["tail","head_tail"]},"grep":{"type":"string"}}}}}`
const terminalExecOutputSchema = `{"type":"object","required":["untrusted","command_id","terminal_id"],"properties":{"exit_code":{"type":["integer","null"]},"signal":{"type":["string","null"]},"output":{"type":"string"},"truncated":{"type":"boolean"},"untrusted":{"const":true},"duration_ms":{"type":"integer"},"command_id":{"type":"string"},"still_running":{"type":"boolean"},"terminal_id":{"type":["string","null"]},"spill":{"type":"object"}}}`
const terminalWriteInputSchema = `{"type":"object","required":["terminal_id"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"},"data":{"type":"string"}}}`
const terminalWriteOutputSchema = `{"type":"object","required":["accepted","lease_state"],"properties":{"accepted":{"type":"boolean"},"lease_state":{"enum":["agent_owned","human_owned","available"]},"content":{"type":"string"},"seq":{"type":"integer"},"busy":{"type":"boolean"}}}`
const terminalReadInputSchema = `{"type":"object","required":["terminal_id","view"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"},"view":{"enum":["screen","tail","lines"]},"max_bytes":{"type":"integer","minimum":1},"since_seq":{"type":"integer","minimum":0},"from":{"type":"integer","minimum":0},"to":{"type":"integer","minimum":0},"grep":{"type":"string"}}}`
const terminalReadOutputSchema = `{"type":"object","required":["content","seq","truncated","busy","untrusted"],"properties":{"content":{"type":"string"},"seq":{"type":"integer"},"truncated":{"type":"boolean"},"busy":{"type":"boolean"},"untrusted":{"const":true}}}`
const terminalWaitInputSchema = `{"type":"object","required":["terminal_id","until"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"},"until":{"enum":["exit","idle","match"]},"pattern":{"type":"string"},"timeout_ms":{"type":"integer","minimum":0,"maximum":60000}}}`
const terminalWaitOutputSchema = `{"type":"object","required":["reason","screen","untrusted"],"properties":{"reason":{"enum":["exit","match","idle","timeout","still_running","stalled"]},"exit_code":{"type":"integer"},"screen":{"type":"string"},"untrusted":{"const":true}}}`
const terminalSignalInputSchema = `{"type":"object","required":["terminal_id","signal"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"},"signal":{"enum":["INT","TERM","KILL","HUP"]}}}`
const deliveredOutputSchema = `{"type":"object","required":["delivered"],"additionalProperties":false,"properties":{"delivered":{"type":"boolean"}}}`
const terminalCloseOutputSchema = `{"type":"object","required":["exit"],"properties":{"exit":{"type":"object"}}}`
const terminalListOutputSchema = `{"type":"object","required":["terminals"],"properties":{"terminals":{"type":"array","items":{"type":"object"}}}}`
const terminalRequestInputSchema = `{"type":"object","required":["terminal_id","reason","prompt_excerpt"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"},"reason":{"type":"string"},"prompt_excerpt":{"type":"string"},"redact":{"type":"boolean"}}}`
const terminalInputOutcomeSchema = `{"type":"object","required":["outcome","redacted","length"],"properties":{"outcome":{"enum":["answered","rejected","superseded","expired"]},"redacted":{"type":"boolean"},"length":{"type":"integer"}}}`
const terminalYieldInputSchema = `{"type":"object","required":["terminal_id","reason"],"additionalProperties":false,"properties":{"terminal_id":{"type":"string"},"reason":{"type":"string"}}}`
const terminalLeaseOutputSchema = `{"type":"object","required":["lease_state"],"properties":{"granted":{"type":"boolean"},"lease_state":{"enum":["agent_owned","human_owned","available"]},"controller":{"type":"object"}}}`
