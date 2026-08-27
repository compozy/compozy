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
	outputProfile OutputProfile,
) string {
	body = strings.TrimSpace(body)

	var sections []string
	if section := renderOutputFormatsSection(body, outputProfile); section != "" {
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
// e.g. "compozy_session_list.md" → "compozy session list"
func filenameToCommand(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	return baseNameToCommand(name)
}

// extractDescription pulls the short description from Cobra markdown.
// Cobra generates: ## compozy session list\n\nShort description here\n\n### Synopsis
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

func renderOutputFormatsSection(body string, outputProfile OutputProfile) string {
	if !strings.Contains(body, "--output string") {
		return ""
	}
	if strings.Contains(body, "## Output Formats") {
		return ""
	}

	var b strings.Builder
	b.WriteString("## Output Formats\n\n")
	b.WriteString(renderOutputFormatProfile(outputProfile))

	if outputProfile == OutputProfileResult ||
		outputProfile == OutputProfileHumanJSONL ||
		outputProfile == OutputProfileTerminalOpen ||
		outputProfile == OutputProfileTerminalQuote {
		usage := extractOutputExampleLine(body)
		if usage == "" {
			usage = extractUsageLine(body)
		}
		if usage == "" {
			return b.String()
		}
		if outputProfile == OutputProfileTerminalQuote {
			usage += " --lines 1-5"
		}
		b.WriteString("\nExample:\n\n```bash\n")
		switch outputProfile {
		case OutputProfileHumanJSONL:
			b.WriteString(outputExampleCommandWithFormat(usage, "jsonl"))
		case OutputProfileTerminalOpen:
			b.WriteString(outputExampleCommandWithFormat(usage+" --detach", "json"))
		default:
			b.WriteString(outputExampleCommand(usage))
		}
		b.WriteString("\n```")
	}

	return b.String()
}

func extractOutputExampleLine(body string) string {
	inExamples := false
	inFence := false
	outputExample := false
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				outputExample = false
			}
			inFence = !inFence
			continue
		}
		if !inFence {
			if trimmed == "### Examples" {
				inExamples = true
				continue
			}
			if inExamples && strings.HasPrefix(trimmed, "### ") {
				return ""
			}
		}
		if !inExamples || !inFence {
			continue
		}
		if trimmed == "# Output example" {
			outputExample = true
			continue
		}
		if outputExample && strings.HasPrefix(trimmed, "compozy ") {
			return trimmed
		}
	}
	return ""
}

func renderOutputFormatProfile(outputProfile OutputProfile) string {
	switch outputProfile {
	case OutputProfileHelp:
		return "This command displays human-readable help and its subcommand list. " +
			"The inherited `-o, --output` and `--json` flags do not serialize help output.\n\n" +
			renderOutputFormatTable(
				"Output",
				"Help and subcommand list",
				"Not emitted for help",
				"Not emitted for help",
				"Not emitted for help",
			)
	case OutputProfileNoOutput:
		return "This command has no command-result renderer. The inherited `-o, --output` and " +
			"`--json` flags do not add JSON, JSONL, or TOON output.\n\n" +
			renderOutputFormatTable("Output", "No command result", "Not emitted", "Not emitted", "Not emitted")
	case OutputProfileProtocol:
		return "This command reserves standard output for its MCP transport. The inherited `-o, --output` " +
			"and `--json` flags do not change that protocol stream.\n\n" +
			renderOutputFormatTable(
				"Output",
				"MCP transport stream",
				"Not a command-result renderer",
				"Not a command-result renderer",
				"Not a command-result renderer",
			)
	case OutputProfileRaw:
		return "This command writes its shell-completion script directly to standard output. The inherited " +
			"`-o, --output` and `--json` flags do not convert that script to a command result.\n\n" +
			renderOutputFormatTable(
				"Output",
				"Shell-completion script",
				"Not a command-result renderer",
				"Not a command-result renderer",
				"Not a command-result renderer",
			)
	case OutputProfileHumanJSONL:
		return "This streaming command supports interactive human output or newline-delimited JSON. " +
			"It rejects the inherited `json` and `toon` output modes.\n\n" +
			renderOutputFormatTable(
				"Stream output",
				"Interactive event stream",
				"Not supported",
				"One JSON event per line",
				"Not supported",
			)
	case OutputProfileTerminalInteractive:
		return "This command writes an interactive terminal stream. The inherited `-o, --output` and " +
			"`--json` flags do not serialize attached terminal bytes.\n\n" +
			renderOutputFormatTable(
				"Stream output",
				"Interactive terminal rendering",
				"Not supported",
				"Not supported",
				"Not supported",
			)
	case OutputProfileTerminalOpen:
		return "Without `--detach`, this command writes an interactive terminal stream. Add `--detach` " +
			"before selecting a structured output format.\n\n" +
			renderOutputFormatTable(
				"Command result",
				"Interactive terminal rendering, or a detached terminal record",
				"Detached terminal record",
				"Detached terminal record",
				"Detached terminal record",
			)
	case OutputProfileTerminalQuote:
		return renderTerminalQuoteOutputProfile()
	default:
		return "The root `-o, --output` flag accepts these values. A renderer is supported only when " +
			"this command writes that result; `jsonl` is for commands that emit one JSON record per line.\n\n" +
			renderOutputFormatTable(
				"Command result",
				"Interactive terminal rendering",
				"Machine-readable JSON rendering",
				"One JSON record per line when supported",
				"Compact agent-readable rendering",
			)
	}
}

func renderTerminalQuoteOutputProfile() string {
	return "This command renders a bounded terminal excerpt as an escaped quote block or a JSON record. " +
		"It does not provide JSONL or TOON renderers.\n\n" +
		renderOutputFormatTable(
			"Command result",
			"Escaped `<terminal_context>` quote block",
			"Terminal quote record",
			"Not supported",
			"Not supported",
		)
}

func renderOutputFormatTable(header, human, jsonOutput, jsonl, toon string) string {
	return "| Format | " + header + " |\n" +
		"| --- | --- |\n" +
		"| `human` | " + human + " |\n" +
		"| `json` | " + jsonOutput + " |\n" +
		"| `jsonl` | " + jsonl + " |\n" +
		"| `toon` | " + toon + " |"
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
		if strings.HasPrefix(trimmed, "compozy ") {
			return trimmed
		}
	}
	return ""
}

func outputExampleCommand(usage string) string {
	return outputExampleCommandWithFormat(usage, "json")
}

func outputExampleCommandWithFormat(usage, format string) string {
	usage = strings.ReplaceAll(usage, "[flags]", "")
	usage = strings.Join(strings.Fields(usage), " ")
	usage = strings.TrimSpace(usage)
	if usage == "" {
		return ""
	}
	beforeCommand, commandArgs, found := strings.Cut(usage, " -- ")
	if found {
		return beforeCommand + " -o " + format + " -- " + commandArgs
	}
	return usage + " -o " + format
}

// stripBoilerplate removes Cobra auto-generated artifacts:
// - The "###### Auto generated" footer line
// - The "### SEE ALSO" section (contains local .md file links)
func stripBoilerplate(raw string) string {
	result := autoGenLine.ReplaceAllString(raw, "")
	result = seeAlsoRe.ReplaceAllString(result, "")
	return result
}
