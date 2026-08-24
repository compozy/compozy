package spec

import (
	"github.com/compozy/compozy/internal/api/contract"

	"github.com/getkin/kin-openapi/openapi3"
)

const afterSequenceKey = "after_sequence"

func pathParam(name string, description string) ParameterSpec {
	return ParameterSpec{Name: name, In: specParameterInPath, Description: description, Required: true}
}

func headerParam(name string, description string) ParameterSpec {
	return ParameterSpec{Name: name, In: openapi3.ParameterInHeader, Description: description, Required: true}
}

func optionalLastEventIDHeaderParam(description string) ParameterSpec {
	return ParameterSpec{
		Name: specLastEventIDHeader, In: openapi3.ParameterInHeader, Description: description, Required: false,
	}
}

func queryParam(name string, description string, required bool) ParameterSpec {
	return ParameterSpec{Name: name, In: specParameterInQuery, Description: description, Required: required}
}

func enumQueryParam(name string, description string, values []string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          specParameterInQuery,
		Description: description,
		Required:    false,
		Enum:        values,
	}
}

func boolQueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          specParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        "boolean",
	}
}

func intQueryParam(name string, description string) ParameterSpec {
	return ParameterSpec{
		Name:        name,
		In:          specParameterInQuery,
		Description: description,
		Required:    false,
		Kind:        specIntegerKey,
		Format:      "int32",
	}
}

func requiredIntQueryParam(name string, description string) ParameterSpec {
	parameter := intQueryParam(name, description)
	parameter.Required = true
	return parameter
}

func requiredEnumQueryParam(name string, description string, values []string) ParameterSpec {
	parameter := enumQueryParam(name, description, values)
	parameter.Required = true
	return parameter
}

// withProfileScope documents the two — and only two — profile read modes every
// work-read surface accepts: scoped to one profile, or the explicit owner-labeled
// aggregate. Sending both is rejected with `profile_selection_conflict`; sending
// neither resolves the caller's profile rather than widening (ADR-005, ADR-015).
func withProfileScope(params ...ParameterSpec) []ParameterSpec {
	return append(params,
		queryParam(specProfileKey, "Read one profile's rows by name", false),
		boolQueryParam("all_profiles", "Read the owner-labeled all-profiles aggregate"),
	)
}

// withProfileSelector documents the scoped selector alone. Mutating surfaces resolve
// an owner rather than a view, so the aggregate is refused there and is not offered.
func withProfileSelector(params ...ParameterSpec) []ParameterSpec {
	return append(params, queryParam(specProfileKey, "Act as this profile by name", false))
}

func ensureProfileParameters(params []ParameterSpec, read bool) []ParameterSpec {
	for _, parameter := range params {
		if parameter.In == specParameterInQuery && parameter.Name == specProfileKey {
			return params
		}
	}
	if read {
		return withProfileScope(params...)
	}
	return withProfileSelector(params...)
}

func settingsLayeredParameters() []ParameterSpec {
	return []ParameterSpec{
		enumQueryParam(specScopeKey, "Select the settings scope", settingsLayeredScopeValues()),
		queryParam("workspace_id", "Select the workspace context", false),
		queryParam(specProfileKey, "Select the profile layer", false),
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
		Name:        afterSequenceKey,
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
