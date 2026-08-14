package speccycle

import "encoding/json"

var (
	writeArtifactsInputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["task_name","issues"],
		"properties":{
			"task_name":{"type":"string","minLength":1},
			"issues":{"type":"array","items":{"$ref":"#/$defs/review_issue"}}
		},
		"$defs":{"review_issue":{
			"type":"object",
			"additionalProperties":false,
			"required":["title","body","severity"],
			"properties":{
				"title":{"type":"string","minLength":1},
				"body":{"type":"string","minLength":1},
				"severity":{"type":"string","minLength":1},
				"file":{"type":"string"},
				"line":{"type":"integer","minimum":1},
				"author":{"type":"string"}
			}
		}}
	}`)
	writeArtifactsOutputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["task_name","round","files","batches","issues","count"],
		"properties":{
			"task_name":{"type":"string"},
			"round":{"type":"integer","minimum":0},
			"round_dir":{"type":"string"},
			"files":{"type":"array","items":{
				"type":"object","additionalProperties":false,"required":["path","file"],
				"properties":{"path":{"type":"string"},"file":{"type":"string"}}
			}},
			"batches":{"type":"array","items":{
				"type":"object","additionalProperties":false,"required":["file","issue_files"],
				"properties":{"file":{"type":"string"},"issue_files":{"type":"array","items":{"type":"string"}}}
			}},
			"issues":{"type":"array","items":{"$ref":"#/$defs/review_issue"}},
			"count":{"type":"integer","minimum":0}
		},
		"$defs":{"review_issue":{
			"type":"object",
			"additionalProperties":false,
			"required":["title","body","severity"],
			"properties":{
				"title":{"type":"string"},
				"body":{"type":"string"},
				"severity":{"type":"string"},
				"file":{"type":"string"},
				"line":{"type":"integer"},
				"author":{"type":"string"}
			}
		}}
	}`)
	finalizeRoundInputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["task_name","round"],
		"properties":{
			"task_name":{"type":"string","minLength":1},
			"round":{"type":"integer","minimum":1}
		}
	}`)
	finalizeRoundOutputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["task_name","round","resolved","invalid","pending"],
		"properties":{
			"task_name":{"type":"string"},
			"round":{"type":"integer","minimum":1},
			"resolved":{"type":"integer","minimum":0},
			"invalid":{"type":"integer","minimum":0},
			"pending":{"type":"integer","minimum":0}
		}
	}`)
)
