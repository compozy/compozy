package cli

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const extensionLogMessageKey = "message"

func extensionLogsBundle(entries []ExtensionLogRecord) outputBundle {
	return outputBundle{
		jsonValue: entries,
		jsonl: func(cmd *cobra.Command) error {
			for _, entry := range entries {
				if err := writeJSONLine(cmd, entry); err != nil {
					return err
				}
			}
			return nil
		},
		human: func() (string, error) {
			rows := make([]keyValue, 0, len(entries))
			for _, entry := range entries {
				rows = append(rows, keyValue{
					Label: entry.Timestamp.Format("15:04:05") + " #" + strconv.FormatInt(entry.Sequence, 10),
					Value: entry.Message,
				})
			}
			return renderHumanSectionResult("Extension logs", rows)
		},
		toon: func() (string, error) {
			rows := make([][]string, 0, len(entries))
			for _, entry := range entries {
				rows = append(rows, []string{
					strconv.FormatInt(entry.Sequence, 10),
					entry.Timestamp.Format(time.RFC3339Nano),
					entry.Message,
				})
			}
			return renderToonArray(
				"extension_logs",
				[]string{"sequence", "timestamp", extensionLogMessageKey},
				rows,
			), nil
		},
	}
}
