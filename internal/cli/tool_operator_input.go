package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/spf13/cobra"
)

func runToolCommand(cmd *cobra.Command, deps commandDeps, run func(DaemonClient) error) error {
	client, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	err = run(client)
	if err != nil {
		return writeToolCommandError(cmd, err)
	}
	return nil
}

func parseToolIDArg(raw string) (toolspkg.ToolID, error) {
	id := toolspkg.ToolID(strings.TrimSpace(raw))
	if err := id.Validate(); err != nil {
		return "", toolValidationCommandError("", "tool id is invalid", err)
	}
	return id, nil
}

func parseToolsetIDArg(raw string) (toolspkg.ToolsetID, error) {
	id := toolspkg.ToolsetID(strings.TrimSpace(raw))
	if err := id.Validate(); err != nil {
		return "", toolValidationCommandError("", "toolset id is invalid", err)
	}
	return id, nil
}

func resolveToolInvokeInput(cmd *cobra.Command, flags toolInvokeFlags) (json.RawMessage, error) {
	inlineChanged := cmd.Flags().Lookup("input") != nil && cmd.Flags().Lookup("input").Changed
	inputFile := strings.TrimSpace(flags.inputFile)
	if inlineChanged && inputFile != "" {
		return nil, toolspkg.NewValidationError(
			"input",
			toolspkg.ReasonSchemaInvalid,
			"provide --input or --input-file, not both",
		)
	}
	if inlineChanged {
		return parseToolInputJSON("input", flags.input)
	}
	if inputFile != "" {
		return readToolInputFile(cmd, inputFile)
	}
	stdinContent, err := readOptionalCommandInput(cmd.InOrStdin())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(stdinContent) != "" {
		return parseToolInputJSON("stdin", stdinContent)
	}
	return json.RawMessage(`{}`), nil
}

func resolveToolApprovalInput(cmd *cobra.Command, flags toolApprovalFlags) (json.RawMessage, error) {
	inlineChanged := cmd.Flags().Lookup("input") != nil && cmd.Flags().Lookup("input").Changed
	inputFile := strings.TrimSpace(flags.inputFile)
	if inlineChanged && inputFile != "" {
		return nil, toolspkg.NewValidationError(
			"input",
			toolspkg.ReasonSchemaInvalid,
			"provide --input or --input-file, not both",
		)
	}
	if inlineChanged {
		return parseToolInputJSON("input", flags.input)
	}
	if inputFile != "" {
		return readToolInputFile(cmd, inputFile)
	}
	return nil, nil
}

func readToolInputFile(cmd *cobra.Command, path string) (json.RawMessage, error) {
	var payload []byte
	var err error
	if path == "-" {
		payload, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("cli: read tool input stdin: %w", err)
		}
	} else {
		payload, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("cli: read tool input file: %w", err)
		}
	}
	return parseToolInputJSON("input", string(payload))
}

func parseToolInputJSON(field string, raw string) (json.RawMessage, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, toolspkg.NewValidationError(field, toolspkg.ReasonSchemaInvalid, "input JSON is required")
	}
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, []byte(trimmed)); err != nil {
		return nil, toolspkg.NewValidationError(field, toolspkg.ReasonSchemaInvalid, "input must be valid JSON")
	}
	return json.RawMessage(compacted.String()), nil
}
