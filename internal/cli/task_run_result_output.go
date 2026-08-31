package cli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/spf13/cobra"
)

func newTaskRunResultCommand(deps commandDeps) *cobra.Command {
	var offset int64
	var limit int64
	cmd := &cobra.Command{
		Use:   "result <run-id>",
		Short: "Read one exact task-run result page",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if offset < 0 || limit < 0 || limit > taskpkg.MaxRunResultPageBytes {
				return errors.New("task run result offset or limit is outside supported bounds")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			page, err := client.ReadTaskRunResult(cmd.Context(), args[0], offset, limit)
			if err != nil {
				return err
			}
			return writeTaskRunResultPage(cmd, page)
		},
	}
	cmd.Flags().Int64Var(&offset, "offset", 0, "Zero-based byte offset")
	cmd.Flags().Int64Var(&limit, "limit", 0, "Page size in bytes (default and maximum 65536)")
	configureProfileReadCommand(cmd, deps)
	return cmd
}

func writeTaskRunResultPage(cmd *cobra.Command, page TaskRunResultPageRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputHuman {
		return writeCommandOutput(cmd, taskRunResultPageBundle(page))
	}
	decoded, err := base64.StdEncoding.DecodeString(page.DataBase64)
	if err != nil {
		return fmt.Errorf("cli: decode task run result page: %w", err)
	}
	if int64(len(decoded)) != page.Bytes {
		return fmt.Errorf("cli: decoded task run result page has %d bytes, expected %d", len(decoded), page.Bytes)
	}
	if _, err := io.Copy(cmd.OutOrStdout(), bytes.NewReader(decoded)); err != nil {
		return fmt.Errorf("cli: write task run result page: %w", err)
	}
	return nil
}

func taskRunResultPageBundle(page TaskRunResultPageRecord) outputBundle {
	return outputBundle{
		jsonValue: page,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, page)
		},
		human: func() (string, error) {
			return "", errors.New("cli: task run result human output is handled as exact bytes")
		},
		toon: func() (string, error) {
			nextOffset := ""
			if page.NextOffset != nil {
				nextOffset = fmt.Sprintf("%d", *page.NextOffset)
			}
			return renderToonObject(
				"task_run_result_page",
				[]string{
					"run_id",
					"result_ref",
					"offset",
					cliBytesKey,
					"total_bytes",
					"data_base64",
					"next_offset",
					"eof",
				},
				[]string{
					page.RunID,
					page.ResultRef,
					fmt.Sprintf("%d", page.Offset),
					fmt.Sprintf("%d", page.Bytes),
					fmt.Sprintf("%d", page.TotalBytes),
					page.DataBase64,
					nextOffset,
					formatBool(page.EOF),
				},
			), nil
		},
	}
}
