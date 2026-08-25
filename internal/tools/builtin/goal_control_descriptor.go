package builtin

import (
	"encoding/json"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

var goalControlDescriptor = func() toolspkg.Descriptor {
	descriptor := nativeLoopDescriptor(
		toolspkg.ToolIDGoalControl,
		"goal_control",
		"Goal Control",
		"Apply one authenticated structured Goal operation to a caller session or its child session.",
		goalControlInputSchema,
		toolspkg.RiskMutating,
		false,
		false,
		[]string{goalKey, descriptorKeywordUpdate, descriptorKeywordSession},
		[]string{"set goal", "replace goal", "pause goal", "resume goal", "clear goal"},
	)
	descriptor.OutputSchema = json.RawMessage(goalControlOutputSchema)
	return descriptor
}()

const goalControlInputSchema = `{
	"type":"object",
	"required":["operation","session_id"],
	"additionalProperties":false,
	"properties":{
		"workspace":{"type":"string"},
		"session_id":{"type":"string","minLength":1},
		"operation":{"type":"string","enum":["set","replace","status","pause","resume","clear"]},
		"objective":{"type":"string"},
		"expected_run_id":{"type":"string"},
		"runtime":{
			"type":"object",
			"additionalProperties":false,
			"required":["provider"],
			"properties":{
				"provider":{"type":"string","minLength":1},
				"model":{"type":"string"},
				"reasoning_effort":{"type":"string","enum":["none","minimal","low","medium","high","xhigh","max"]},
				"speed":{"type":"string","enum":["normal","fast"]}
			}
		}
	}
}`

const goalControlOutputSchema = `{
	"type":"object",
	"required":["outcome","reason_code","snapshot","replaced_run_id"],
	"additionalProperties":false,
	"properties":{
		"outcome":{"type":"string","enum":["started","replaced","status","paused","resumed","cleared","error"]},
		"reason_code":{"type":["string","null"]},
		"snapshot":` + goalControlSnapshotSchema + `,
		"replaced_run_id":{"type":["string","null"]}
	}
}`

const goalControlSnapshotSchema = `{
	"type":["object","null"],
	"required":["run_id","node_id","objective","origin_session_id",` +
	`"bound_session_id","status","run_status","cause","turns_used","turn_limit",` +
	`"live","contract_summary","last_verdict","context"],
	"additionalProperties":false,
	"properties":{
		"run_id":{"type":"string"},
		"node_id":{"type":"string"},
		"objective":{"type":"string"},
		"origin_session_id":{"type":"string"},
		"bound_session_id":{"type":"string"},
		"status":{"type":"string","enum":["active","paused","blocked","usage-limited","budget-limited","complete"]},
		"run_status":{"type":"string","enum":["queued","running","watching",` +
	`"needs-approval","paused","done","no-op","blocked","failed","exhausted",` +
	`"stalled","canceled"]},
		"cause":{"type":["string","null"]},
		"turns_used":{"type":"integer","minimum":0},
		"turn_limit":{"type":"integer","minimum":0},
		"live":{"type":"boolean"},
		"contract_summary":{"type":"string"},
		"last_verdict":{
			"type":["object","null"],
			"required":["outcome","blocking_issues","evidence_ref","evaluated_at"],
			"additionalProperties":false,
			"properties":{
				"outcome":{"type":"string","enum":["approved","rejected","awaiting_approval","blocked","error","timeout",` +
	`"invalid_output"]},
				"blocking_issues":{"type":"array","items":{"type":"object","required":["id","note"],` +
	`"additionalProperties":false,"properties":{"id":{"type":"string"},"note":{"type":"string"}}}},
				"evidence_ref":{"type":["string","null"]},
				"evaluated_at":{"type":"string","format":"date-time"}
			}
		},
		"context":{
			"type":"object",
			"required":["state","used","size","ratio","nudge_ratio","reported_at"],
			"additionalProperties":false,
			"properties":{
				"state":{"type":"string","enum":["known","unknown","pending"]},
				"used":{"type":["integer","null"]},
				"size":{"type":["integer","null"]},
				"ratio":{"type":["number","null"]},
				"nudge_ratio":{"type":"number"},
				"reported_at":{"type":["string","null"],"format":"date-time"}
			}
		}
	}
}`
