package loop

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/loop/dsl/refs"
)

// MaterializeContract resolves the user-facing contract narrative from Run inputs.
func MaterializeContract(contract dsl.Contract, inputs map[string]any) (dsl.Contract, error) {
	namespace := map[string]any{namespaceInputsKey: cloneAnyMap(inputs)}
	materialized := contract
	var err error
	materialized.Goal, err = refs.RenderTemplateString("contract.goal", contract.Goal, namespace)
	if err != nil {
		return dsl.Contract{}, fmt.Errorf("%w: materialize contract.goal: %w", ErrActionMaterialization, err)
	}
	materialized.DefinitionOfDone, err = refs.RenderTemplateString(
		"contract.definition_of_done",
		contract.DefinitionOfDone,
		namespace,
	)
	if err != nil {
		return dsl.Contract{}, fmt.Errorf(
			"%w: materialize contract.definition_of_done: %w",
			ErrActionMaterialization,
			err,
		)
	}
	materialized.Constraints, err = materializeContractLines(
		"contract.constraints",
		contract.Constraints,
		namespace,
	)
	if err != nil {
		return dsl.Contract{}, err
	}
	materialized.Boundaries, err = materializeContractLines(
		"contract.boundaries",
		contract.Boundaries,
		namespace,
	)
	if err != nil {
		return dsl.Contract{}, err
	}
	return materialized, nil
}

func materializeContractLines(
	prefix string,
	lines []string,
	namespace map[string]any,
) ([]string, error) {
	materialized := make([]string, 0, len(lines))
	for index, line := range lines {
		rendered, err := refs.RenderTemplateString(fmt.Sprintf("%s[%d]", prefix, index), line, namespace)
		if err != nil {
			return nil, fmt.Errorf("%w: materialize %s[%d]: %w", ErrActionMaterialization, prefix, index, err)
		}
		materialized = append(materialized, rendered)
	}
	return materialized, nil
}

// RenderContractBlock renders the ADR-018 loop contract for worker prompt context.
func RenderContractBlock(contract dsl.Contract) string {
	var builder strings.Builder
	builder.WriteString("Loop contract\n")
	builder.WriteString("Goal:\n")
	builder.WriteString(strings.TrimSpace(contract.Goal))
	builder.WriteString("\n\nDefinition of done:\n")
	builder.WriteString(strings.TrimSpace(contract.DefinitionOfDone))
	if len(contract.Constraints) > 0 {
		builder.WriteString("\n\nConstraints:\n")
		writeBulletLines(&builder, contract.Constraints)
	}
	if len(contract.Boundaries) > 0 {
		builder.WriteString("\n\nBoundaries:\n")
		writeBulletLines(&builder, contract.Boundaries)
	}
	if stopWhen := strings.TrimSpace(contract.StopWhen.Expr); stopWhen != "" {
		builder.WriteString("\n\nStop when:\n")
		builder.WriteString(stopWhen)
	}
	return builder.String()
}

func writeBulletLines(builder *strings.Builder, values []string) {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		builder.WriteString("- ")
		builder.WriteString(trimmed)
		builder.WriteByte('\n')
	}
}
