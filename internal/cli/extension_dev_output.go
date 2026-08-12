package cli

import (
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

const (
	extensionLogEventName      = "extension_log"
	extensionLogMessageKey     = "message"
	extensionLogResetEventName = "extension_log_reset"
	extensionLogStreamEpochKey = "stream_epoch"
)

type extensionLogResetRecord struct {
	Event       string               `json:"event"`
	Logs        []ExtensionLogRecord `json:"logs"`
	StreamEpoch string               `json:"stream_epoch"`
}

func extensionLogsBundle(snapshot ExtensionLogsRecord) outputBundle {
	return outputBundle{
		jsonValue: snapshot,
		jsonl: func(cmd *cobra.Command) error {
			for _, entry := range snapshot.Logs {
				if err := writeJSONLine(cmd, entry); err != nil {
					return err
				}
			}
			return nil
		},
		human: func() (string, error) {
			rows := make([]keyValue, 0, len(snapshot.Logs))
			for _, entry := range snapshot.Logs {
				rows = append(rows, keyValue{
					Label: entry.Timestamp.Format("15:04:05") + " #" + strconv.FormatInt(entry.Sequence, 10),
					Value: entry.Message,
				})
			}
			return renderHumanSectionResult("Extension logs", rows)
		},
		toon: func() (string, error) {
			rows := make([][]string, 0, len(snapshot.Logs))
			for _, entry := range snapshot.Logs {
				rows = append(rows, []string{
					entry.StreamEpoch,
					strconv.FormatInt(entry.Sequence, 10),
					entry.Timestamp.Format(time.RFC3339Nano),
					entry.Message,
				})
			}
			return renderToonArray(
				"extension_logs",
				[]string{extensionLogStreamEpochKey, "sequence", "timestamp", extensionLogMessageKey},
				rows,
			), nil
		},
	}
}

func extensionLogResetBundle(snapshot ExtensionLogsRecord) outputBundle {
	record := extensionLogResetRecord{
		Event:       extensionLogResetEventName,
		Logs:        snapshot.Logs,
		StreamEpoch: snapshot.StreamEpoch,
	}
	return outputBundle{
		jsonValue: record,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, record)
		},
		human: func() (string, error) {
			logs, err := extensionLogsBundle(snapshot).human()
			if err != nil {
				return "", err
			}
			return renderHumanBlocks("Extension log stream reset · "+snapshot.StreamEpoch, logs), nil
		},
		toon: func() (string, error) {
			logs, err := extensionLogsBundle(snapshot).toon()
			if err != nil {
				return "", err
			}
			return renderHumanBlocks(
				renderToonObject(
					extensionLogResetEventName,
					[]string{extensionLogStreamEpochKey, "log_count"},
					[]string{snapshot.StreamEpoch, strconv.Itoa(len(snapshot.Logs))},
				),
				logs,
			), nil
		},
	}
}
