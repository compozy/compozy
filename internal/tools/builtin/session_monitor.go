package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	monitorKeyword = "monitor"
	wakeKeyword    = "wake"
)

func sessionMonitorDescriptors() []toolspkg.Descriptor {
	create := nativeDescriptor(
		toolspkg.ToolIDMonitorCreate,
		"monitor_create",
		"Monitor Create",
		"Register a daemon-owned monitor that wakes this session with a new turn when the condition fires. Use it "+
			"for every wait on external work, inside or outside your turn: CI, deploys, builds, polling remote state, "+
			"or a delay. kind=terminal watches a terminal_id from compozy__terminal_exec (still_running) or "+
			"compozy__terminal_open until it exits or its output matches a pattern; kind=timer fires once after "+
			"delay_seconds; kind=session watches other sessions of this workspace and profile in "+
			"target_session_ids until any or "+
			"all of them reach states (one wake with each session's state; failed, stopped, or deleted sessions "+
			"always count). Sessions with queued input are still working. A target is read as it is now, and "+
			"a condition that already holds fires at once. "+
			"Use it instead of shell watches such as gh pr checks --watch, sleep loops, polling, or "+
			"provider-internal background tasks, Monitor, and ScheduleWakeup — those cannot wake a CompozyOS session. "+
			"After registering, keep working on steps that do not depend on it; end your turn only when every "+
			"remaining step waits on a registered monitor or subagent. The wake arrives as a "+
			"[COMPOZY MONITOR - NOT USER INPUT] message. If the session is busy the wake queues behind the current turn. "+
			"Remote state such as CI has no polling kind: run its bounded watch command with compozy__terminal_exec, "+
			"monitor that terminal, and set timeout_seconds past the expected wait. Registering the same terminal "+
			"request (including timeout and description) again returns the existing monitor instead of adding a second wake.",
		monitorCreateInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, monitorKeyword, wakeKeyword},
		[]string{
			"monitor background command",
			"wait for CI",
			"watch pr checks",
			"wait for deploy",
			"wake me when done",
			"watch terminal",
			"schedule wakeup",
			"notify on exit",
			"wait for sessions",
			"wake when all sessions finish",
		},
	)
	create.OutputSchema = json.RawMessage(monitorOutputSchema)

	list := nativeDescriptor(
		toolspkg.ToolIDMonitorList,
		"monitor_list",
		"Monitor List",
		"List this session's monitors: active ones first, then delivering (wake retrying a full input queue), "+
			"fired, canceled, undelivered, or interrupted (a daemon restart ended the watch and its owner could not "+
			"be resumed) ones from the last hour.",
		emptyInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, monitorKeyword},
		[]string{"list monitors", "pending wakeups"},
	)
	list.OutputSchema = json.RawMessage(monitorListOutputSchema)
	list.ConcurrencySafe = true

	cancel := nativeDescriptor(
		toolspkg.ToolIDMonitorCancel,
		"monitor_cancel",
		"Monitor Cancel",
		"Cancel one active monitor owned by this session so it never wakes the session. "+
			"It does not stop the watched terminal process.",
		monitorCancelInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{sessionsSessionsKey, monitorKeyword},
		[]string{"cancel monitor", "cancel wakeup"},
	)
	cancel.OutputSchema = json.RawMessage(monitorOutputSchema)
	return []toolspkg.Descriptor{create, list, cancel}
}

const monitorCreateInputSchema = `{
	"type":"object",
	"required":["kind","description"],
	"properties":{
		"kind":{"type":"string","enum":["terminal","timer","session"]},
		"description":{"type":"string","minLength":1,"maxLength":200,
			"description":"What this monitor waits for; echoed back in the wake."},
		"terminal_id":` + terminalIDPropertySchema + `,
		"until":{"type":"string","enum":["exit","match","any","all"],
			"description":"Terminal: exit (default) or match. Session: all (default) or any."},
		"pattern":{"type":"string","minLength":1,"maxLength":512,"description":"Go regular expression for until=match."},
		"timeout_seconds":{"type":"integer","minimum":1,"maximum":21600,
			"description":"Defaults to 1800 for terminal and 7200 for session. A timeout still wakes the session."},
		"delay_seconds":{"type":"integer","minimum":5,"maximum":86400,"description":"Timer monitors only."},
		"target_session_ids":{"type":"array","minItems":1,"maxItems":32,
			"items":{"type":"string","minLength":1},
			"description":"Session monitors only: other sessions of this workspace and profile to watch."},
		"states":{"type":"array","minItems":1,"uniqueItems":true,
			"items":{"type":"string","enum":["idle","waiting-for-input","waiting-for-auth","needs-attention",
				"hung","stopped","failed"]},
			"description":"Session monitors only: states that satisfy a target. Omit for all but hung."}
	},
	"additionalProperties":false
}`

const monitorCancelInputSchema = `{
	"type":"object",
	"required":["monitor_id"],
	"properties":{"monitor_id":{"type":"string","pattern":"^mon-[0-9a-f]{12}$"}},
	"additionalProperties":false
}`

const monitorItemSchema = `{
	"type":"object",
	"required":["monitor_id","session_id","kind","description","status","created_at","deadline_at"],
	"properties":{
		"monitor_id":{"type":"string"},
		"session_id":{"type":"string"},
		"kind":{"type":"string","enum":["terminal","timer","session"]},
		"description":{"type":"string"},
		"terminal_id":{"type":"string"},
		"until":{"type":"string"},
		"pattern":{"type":"string"},
		"status":{"type":"string","enum":["active","delivering","fired","canceled","undelivered","interrupted"]},
		"created_at":{"type":"string","format":"date-time"},
		"deadline_at":{"type":"string","format":"date-time"},
		"fired_at":{"type":"string","format":"date-time"},
		"reason":{"type":"string"},
		"exit_code":{"type":"integer"},
		"target_session_ids":{"type":"array","items":{"type":"string"}},
		"states":{"type":"array","items":{"type":"string"}},
		"targets":{"type":"array","items":{
			"type":"object",
			"required":["session_id","state","matched"],
			"properties":{
				"session_id":{"type":"string"},
				"name":{"type":"string"},
				"state":{"type":"string"},
				"matched":{"type":"boolean"}
			}
		}}
	}
}`

const monitorOutputSchema = `{
	"type":"object",
	"required":["monitor"],
	"properties":{"monitor":` + monitorItemSchema + `}
}`

const monitorListOutputSchema = `{
	"type":"object",
	"required":["monitors"],
	"properties":{"monitors":{"type":"array","items":` + monitorItemSchema + `}}
}`
