package cli

import "github.com/spf13/cobra"

const (
	listPageRecordType  = "page"
	listReturnedLabel   = "Returned"
	listOffsetField     = "offset"
	listNextOffsetField = "next_offset"
	listReturnedField   = "returned"
	listTotalField      = "total"
	listLimitField      = "limit"
	listHasMoreField    = "has_more"
	listNextCursorField = "next_cursor"
	listTotalLabel      = "Total"
	listLimitLabel      = "Limit"
	listHasMoreLabel    = "Has More"
	listNextCursorLabel = "Next Cursor"
	sessionIDLabel      = "Session ID"
)

func withListPageOutput(bundle outputBundle, page any, rows []keyValue, fields, values []string) outputBundle {
	baseJSONL, baseToon := bundle.jsonl, bundle.toon
	bundle.jsonl = func(cmd *cobra.Command) error {
		if err := baseJSONL(cmd); err != nil {
			return err
		}
		return writeJSONLine(cmd, struct {
			Type string `json:"type"`
			Page any    `json:"page"`
		}{Type: listPageRecordType, Page: page})
	}
	bundle.human = withListPageHuman(bundle.human, rows)
	bundle.toon = func() (string, error) {
		items, err := baseToon()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(items, renderToonObject(listPageRecordType, fields, values)), nil
	}
	return bundle
}

func withListPageHuman(base func() (string, error), rows []keyValue) func() (string, error) {
	return func() (string, error) {
		table, err := base()
		if err != nil {
			return "", err
		}
		return renderHumanBlocks(table, renderHumanSection("Page", rows)), nil
	}
}
