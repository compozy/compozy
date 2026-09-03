package command

import (
	"errors"
	"fmt"
	"strings"
)

const (
	MaxSkillBytes        = 24_000
	MaxInvokedSkillBytes = 64_000
)

var (
	// ErrSkillContextTooLarge reports explicit skill instructions outside the bounded turn budget.
	ErrSkillContextTooLarge = errors.New("command: invoked skill context too large")
)

// InvokedSkill pairs an admitted reference with its verified instruction body.
type InvokedSkill struct {
	Ref     SkillRef
	Content string
}

// BuildInvokedSkillsBlock renders verified explicit skill instructions for one turn.
func BuildInvokedSkillsBlock(skills []InvokedSkill) (string, error) {
	if len(skills) == 0 {
		return "", nil
	}
	total := 0
	var builder strings.Builder
	builder.WriteString("<invoked-skills>\n")
	for _, skill := range skills {
		content := strings.TrimSpace(skill.Content)
		if len(content) > MaxSkillBytes {
			return "", fmt.Errorf(
				"%w: %s exceeds %d bytes",
				ErrSkillContextTooLarge,
				skill.Ref.CommandID,
				MaxSkillBytes,
			)
		}
		total += len(content)
		if total > MaxInvokedSkillBytes {
			return "", fmt.Errorf("%w: total exceeds %d bytes", ErrSkillContextTooLarge, MaxInvokedSkillBytes)
		}
		builder.WriteString(`<skill command-id="`)
		builder.WriteString(escapeAttribute(skill.Ref.CommandID))
		builder.WriteString(`" name="`)
		builder.WriteString(escapeAttribute(skill.Ref.Name))
		builder.WriteString(`" source="`)
		builder.WriteString(escapeAttribute(sourceLabel(skill.Ref.Source)))
		builder.WriteString(`" scope="`)
		builder.WriteString(escapeAttribute(skill.Ref.Source.Scope))
		builder.WriteString("\">\n")
		builder.WriteString(`<resource-loading command_id="`)
		builder.WriteString(escapeAttribute(skill.Ref.CommandID))
		builder.WriteString(
			`">For every referenced file, call compozy__skill_view with this exact command_id and file path. Do not replace command_id with the skill name. Read only file paths explicitly named by this skill body; do not invent a resource path.</resource-loading>`,
		)
		builder.WriteString("\n")
		builder.WriteString(content)
		builder.WriteString("\n</skill>\n")
	}
	builder.WriteString("</invoked-skills>")
	return builder.String(), nil
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
