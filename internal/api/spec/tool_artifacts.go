package spec

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/getkin/kin-openapi/openapi3"
)

func applyToolArtifactContract(operations []OperationSpec) []OperationSpec {
	for i := range operations {
		if operations[i].OperationID == "invokeTool" {
			operations[i].Responses = append(operations[i].Responses, ResponseSpec{
				Status:      507,
				Description: "Oversized result persistence failed; a bounded partial result is preserved",
				Body:        contract.ToolErrorResponse{},
			})
			break
		}
	}
	return append(operations, OperationSpec{
		Method:      httpMethodGet,
		Path:        "/api/workspaces/{workspace_id}/tool-artifacts/{artifact_id}",
		OperationID: "readToolArtifact",
		Summary:     "Read one retained oversized tool-result page",
		Tags:        []string{specToolsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("workspace_id", "Durable workspace id"),
			pathParam("artifact_id", "Opaque content-addressed artifact id"),
			toolArtifactInt64QueryParam("offset", "Zero-based byte offset"),
			toolArtifactInt64QueryParam("limit", "Page size in bytes; defaults to and is capped at 65536"),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.ToolArtifactPageResponse{}},
			{Status: 400, Description: "Invalid artifact id or byte range", Body: contract.ToolErrorResponse{}},
			{
				Status:      404,
				Description: "Artifact not found in the resolved workspace",
				Body:        contract.ToolErrorResponse{},
			},
			{
				Status:      502,
				Description: "Retained artifact is corrupt or unreadable",
				Body:        contract.ToolErrorResponse{},
			},
			{Status: 503, Description: "Tool artifact store unavailable", Body: contract.ErrorPayload{}},
		},
	})
}

func toolArtifactInt64QueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        specIntegerKey,
		Format:      "int64",
	}
}
