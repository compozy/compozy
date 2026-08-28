package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	terminalpkg "github.com/compozy/compozy/internal/terminal"
	"github.com/spf13/cobra"
)

func terminalAgentClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
) (TerminalAgentClient, string, error) {
	base, workspaceID, err := terminalClientAndWorkspace(cmd, deps, workspaceRef)
	if err != nil {
		return nil, "", err
	}
	client, ok := base.(TerminalAgentClient)
	if !ok {
		return nil, "", newTerminalTransportError(
			terminalTransportCodeUnavailable, "terminal agent client is unavailable", nil,
		)
	}
	return client, workspaceID, nil
}

func newTerminalExecCommand(deps commandDeps) *cobra.Command {
	var workspace, cwd, yield, grep string
	var env []string
	var visible bool
	var tail int
	command := &cobra.Command{
		Use: "exec -- <command> [args...]", Short: "Run a command in the workspace",
		Args: terminalArgs(cobra.MinimumNArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tail < 0 {
				return terminalInvalidRequest("--tail must be greater than or equal to zero", nil)
			}
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			yieldMS, err := terminalYieldMilliseconds(yield)
			if err != nil {
				return err
			}
			environment, err := terminalEnvironment(env)
			if err != nil {
				return err
			}
			result, err := client.ExecTerminal(cmd.Context(), workspaceID, TerminalExecRequest{
				Command: args[0], Args: args[1:], Cwd: cwd, Env: environment, YieldMs: yieldMS, Visible: visible,
				Output: terminalpkg.OutputShape{Grep: grep},
			})
			if err != nil {
				return err
			}
			if tail > 0 {
				shaped, tailTruncated := terminalTailLines(result.Output, tail)
				result.Truncated = result.Truncated || tailTruncated
				result.Output = shaped
			}
			return writeCommandOutput(cmd, terminalExecBundle(result))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&cwd, "cwd", "", "Working directory under the workspace")
	command.Flags().StringVar(&yield, "yield", "10s", "Return control while a longer command continues")
	command.Flags().StringVar(&grep, "grep", "", "Keep matching output lines")
	command.Flags().StringArrayVar(&env, "env", nil, "Set an environment variable as K=V")
	command.Flags().BoolVar(&visible, "visible", false, "Run in a watchable interactive terminal")
	command.Flags().IntVar(&tail, "tail", 0, "Keep the last N output lines")
	configureProfileMutationCommand(command, deps)
	return command
}

func newTerminalSignalCommand(deps commandDeps) *cobra.Command {
	var workspace, signal string
	command := &cobra.Command{
		Use: "signal <id>", Short: "Send a signal without closing the terminal", Args: terminalArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			if err := client.SignalTerminal(cmd.Context(), workspaceID, args[0], signal); err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalSignalBundle(args[0], signal))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&signal, "signal", "INT", "Signal: INT, TERM, KILL, or HUP")
	configureProfileMutationCommand(command, deps)
	return command
}

