package docpost

import (
	"fmt"

	"strings"
)

// TransformMarkdown converts raw Cobra markdown to Fumadocs MDX format.
// It strips boilerplate, extracts metadata, and prepends YAML frontmatter.
func TransformMarkdown(raw, cmdName string) string {
	description := extractDescription(raw)
	body := stripBoilerplate(raw)
	body = fenceIndentedBlocks(body)
	body = escapeJSX(body)
	body = rewriteLinks(body)
	body = strings.TrimSpace(body)

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: %q\n", cmdName)
	fmt.Fprintf(&b, "description: %q\n", description)
	b.WriteString("---\n\n")
	b.WriteString(body)
	b.WriteString("\n")

	return b.String()
}

func enrichDocument(
	body string,
	current input,
	inputs []input,
	targets map[string]string,
) string {
	body = strings.TrimSpace(body)

	var sections []string
	if section := renderOutputFormatsSection(body); section != "" {
		sections = append(sections, section)
	}
	if section := renderSubcommandsSection(current, inputs, targets); section != "" {
		sections = append(sections, section)
	}

	if len(sections) == 0 {
		return body + "\n"
	}

	return body + "\n\n" + strings.Join(sections, "\n\n") + "\n"
}

// filenameToCommand converts a Cobra-generated filename to a command name.
// e.g. "agh_session_list.md" → "agh session list"
func filenameToCommand(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	return baseNameToCommand(name)
}

// extractDescription pulls the short description from Cobra markdown.
// Cobra generates: ## agh session list\n\nShort description here\n\n### Synopsis
// We grab the first paragraph after the H2 heading.
func extractDescription(raw string) string {
	lines := strings.Split(raw, "\n")
	inDescription := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			inDescription = true
			continue
		}

		if inDescription {
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				break
			}
			return trimmed
		}
	}

	return ""
}

func renderOutputFormatsSection(body string) string {
	if !strings.Contains(body, "--output string") {
		return ""
	}
	if strings.Contains(body, "## Output Formats") {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Output Formats\n\n")
	b.WriteString("Every AGH command supports `-o, --output`:\n\n")
	b.WriteString("- `human` for interactive terminal use\n")
	b.WriteString("- `json` for scripts and other machine-readable consumers\n")
	b.WriteString("- `jsonl` for wait or streaming commands that emit one JSON record per line\n")
	b.WriteString("- `toon` for compact agent-readable summaries\n")

	if usage := extractUsageLine(body); usage != "" {
		b.WriteString("\nExample:\n\n```bash\n")
		b.WriteString(outputExampleCommand(usage))
		b.WriteString("\n```")
	}

	return b.String()
}

func extractUsageLine(body string) string {
	lines := strings.Split(body, "\n")
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if !inFence {
			continue
		}
		if strings.HasPrefix(trimmed, "agh ") {
			return trimmed
		}
	}
	return ""
}

func outputExampleCommand(usage string) string {
	usage = strings.ReplaceAll(usage, "[flags]", "")
	usage = strings.Join(strings.Fields(usage), " ")
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return ""
	}
	return usage + " -o json"
}

// stripBoilerplate removes Cobra auto-generated artifacts:
// - The "###### Auto generated" footer line
// - The "### SEE ALSO" section (contains local .md file links)
func stripBoilerplate(raw string) string {
	result := autoGenLine.ReplaceAllString(raw, "")
	result = seeAlsoRe.ReplaceAllString(result, "")
	return result
}
