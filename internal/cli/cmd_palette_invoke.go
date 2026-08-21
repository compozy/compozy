package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/spf13/cobra"
)

type cmdPaletteInvokeOutputRecord struct {
	Status       cmdpalette.InvokeStatus `json:"status"`
	Command      string                  `json:"command"`
	Result       json.RawMessage         `json:"result,omitempty"`
	ApprovalID   string                  `json:"approval_id,omitempty"`
	InvocationID string                  `json:"invocation_id,omitempty"`
	Message      string                  `json:"message,omitempty"`
}

func newCmdPaletteInvokeCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	var rawArgs []string
	cmd := &cobra.Command{
		Use:   "invoke <id>",
		Short: "Invoke one command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			catalog, err := client.ListCmdPaletteCommands(cmd.Context(), workspace, scope.clientID)
			if err != nil {
				return err
			}
			command, ok := findCmdPaletteCommand(catalog.Commands, commandID)
			if !ok {
				return cmdPaletteCommandNotFoundError(commandID)
			}
			invokeArgs, err := parseCmdPaletteArgs(command, rawArgs)
			if err != nil {
				return withCommandExitCode(2, err)
			}
			result, err := client.InvokeCmdPaletteCommand(cmd.Context(), commandID, contract.CmdPaletteInvokeRequest{
				Workspace: workspace, Args: invokeArgs, Client: strings.TrimSpace(scope.clientID),
			})
			if err != nil {
				return cmdPaletteInvokeError(err)
			}
			output := cmdPaletteInvokeOutputRecord{
				Status: result.Status, Command: commandID, Result: result.Result,
				ApprovalID: result.ApprovalID, InvocationID: result.InvocationID,
			}
			if result.Status == cmdpalette.InvokeStatusApprovalPending {
				output.Message = "destructive command requires approval"
			}
			return writeCommandOutput(cmd, cmdPaletteInvokeOutput(output))
		},
	}
	scope.add(cmd, true)
	cmd.Flags().StringArrayVar(&rawArgs, cmdPaletteArgFlag, nil, "Set one argument as name=value (repeatable)")
	return cmd
}

func parseCmdPaletteArgs(
	command contract.CmdPaletteCommand,
	values []string,
) (map[string]any, error) {
	types := make(map[string]cmdpalette.ArgumentType, len(command.Arguments))
	for _, argument := range command.Arguments {
		types[argument.Name] = argument.Type
	}
	parsed := make(map[string]any, len(values))
	for _, value := range values {
		name, raw, found := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		if !found || name == "" {
			return nil, fmt.Errorf("cli: --arg must use name=value")
		}
		if types[name] == cmdpalette.ArgumentTypeCheckbox {
			boolean, err := strconv.ParseBool(strings.TrimSpace(raw))
			if err != nil {
				return nil, fmt.Errorf("cli: --arg %s must be true or false: %w", name, err)
			}
			parsed[name] = boolean
			continue
		}
		parsed[name] = raw
	}
	return parsed, nil
}

func cmdPaletteInvokeError(err error) error {
	paletteErr, ok := errors.AsType[*cmdPaletteAPIError](err)
	if ok && paletteErr.statusCode == 422 {
		return withCommandExitCode(2, err)
	}
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	if ok && apiErr.statusCode == 422 {
		return withCommandExitCode(2, err)
	}
	return withCommandExitCode(1, err)
}

func cmdPaletteInvokeOutput(record cmdPaletteInvokeOutputRecord) outputBundle {
	return outputBundle{
		jsonValue: record,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, record) },
		human: func() (string, error) {
			encoded, err := json.Marshal(record)
			if err != nil {
				return "", fmt.Errorf("cli: encode command invocation output: %w", err)
			}
			return string(encoded), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"invocation",
				[]string{automationStatusKey, configCommandKey, "approval_id", "invocation_id", "message"},
				[]string{
					string(record.Status), record.Command, record.ApprovalID, record.InvocationID, record.Message,
				},
			), nil
		},
	}
}