func newTerminalInputRequestsCommand(deps commandDeps) *cobra.Command {
	var workspace, terminalID string
	command := &cobra.Command{
		Use: "input-requests", Short: "List pending terminal input requests", Args: terminalArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			requests, err := client.ListTerminalInputRequests(cmd.Context(), workspaceID, TerminalInputRequestQuery{
				TerminalID: terminalID,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalInputRequestsBundle(requests))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&terminalID, "terminal", "", "Filter by terminal id")
	configureProfileReadCommand(command, deps)
	return command
}

func newTerminalRespondCommand(deps commandDeps) *cobra.Command {
	var workspace, requestID, reason string
	var reject bool
	command := &cobra.Command{
		Use: "respond <id>", Short: "Answer or reject a pending input request", Args: terminalArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			resolvedID, err := resolveTerminalRequestID(cmd, client, workspaceID, args[0], requestID)
			if err != nil {
				return err
			}
			if reject {
				if err := client.RejectTerminalInputRequest(
					cmd.Context(), workspaceID, args[0], resolvedID, reason,
				); err != nil {
					return err
				}
				return writeCommandOutput(cmd, terminalRejectedInputBundle(resolvedID))
			}
			input, err := readTerminalResponse(cmd.InOrStdin())
			if err != nil {
				return err
			}
			delivered, redacted, err := client.AnswerTerminalInputRequest(
				cmd.Context(), workspaceID, args[0], resolvedID, input,
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalAnsweredInputBundle(resolvedID, delivered, redacted))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&requestID, "request", "", "Pending request id")
	command.Flags().BoolVar(&reject, "reject", false, "Reject the request without sending input")
	command.Flags().StringVar(&reason, "reason", "rejected by operator", "Rejection reason")
	configureProfileMutationCommand(command, deps)
	return command
}

func newTerminalJournalCommand(deps commandDeps) *cobra.Command {
	var workspace, actor, since, terminalID, cursor string
	var failed bool
	var limit int
	command := &cobra.Command{
		Use: "journal", Short: "Query the terminal command journal", Args: terminalArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			page, err := client.QueryTerminalJournal(cmd.Context(), workspaceID, TerminalJournalQuery{
				Actor: actor, Since: since, TerminalID: terminalID, Failed: failed, Limit: limit, Cursor: cursor,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalJournalBundle(page))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&actor, "actor", "", "Filter by human or agent")
	command.Flags().StringVar(&since, "since", "", "Filter by age, such as 24h")
	command.Flags().StringVar(&terminalID, "terminal", "", "Filter by terminal id")
	command.Flags().StringVar(&cursor, "cursor", "", "Continue from the previous page")
	command.Flags().BoolVar(&failed, "failed", false, "Show only failed commands")
	command.Flags().IntVar(&limit, "limit", 50, "Maximum journal rows")
	configureHistoricalProfileReadCommand(command, deps)
	return command
}

func newTerminalRecordCommand(deps commandDeps) *cobra.Command {
	command := &cobra.Command{
		Use: "record", Short: "Start or stop terminal recording", Args: terminalArgs(cobra.NoArgs),
	}
	command.AddCommand(newTerminalRecordingActionCommand(deps, terminalRecordingStartAction))
	command.AddCommand(newTerminalRecordingActionCommand(deps, "stop"))
	return command
}

func newTerminalRecordingActionCommand(deps commandDeps, action string) *cobra.Command {
	var workspace string
	command := &cobra.Command{
		Use:   action + " <id>",
		Short: strings.ToUpper(action[:1]) + action[1:] + " terminal recording",
		Args:  terminalArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			recording, err := client.ControlTerminalRecording(cmd.Context(), workspaceID, args[0], action)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalRecordingBundle(recording, action))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	configureProfileMutationCommand(command, deps)
	return command
}

func newTerminalQuoteCommand(deps commandDeps) *cobra.Command {
	var workspace, lines string
	command := &cobra.Command{
		Use: "quote <id>", Short: "Print a terminal excerpt for a conversation",
		Args: terminalArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, to, err := terminalLineRange(lines)
			if err != nil {
				return err
			}
			client, workspaceID, err := terminalAgentClientAndWorkspace(cmd, deps, workspace)
			if err != nil {
				return err
			}
			result, err := client.ReadTerminal(cmd.Context(), workspaceID, args[0], TerminalReadOptions{
				View: "lines", FromLine: from - 1, ToLine: to,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, terminalQuoteBundle(args[0], from, to, result.Content))
		},
	}
	addTerminalWorkspaceFlag(command, &workspace)
	command.Flags().StringVar(&lines, "lines", "", "Scrollback range, such as 120-124")
	configureSingleProfileReadCommand(command, deps)
	return command
}

func terminalYieldMilliseconds(value string) (int, error) {
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, terminalInvalidRequest("--yield must be a duration between 250ms and 30s", err)
	}
	if err := terminalpkg.ValidateExecYieldDuration(duration); err != nil {
		return 0, err
	}
	return int(duration.Milliseconds()), nil
}

func terminalEnvironment(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	environment := make(map[string]string, len(values))
	for _, value := range values {
		name, content, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, newTerminalTransportError(
				terminalTransportCodeInvalidRequest, fmt.Sprintf("invalid --env %q; expected K=V", value), nil,
			)
		}
		environment[name] = content
	}
	return environment, nil
}

func terminalLineRange(value string) (int, int, error) {
	fromText, toText, ok := strings.Cut(value, "-")
	if !ok {
		return 0, 0, terminalInvalidRequest("--lines must use A-B", nil)
	}
	from, fromErr := strconv.Atoi(fromText)
	to, toErr := strconv.Atoi(toText)
	if fromErr != nil || toErr != nil || from < 1 || to < from {
		return 0, 0, terminalInvalidRequest(
			"--lines must be a positive ordered range", errors.Join(fromErr, toErr),
		)
	}
	return from, to, nil
}

func resolveTerminalRequestID(
	cmd *cobra.Command,
	client TerminalAgentClient,
	workspaceID, terminalID, requestedID string,
) (string, error) {
	if strings.TrimSpace(requestedID) != "" {
		return strings.TrimSpace(requestedID), nil
	}
	requests, err := client.ListTerminalInputRequests(cmd.Context(), workspaceID, TerminalInputRequestQuery{
		TerminalID: terminalID,
	})
	if err != nil {
		return "", err
	}
	if len(requests.Pending) != 1 {
		if len(requests.Pending) > 1 {
			return "", terminalInvalidRequest(
				fmt.Sprintf("terminal %s has multiple pending input requests; select one with --request", terminalID),
				nil,
			)
		}
		return "", &terminalpkg.Error{
			Code: terminalpkg.ErrorCodeInputRequestNotFound,
			Message: fmt.Sprintf(
				"expected one pending request on %s, found %d",
				terminalID,
				len(requests.Pending),
			),
			Err: terminalpkg.ErrInputNotFound,
		}
	}
	return string(requests.Pending[0].ID), nil
}

func readTerminalResponse(input io.Reader) ([]byte, error) {
	line, err := bufio.NewReader(io.LimitReader(input, 64*1024+1)).ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("cli: read terminal response: %w", err)
	}
	if len(line) > 64*1024 {
		return nil, newTerminalTransportError(
			terminalTransportCodeInvalidRequest, "terminal response exceeds 64 KiB", nil,
		)
	}
	return []byte(strings.TrimSuffix(strings.TrimSuffix(string(line), "\n"), "\r")), nil
}

func terminalYieldRangeError(message string, cause error) error {
	return &terminalpkg.Error{
		Code: terminalpkg.ErrorCodeTimeoutOutOfRange, Message: message,
		Action: "fix_command", Err: errors.Join(terminalpkg.ErrUnsupported, cause),
	}
}

func terminalTailLines(content string, count int) (string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	truncated := count < len(lines)
	if truncated {
		lines = lines[len(lines)-count:]
	}
	return strings.Join(lines, "\n"), truncated
}
