package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

func sessionProcessDescriptors() []toolspkg.Descriptor {
	list := nativeDescriptor(
		toolspkg.ToolIDSessionProcessesList,
		"session_processes_list",
		"Session Processes List",
		"List processes attributed to a same-workspace session. Defaults to the caller. "+
			"Includes background processes, active tools and native terminals; supported=false on unsupported hosts.",
		sessionProcessesListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{"sessions", "processes"},
		[]string{"left running", "background processes", "list ports"},
	)
	list.OutputSchema = json.RawMessage(sessionProcessesOutputSchema)
	list.ConcurrencySafe = true
	terminate := nativeDescriptor(
		toolspkg.ToolIDSessionProcessesTerminate,
		"session_processes_terminate",
		"Session Processes Terminate",
		"Terminate selected session-owned processes after exact PID/start-time verification. "+
			"Provide targets returned by list or all:true. "+
			"Uses TERM, a 3-second grace, then identity-checked KILL; native terminals use their manager. "+
			"Never stops the agent leader or daemon. Subject to destructive-tool approval policy.",
		sessionProcessesTerminateInputSchema,
		toolspkg.RiskDestructive,
		false,
		true,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDSessions},
		[]string{"sessions", "processes"},
		[]string{"terminate processes", "stop dev server", "clean up background"},
	)
	terminate.OutputSchema = json.RawMessage(sessionProcessesTerminateOutputSchema)
	return []toolspkg.Descriptor{list, terminate}
}

const sessionProcessPolicySchema = `{"type":"string","enum":["keep","terminate"],"description":"Keep (default) or terminate attributed session processes."}`

const sessionProcessesListInputSchema = `{"type":"object","properties":{"session_id":{"type":"string","minLength":1}},"additionalProperties":false}`
const sessionProcessesTerminateInputSchema = `{
  "type": "object",
  "properties": {
    "session_id": {
      "type": "string",
      "minLength": 1
    },
    "all": {
      "type": "boolean"
    },
    "targets": {
      "type": "array",
      "minItems": 1,
      "maxItems": 100,
      "items": {
        "type": "object",
        "required": [
          "pid",
          "started_at"
        ],
        "properties": {
          "pid": {
            "type": "integer",
            "minimum": 2
          },
          "started_at": {
            "type": "string",
            "format": "date-time"
          }
        },
        "additionalProperties": false
      }
    }
  },
  "additionalProperties": false
}`
const sessionProcessItemSchema = `{
  "type": "object",
  "required": [
    "pid",
    "pgid",
    "started_at",
    "kind",
    "attribution",
    "command",
    "argv0",
    "cwd",
    "ports",
    "age_seconds",
    "parent_alive"
  ],
  "properties": {
    "pid": {
      "type": "integer"
    },
    "pgid": {
      "type": "integer"
    },
    "started_at": {
      "type": "string",
      "format": "date-time"
    },
    "kind": {
      "enum": [
        "background",
        "tool",
        "terminal"
      ]
    },
    "attribution": {
      "enum": [
        "env",
        "lineage"
      ]
    },
    "command": {
      "type": "string"
    },
    "argv0": {
      "type": "string"
    },
    "cwd": {
      "type": "string"
    },
    "ports": {
      "type": "array",
      "items": {
        "type": "integer"
      }
    },
    "age_seconds": {
      "type": "number"
    },
    "parent_alive": {
      "type": "boolean"
    },
    "terminal_id": {
      "type": "string"
    },
    "cpu_percent": {
      "type": "number"
    },
    "rss_kb": {
      "type": "integer"
    }
  }
}`

const sessionProcessesOutputSchema = `{"type":"object","required":["supported","scanned_at","items"],"properties":{"supported":{"type":"boolean"},"scanned_at":{"type":"string","format":"date-time"},"items":{"type":"array","items":` + sessionProcessItemSchema + `}}}`

const sessionProcessesTerminateOutputSchema = `{"type":"object","required":["supported","results"],"properties":{"supported":{"type":"boolean"},"results":{"type":"array","items":{"type":"object","required":["pid","started_at","outcome"],"properties":{"pid":{"type":"integer"},"started_at":{"type":"string","format":"date-time"},"outcome":{"enum":["terminated","already_exited","identity_mismatch","failed","unproven"]},"error":{"type":"string"}}}}}}`
