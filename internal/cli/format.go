package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"
)

// OutputFormat controls CLI output rendering.
type OutputFormat string

const (
	// OutputHuman renders human-readable tables and sections.
	OutputHuman OutputFormat = "human"
	// OutputJSON renders raw JSON payloads.
	OutputJSON OutputFormat = "json"
	// OutputJSONL renders newline-delimited JSON for streaming-style commands.
	OutputJSONL OutputFormat = "jsonl"
	// OutputToon renders a compact LLM-friendly TOON-like text document.
	OutputToon OutputFormat = "toon"
)

const (
	cliActiveGenerationKey   = "active_generation"
	cliActiveGenerationValue = "Active Generation"
	cliAppliedKey            = "applied"
	cliAppliedValue          = "Applied"
	cliApplyRecordIDKey      = "apply_record_id"
	cliAuthModeKey           = "auth_mode"
	cliPriorityValue         = "Priority"
	cliCodeValue             = "Code"
	cliCommandValue          = "Command"
	cliDurationMSKey         = "duration_ms"
	cliDurationValue         = "Duration"
	cliHashValue             = "Hash"
	cliInputTokensKey        = "input_tokens"
	cliInputTokensValue      = "Input Tokens"
	cliLifecycleKey          = "lifecycle"
	cliLifecycleValue        = "Lifecycle"
	cliNextActionKey         = "next_action"
	cliNextActionValue       = "Next Action"
	cliPIDKey                = "pid"
	cliPIDValue              = "PID"
	cliOutputTokensKey       = "output_tokens"
	cliOutputTokensValue     = "Output Tokens"
	cliSeverityValue         = "Severity"
	cliStatusHeader          = "STATUS"
	cliTurnsValue            = "Turns"
	cliUptimeValue           = "Uptime"
)

type outputBundle struct {
	jsonValue any
	jsonl     func(*cobra.Command) error
	human     func() (string, error)
	toon      func() (string, error)
}

func listBundle[T any](
	jsonValue any,
	items []T,
	humanTitle string,
	humanHeaders []string,
	toonName string,
	toonFields []string,
	humanRow func(T) []string,
	toonRow func(T) []string,
) outputBundle {
	return outputBundle{
		jsonValue: jsonValue,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLines(cmd, items)
		},
		human: func() (string, error) {
			if humanRow == nil {
				return "", errors.New("cli: human list row renderer is required")
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, humanRow(item))
			}
			return renderHumanTable(humanTitle, humanHeaders, rows), nil
		},
		toon: func() (string, error) {
			if toonRow == nil {
				return "", errors.New("cli: toon list row renderer is required")
			}
			rows := make([][]string, 0, len(items))
			for _, item := range items {
				rows = append(rows, toonRow(item))
			}
			return renderToonArray(toonName, toonFields, rows), nil
		},
	}
}

type keyValue struct {
	Label string
	Value string
}

func resolveOutputFormat(cmd *cobra.Command) (OutputFormat, error) {
	if cmd == nil {
		return "", errors.New("cli: command is required")
	}

	value, err := cmd.Flags().GetString(outputFlagName)
	if err != nil {
		return "", fmt.Errorf("cli: read output flag: %w", err)
	}
	jsonEnabled, err := cmd.Flags().GetBool(jsonFlagName)
	if err == nil && jsonEnabled {
		outputFlag := cmd.Flag(outputFlagName)
		normalized := OutputFormat(strings.ToLower(strings.TrimSpace(value)))
		if outputFlag != nil && outputFlag.Changed && normalized != "" && normalized != OutputJSON {
			return "", errors.New("cli: --json cannot be combined with a non-json output format")
		}
		return OutputJSON, nil
	}

	switch OutputFormat(strings.ToLower(strings.TrimSpace(value))) {
	case "", OutputHuman:
		return OutputHuman, nil
	case OutputJSON:
		return OutputJSON, nil
	case OutputJSONL:
		return OutputJSONL, nil
	case OutputToon:
		return OutputToon, nil
	default:
		return "", fmt.Errorf("cli: invalid output format %q", value)
	}
}

func writeCommandOutput(cmd *cobra.Command, bundle outputBundle) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}

	switch mode {
	case OutputJSON:
		return writeJSON(cmd, bundle.jsonValue)
	case OutputJSONL:
		if bundle.jsonl == nil {
			return errors.New("cli: jsonl formatter is required")
		}
		return bundle.jsonl(cmd)
	case OutputToon:
		if bundle.toon == nil {
			return errors.New("cli: toon formatter is required")
		}
		rendered, err := bundle.toon()
		if err != nil {
			return err
		}
		return writeRawCommandOutput(cmd, rendered)
	default:
		if bundle.human == nil {
			return errors.New("cli: human formatter is required")
		}
		rendered, err := bundle.human()
		if err != nil {
			return err
		}
		return writeRawCommandOutput(cmd, rendered)
	}
}

