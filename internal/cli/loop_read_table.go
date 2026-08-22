package cli

import "strings"

const (
	loopDurationHeader = "DURATION"
	loopItemIndexKey   = "item_index"
	loopNeedsYouLabel  = "NEEDS YOU"
	loopNodeIDJSONKey  = "node_id"
	loopRoundHeader    = "ROUND"
)

func renderLoopReadTable(headers []string, rows [][]string) string {
	tableRows := make([][]string, 0, len(rows)+1)
	tableRows = append(tableRows, headers)
	tableRows = append(tableRows, rows...)
	widths := humanTableColumnWidths(tableRows)
	var builder strings.Builder
	for _, row := range tableRows {
		writeHumanTableRow(&builder, row, widths)
	}
	return strings.TrimRight(builder.String(), "\n")
}
