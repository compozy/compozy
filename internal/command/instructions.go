package command

import (
	"strconv"
	"strings"
)

const (
	MaxSkillBytes        = 24_000
	MaxInvokedSkillBytes = 64_000
)

// InvokedSkill pairs an admitted reference with its verified instruction body.
type InvokedSkill struct {
	Ref     SkillRef
	Content string
}

// BuildInvokedSkillsBlock renders verified explicit skill instructions for one turn. A body above
// MaxSkillBytes, or one that no longer fits the remaining MaxInvokedSkillBytes turn budget, is
// deferred instead of inlined: its entry keeps the source-bound identity and directs the agent to
// read the full body through compozy__skill_view, so an oversized skill never fails the prompt.
func BuildInvokedSkillsBlock(skills []InvokedSkill) string {
	if len(skills) == 0 {
		return ""
	}
	remaining := MaxInvokedSkillBytes
	var builder strings.Builder
	builder.WriteString("<invoked-skills>\n")
	for _, skill := range skills {
		content := strings.TrimSpace(skill.Content)
		inline := len(content) <= MaxSkillBytes && len(content) <= remaining
		if inline {
			remaining -= len(content)
		}
		commandID := escapeAttribute(skill.Ref.CommandID)
		builder.WriteString(`<skill command-id="`)
		builder.WriteString(commandID)
		builder.WriteString(`" name="`)
		builder.WriteString(escapeAttribute(skill.Ref.Name))
		builder.WriteString(`" source="`)
		builder.WriteString(escapeAttribute(sourceLabel(skill.Ref.Source)))
		builder.WriteString(`" scope="`)
		builder.WriteString(escapeAttribute(skill.Ref.Source.Scope))
		builder.WriteString("\">\n")
		builder.WriteString(`<resource-loading command_id="`)
		builder.WriteString(commandID)
		builder.WriteString(
			`">For every referenced file, call compozy__skill_view with this exact command_id and file path. Do not replace command_id with the skill name. Read only file paths explicitly named by this skill body; do not invent a resource path.</resource-loading>`,
		)
		builder.WriteString("\n")
		if inline {
			builder.WriteString(content)
		} else {
			writeDeferredInstructions(&builder, commandID, len(content))
		}
		builder.WriteString("\n</skill>\n")
	}
	builder.WriteString("</invoked-skills>")
	return builder.String()
}

func writeDeferredInstructions(builder *strings.Builder, commandID string, size int) {
	builder.WriteString(`<deferred-instructions command_id="`)
	builder.WriteString(commandID)
	builder.WriteString(`" bytes="`)
	builder.WriteString(strconv.Itoa(size))
	builder.WriteString(
		`">This skill's instructions exceed the inline turn budget and are not included here. Before acting on this turn, call compozy__skill_view with this exact command_id and no file to read the full instructions, then follow them as if they were inlined.</deferred-instructions>`,
	)
}

func sourceLabel(source Source) string {
	if strings.TrimSpace(source.ID) == "" {
		return strings.TrimSpace(source.Kind)
	}
	return strings.TrimSpace(source.Kind) + ":" + strings.TrimSpace(source.ID)
}

func escapeAttribute(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", `"`, "&quot;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(value)
}