func writeJSONLine(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeJSONLines[T any](cmd *cobra.Command, items []T) error {
	for _, item := range items {
		if err := writeJSONLine(cmd, item); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeRawCommandOutput(cmd *cobra.Command, text string) error {
	writer := cmd.OutOrStdout()
	if strings.HasSuffix(text, "\n") {
		_, err := fmt.Fprint(writer, text)
		return err
	}
	_, err := fmt.Fprintln(writer, text)
	return err
}

func renderHumanSection(title string, rows []keyValue) string {
	rendered, err := renderHumanSectionResult(title, rows)
	if err != nil {
		panic(fmt.Sprintf("cli: render human section: %v", err))
	}
	return rendered
}

func renderHumanSectionResult(title string, rows []keyValue) (string, error) {
	var buffer bytes.Buffer
	if title != "" {
		if _, err := fmt.Fprintf(&buffer, "%s\n", title); err != nil {
			return "", fmt.Errorf("cli: write human section title: %w", err)
		}
		if _, err := fmt.Fprintf(&buffer, "%s\n", strings.Repeat("=", humanTableCellWidth(title))); err != nil {
			return "", fmt.Errorf("cli: write human section underline: %w", err)
		}
	}

	writer := tabwriter.NewWriter(&buffer, 0, 0, 2, ' ', 0)
	for _, row := range rows {
		if _, err := fmt.Fprintf(writer, "%s:\t%s\n", row.Label, row.Value); err != nil {
			return "", fmt.Errorf("cli: write human section row %q: %w", row.Label, err)
		}
	}
	if err := writer.Flush(); err != nil {
		return "", fmt.Errorf("cli: flush human section rows: %w", err)
	}

	return strings.TrimRight(buffer.String(), "\n"), nil
}

func renderHumanTable(title string, headers []string, rows [][]string) string {
	var builder strings.Builder
	if title != "" {
		builder.WriteString(title)
		builder.WriteByte('\n')
		builder.WriteString(strings.Repeat("=", humanTableCellWidth(title)))
		builder.WriteByte('\n')
	}
	if len(headers) == 0 {
		return strings.TrimRight(builder.String(), "\n")
	}

	separators := make([]string, 0, len(headers))
	for _, header := range headers {
		separators = append(separators, strings.Repeat("-", max(3, humanTableCellWidth(header))))
	}

	tableRows := make([][]string, 0, len(rows)+2)
	tableRows = append(tableRows, headers, separators)
	if len(rows) == 0 {
		tableRows = append(tableRows, []string{"(empty)"})
	} else {
		tableRows = append(tableRows, rows...)
	}
	widths := humanTableColumnWidths(tableRows)
	for _, row := range tableRows {
		writeHumanTableRow(&builder, row, widths)
	}

	return strings.TrimRight(builder.String(), "\n")
}

func humanTableColumnWidths(rows [][]string) []int {
	var widths []int
	for _, row := range rows {
		for column, cell := range row {
			if column == len(widths) {
				widths = append(widths, 0)
			}
			widths[column] = max(widths[column], humanTableCellWidth(cell))
		}
	}
	return widths
}

func writeHumanTableRow(builder *strings.Builder, row []string, widths []int) {
	for column, cell := range row {
		if column > 0 {
			builder.WriteString("  ")
		}
		builder.WriteString(cell)
		if column < len(row)-1 && column < len(widths) {
			builder.WriteString(strings.Repeat(" ", max(0, widths[column]-humanTableCellWidth(cell))))
		}
	}
	builder.WriteByte('\n')
}

func humanTableCellWidth(cell string) int {
	return utf8.RuneCountInString(cell)
}

func renderHumanBlocks(blocks ...string) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		trimmed := strings.TrimSpace(block)
		if trimmed == "" {
			continue
		}
		parts = append(parts, trimmed)
	}
	return strings.Join(parts, "\n\n")
}

func renderToonObject(name string, fields []string, values []string) string {
	var builder strings.Builder
	builder.WriteString(name)
	builder.WriteByte('{')
	builder.WriteString(strings.Join(fields, ","))
	builder.WriteString("}:\n  ")
	writeToonValues(&builder, values)
	return builder.String()
}

func renderToonArray(name string, fields []string, rows [][]string) string {
	var builder strings.Builder
	builder.WriteString(name)
	builder.WriteByte('[')
	builder.WriteString(strconv.Itoa(len(rows)))
	builder.WriteByte(']')
	builder.WriteByte('{')
	builder.WriteString(strings.Join(fields, ","))
	builder.WriteString("}:")
	if len(rows) == 0 {
		builder.WriteString("\n  (empty)")
		return builder.String()
	}
	for _, row := range rows {
		builder.WriteString("\n  ")
		writeToonValues(&builder, row)
	}
	return builder.String()
}

func writeToonValues(builder *strings.Builder, values []string) {
	for i, value := range values {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(toonValue(value))
	}
}

func toonValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return `""`
	}
	if strings.ContainsAny(trimmed, ",\n\r\t\"") {
		return strconv.Quote(trimmed)
	}
	return trimmed
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, raw); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return buffer.String()
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatAge(now func() time.Time, then time.Time) string {
	if then.IsZero() {
		return ""
	}
	current := time.Now().UTC()
	if now != nil {
		current = now().UTC()
	}
	delta := max(current.Sub(then.UTC()), 0)

	switch {
	case delta < time.Minute:
		return fmt.Sprintf("%ds", int(delta.Seconds()))
	case delta < time.Hour:
		return fmt.Sprintf("%dm", int(delta.Minutes()))
	case delta < 24*time.Hour:
		return fmt.Sprintf("%dh", int(delta.Hours()))
	default:
		return fmt.Sprintf("%dd", int(delta.Hours()/24))
	}
}

func stringOrDash(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "--"
	}
	return trimmed
}

func intOrDash(value int) string {
	if value <= 0 {
		return "--"
	}
	return strconv.Itoa(value)
}

func int64OrDash(value int64) string {
	if value <= 0 {
		return "--"
	}
	return strconv.FormatInt(value, 10)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
