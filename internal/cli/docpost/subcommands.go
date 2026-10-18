package docpost

import (
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"
)

type subcommandRow struct {
	command     string
	description string
}

func renderSubcommandsSection(current input, inputs []input, targets map[string]string) string {
	children := directChildren(current, inputs)
	if len(children) == 0 {
		return ""
	}

	rows := make([]subcommandRow, 0, len(children))
	for _, child := range children {
		description := cmp.Or(strings.TrimSpace(extractDescription(child.raw)), "See command reference.")
		rows = append(rows, subcommandRow{
			command:     "[" + child.commandName() + "](" + targets[child.baseName] + ")",
			description: strings.ReplaceAll(description, "|", "\\|"),
		})
	}

	return "## Subcommands\n\n" + renderSubcommandTable(rows)
}

func renderSubcommandTable(rows []subcommandRow) string {
	commandWidth := utf8.RuneCountInString("Command")
	descriptionWidth := utf8.RuneCountInString("Description")
	for _, row := range rows {
		commandWidth = max(commandWidth, utf8.RuneCountInString(row.command))
		descriptionWidth = max(descriptionWidth, utf8.RuneCountInString(row.description))
	}

	lines := make([]string, 0, len(rows)+2)
	lines = append(
		lines,
		renderSubcommandRow("Command", "Description", commandWidth, descriptionWidth),
		renderSubcommandRow(
			strings.Repeat("-", commandWidth),
			strings.Repeat("-", descriptionWidth),
			commandWidth,
			descriptionWidth,
		),
	)
	for _, row := range rows {
		lines = append(lines, renderSubcommandRow(row.command, row.description, commandWidth, descriptionWidth))
	}
	return strings.Join(lines, "\n")
}

func renderSubcommandRow(command, description string, commandWidth, descriptionWidth int) string {
	return "| " + padSubcommandCell(command, commandWidth) +
		" | " + padSubcommandCell(description, descriptionWidth) + " |"
}

func padSubcommandCell(value string, width int) string {
	return value + strings.Repeat(" ", width-utf8.RuneCountInString(value))
}

func directChildren(parent input, inputs []input) []input {
	children := make([]input, 0, 4)
	for _, candidate := range inputs {
		if len(candidate.segments) != len(parent.segments)+1 {
			continue
		}
		match := true
		for i := range parent.segments {
			if candidate.segments[i] != parent.segments[i] {
				match = false
				break
			}
		}
		if match {
			children = append(children, candidate)
		}
	}

	slices.SortFunc(children, func(left, right input) int {
		return cmp.Compare(left.commandName(), right.commandName())
	})
	return children
}
