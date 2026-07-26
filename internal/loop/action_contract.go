package loop

import (
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
)

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
	if stopWhen := strings.TrimSpace(contract.StopWhen); stopWhen != "" {
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
