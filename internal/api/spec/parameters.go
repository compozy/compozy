package spec

import (
	"github.com/compozy/agh/internal/api/contract"

	"github.com/getkin/kin-openapi/openapi3"
)

func pathParam(name string, description string) ParameterSpec {
	return ParameterSpec{Name: name, In: openapi3.ParameterInPath, Description: description, Required: true}
}

func headerParam(name string, description string) ParameterSpec {
	return ParameterSpec{Name: name, In: openapi3.ParameterInHeader, Description: description, Required: true}
}

func optionalHeaderParam(name string, description string) ParameterSpec {
	return ParameterSpec{Name: name, In: openapi3.ParameterInHeader, Description: description, Required: false}
}

func queryParam(name string, description string, required bool) ParameterSpec {
	return ParameterSpec{Name: name, In: openapi3.ParameterInQuery, Description: description, Required: required}
}

func enumQueryParam(name string, description string, values []string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Enum:        values,
	}
}

func boolQueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        "boolean",
	}
}

func intQueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        specIntegerKey,
		Format:      "int32",
	}
}

func memorySelectorQueryParams() []ParameterSpec {
	return []ParameterSpec{
		enumQueryParam(specScopeKey, "Memory scope", memoryScopeValues()),
		queryParam("workspace_id", "Durable workspace id", false),
		queryParam("agent_name", "Agent name for agent-scoped memory", false),
		enumQueryParam("agent_tier", "Agent memory tier", memoryAgentTierValues()),
	}
}

func memoryError(status int, description string) ResponseSpec {
	return ResponseSpec{Status: status, Description: description, Body: contract.MemoryErrorPayload{}}
}

func afterSequenceQueryParam(description string) ParameterSpec {
	return ParameterSpec{
		Name:        "after_sequence",
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        specIntegerKey,
		Format:      specFormatInt64,
	}
}

func beforeSequenceQueryParam(description string) ParameterSpec {
	return ParameterSpec{
		Name:        "before_sequence",
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        specIntegerKey,
		Format:      specFormatInt64,
	}
}

func dateTimeQueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          openapi3.ParameterInQuery,
		Description: description,
		Required:    false,
		Format:      "date-time",
	}
}
