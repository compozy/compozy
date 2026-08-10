package loop

import (
	"fmt"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

func contractNarrativeStringFields(contract dsl.Contract) []namedString {
	fields := []namedString{
		{name: "contract.goal", value: contract.Goal},
		{name: "contract.definition_of_done", value: contract.DefinitionOfDone},
	}
	for index, constraint := range contract.Constraints {
		fields = append(fields, namedString{
			name:  fmt.Sprintf("contract.constraints[%d]", index),
			value: constraint,
		})
	}
	for index, boundary := range contract.Boundaries {
		fields = append(fields, namedString{
			name:  fmt.Sprintf("contract.boundaries[%d]", index),
			value: boundary,
		})
	}
	return fields
}

func contractNarrativeNamespace(namespace refs.Namespace) refs.Namespace {
	return refs.Namespace{Inputs: namespace.Inputs}
}
