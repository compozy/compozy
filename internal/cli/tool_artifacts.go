package cli

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/spf13/cobra"
)

type toolArtifactReadFlags struct {
	workspaceID string
	offset      int64
	limit       int64
}

func newToolArtifactCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "artifact",
		Short: "Read retained oversized tool results",
	}
	cmd.AddCommand(newToolArtifactReadCommand(deps))
	return cmd
}

func newToolArtifactReadCommand(deps commandDeps) *cobra.Command {
	var flags toolArtifactReadFlags
	cmd := &cobra.Command{
		Use:   "read <artifact_uri>",
		Short: "Read one workspace-scoped artifact page",
		Example: `  # Emit one page as structured JSON
  agh tool artifact read agh://tool-artifacts/art_<sha256> --workspace ws-1 -o json

  # Decode page bytes directly to stdout
  agh tool artifact read agh://tool-artifacts/art_<sha256> --workspace ws-1 --offset 65536`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			uri := strings.TrimSpace(args[0])
			if _, err := toolspkg.ParseToolArtifactURI(uri); err != nil {
				return writeToolCommandError(cmd, toolArtifactCommandError("artifact URI is invalid", err))
			}
			workspaceID := strings.TrimSpace(flags.workspaceID)
			if workspaceID == "" {
				return writeToolCommandError(cmd, toolArtifactCommandError(
					"workspace id is required",
					toolspkg.NewValidationError("workspace", toolspkg.ReasonSchemaInvalid, "workspace id is required"),
				))
			}
			if flags.offset < 0 || flags.limit < 0 || flags.limit > toolspkg.MaxToolArtifactPageBytes {
				return writeToolCommandError(cmd, toolArtifactCommandError(
					"artifact range is invalid",
					toolspkg.NewValidationError(
						"offset",
						toolspkg.ReasonSchemaInvalid,
						"offset and limit must be within supported bounds",
					),
				))
			}
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				page, err := client.ReadToolArtifact(
					cmd.Context(),
					workspaceID,
					uri,
					flags.offset,
					flags.limit,
				)
				if err != nil {
					return err
				}
				return writeToolArtifactPage(cmd, page)
			})
		},
	}
	cmd.Flags().StringVar(&flags.workspaceID, "workspace", "", "Workspace id that owns the artifact")
	cmd.Flags().Int64Var(&flags.offset, "offset", 0, "Zero-based byte offset")
	cmd.Flags().Int64Var(&flags.limit, "limit", 0, "Page size in bytes (default and maximum 65536)")
	return cmd
}

func toolArtifactCommandError(message string, err error) *toolCommandError {
	return toolValidationCommandError(toolspkg.ToolIDToolArtifactRead, message, err)
}

func writeToolArtifactPage(cmd *cobra.Command, page ToolArtifactPageRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputHuman {
		return writeCommandOutput(cmd, toolArtifactPageBundle(page))
	}
	decoded, err := base64.StdEncoding.DecodeString(page.DataBase64)
	if err != nil {
		return fmt.Errorf("cli: decode tool artifact page: %w", err)
	}
	if int64(len(decoded)) != page.Bytes {
		return fmt.Errorf("cli: decoded tool artifact page has %d bytes, expected %d", len(decoded), page.Bytes)
	}
	if _, err := io.Copy(cmd.OutOrStdout(), bytes.NewReader(decoded)); err != nil {
		return fmt.Errorf("cli: write tool artifact page: %w", err)
	}
	return nil
}

func toolArtifactPageBundle(page ToolArtifactPageRecord) outputBundle {
	return outputBundle{
		jsonValue: page,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, page)
		},
		human: func() (string, error) {
			return "", errors.New("cli: tool artifact human output is handled as exact bytes")
		},
		toon: func() (string, error) {
			return renderToonObject(
				"tool_artifact_page",
				[]string{"artifact_uri", "offset", "bytes", "total_bytes", "data_base64", "next_offset", "eof"},
				[]string{
					page.Artifact.URI,
					fmt.Sprintf("%d", page.Offset),
					fmt.Sprintf("%d", page.Bytes),
					fmt.Sprintf("%d", page.TotalBytes),
					page.DataBase64,
					fmt.Sprintf("%d", page.NextOffset),
					formatBool(page.EOF),
				},
			), nil
		},
	}
}
