package speccycle

import (
	"encoding/json"
	"fmt"

	toolspkg "github.com/compozy/compozy/internal/tools"
)

const (
	toolImportTasks    = "import_tasks"
	toolWriteArtifacts = "write_review_artifacts"
	toolFinalizeRound  = "finalize_review_round"
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
					"required":["id","number","title","type","complexity","path","body","body_ref","blocks"],
					"properties":{
						"id":{"type":"string"},
						"number":{"type":"integer"},
						"title":{"type":"string"},
						"type":{"type":"string"},
						"complexity":{"type":"string"},
						"runtime":{"type":"object","additionalProperties":false,"properties":{
							"provider":{"type":"string"},"model":{"type":"string"},"reasoning":{"type":"string"}
						}},
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
		{toolWriteArtifacts, writeArtifactsInputSchema, writeArtifactsOutputSchema, false, toolspkg.RiskMutating},
		{toolFinalizeRound, finalizeRoundInputSchema, finalizeRoundOutputSchema, false, toolspkg.RiskMutating},
	}
	descriptors := make([]toolspkg.ExtensionToolRuntimeDescriptor, 0, len(specs))
	for _, spec := range specs {
		id, err := runtimeToolID(spec.handler)
		if err != nil {
			return nil, err
		}
		inputDigest, err := toolspkg.SchemaDigest(spec.inputSchema)
		if err != nil {
			return nil, fmt.Errorf("spec-cycle: %s input schema digest: %w", spec.handler, err)
		}
		outputDigest, err := toolspkg.SchemaDigest(spec.outputSchema)
		if err != nil {
			return nil, fmt.Errorf("spec-cycle: %s output schema digest: %w", spec.handler, err)
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
