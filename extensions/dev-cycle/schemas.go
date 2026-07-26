package devcycle

import (
	"encoding/json"
	"fmt"

	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	toolImportTasks     = "import_tasks"
	toolFetchUnresolved = "coderabbit_fetch_unresolved"
	toolResolveThreads  = "coderabbit_resolve_threads"
	toolGitPush         = "git_push"
)

var (
	importTasksInputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["pattern"],
		"properties":{
			"pattern":{"type":"string","minLength":1}
		}
	}`)
	importTasksOutputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["tasks","count"],
		"properties":{
			"tasks":{
				"type":"array",
				"items":{
					"type":"object",
					"additionalProperties":false,
					"required":["id","number","title","path","body","body_ref","blocks"],
					"properties":{
						"id":{"type":"string"},
						"number":{"type":"integer"},
						"title":{"type":"string"},
						"path":{"type":"string"},
						"body":{"type":"string"},
						"body_ref":{"type":"string"},
						"blocks":{"type":"array","items":{"type":"string"}}
					}
				}
			},
			"count":{"type":"integer","minimum":0}
		}
	}`)
	fetchInputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["pr"],
		"properties":{
			"pr":{"oneOf":[{"type":"integer","minimum":1},{"type":"string","minLength":1}]},
			"include_nitpicks":{
				"oneOf":[
					{"type":"boolean","default":false},
					{"type":"string","enum":["true","false"]}
				]
			}
		}
	}`)
	fetchOutputSchema = json.RawMessage(`{
		"type":"object",
		"required":["pr","unresolved_count","issues"],
		"properties":{
			"pr":{"type":"string"},
			"unresolved_count":{"type":"integer","minimum":0},
			"issues":{"type":"array","items":{"type":"object"}}
		}
	}`)
	resolveInputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"required":["pr"],
		"properties":{
			"pr":{"oneOf":[{"type":"integer","minimum":1},{"type":"string","minLength":1}]},
			"issues":{"type":"array","items":{"type":"object"}},
			"results":{"type":"array","items":{"type":"object"}}
		}
	}`)
	resolveOutputSchema = json.RawMessage(`{
		"type":"object",
		"required":["pr","resolved_count","resolved_threads"],
		"properties":{
			"pr":{"type":"string"},
			"resolved_count":{"type":"integer","minimum":0},
			"resolved_threads":{"type":"array","items":{"type":"string"}}
		}
	}`)
	pushInputSchema = json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"properties":{
			"remote":{"type":"string","default":"origin"},
			"branch":{"type":"string"},
			"require_head_advanced":{"type":"boolean","default":false},
			"expected_head":{"type":"string"}
		}
	}`)
	pushOutputSchema = json.RawMessage(`{
		"type":"object",
		"required":["remote","branch","head"],
		"properties":{
			"remote":{"type":"string"},
			"branch":{"type":"string"},
			"head":{"type":"string"},
			"pushed":{"type":"boolean"}
		}
	}`)
)

func runtimeToolDescriptors() ([]toolspkg.ExtensionToolRuntimeDescriptor, error) {
	specs := []struct {
		handler      string
		inputSchema  json.RawMessage
		outputSchema json.RawMessage
		readOnly     bool
		risk         toolspkg.RiskClass
	}{
		{toolImportTasks, importTasksInputSchema, importTasksOutputSchema, true, toolspkg.RiskRead},
		{toolFetchUnresolved, fetchInputSchema, fetchOutputSchema, true, toolspkg.RiskRead},
		{toolResolveThreads, resolveInputSchema, resolveOutputSchema, false, toolspkg.RiskMutating},
		{toolGitPush, pushInputSchema, pushOutputSchema, false, toolspkg.RiskOpenWorld},
	}
	descriptors := make([]toolspkg.ExtensionToolRuntimeDescriptor, 0, len(specs))
	for _, spec := range specs {
		id, err := runtimeToolID(spec.handler)
		if err != nil {
			return nil, err
		}
		inputDigest, err := toolspkg.SchemaDigest(spec.inputSchema)
		if err != nil {
			return nil, fmt.Errorf("dev-cycle: %s input schema digest: %w", spec.handler, err)
		}
		outputDigest, err := toolspkg.SchemaDigest(spec.outputSchema)
		if err != nil {
			return nil, fmt.Errorf("dev-cycle: %s output schema digest: %w", spec.handler, err)
		}
		descriptors = append(descriptors, toolspkg.ExtensionToolRuntimeDescriptor{
			ID:                 id,
			Handler:            spec.handler,
			InputSchemaDigest:  inputDigest,
			OutputSchemaDigest: outputDigest,
			ReadOnly:           spec.readOnly,
			Risk:               spec.risk,
		})
	}
	return descriptors, nil
}
