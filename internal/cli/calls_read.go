package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type callReadFlags struct {
	workspace string
	states    []string
	caller    string
	cursor    string
	limit     int
}

func newCallListCommand(deps commandDeps) *cobra.Command {
	flags := &callReadFlags{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List durable calls",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
			if err != nil {
				return err
			}
			response, err := client.ListCalls(cmd.Context(), callListQuery{
				WorkspaceID: workspaceID, States: flags.states, Caller: flags.caller,
				Cursor: flags.cursor, Limit: flags.limit,
			})
			if err != nil {
				return withCallCommandExit(err)
			}
			return writeCommandOutput(cmd, callListBundle(response))
		},
	}
	addCallReadFlags(cmd, flags, true)
	configureProfileReadCommand(cmd, deps)
	return cmd
}

func newCallShowCommand(deps commandDeps) *cobra.Command {
	flags := &callReadFlags{}
	cmd := &cobra.Command{
		Use:   "show <call-id>",
		Short: "Show one durable call",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
			if err != nil {
				return err
			}
			record, err := client.GetCall(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return withCallCommandExit(err)
			}
			return writeCommandOutput(cmd, callDetailBundle(&record))
		},
	}
	addCallReadFlags(cmd, flags, false)
	configureSingleProfileReadCommand(cmd, deps)
	return cmd
}

func newCallResultCommand(deps commandDeps) *cobra.Command {
	flags := &callReadFlags{}
	cmd := &cobra.Command{
		Use:   "result <call-id>",
		Short: "Print the complete stored JSON result",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
			if err != nil {
				return err
			}
			result, err := client.GetCallResult(cmd.Context(), workspaceID, args[0])
			if err != nil {
				return withCallCommandExit(err)
			}
			return writeCommandOutput(cmd, callResultBundle(result))
		},
	}
	addCallReadFlags(cmd, flags, false)
	configureSingleProfileReadCommand(cmd, deps)
	return cmd
}

func newCallAwaitCommand(deps commandDeps) *cobra.Command {
	flags := &callReadFlags{}
	var timeout time.Duration
	var resume string
	cmd := &cobra.Command{
		Use:   "await <call-id>",
		Short: "Wait for a call checkpoint",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeout < 0 {
				return withCommandExitCode(2, errors.New("cli: --timeout must not be negative"))
			}
			client, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
			if err != nil {
				return err
			}
			response, err := client.AwaitCall(cmd.Context(), workspaceID, args[0], contract.AwaitCallsRequest{
				TimeoutMS: timeout.Milliseconds(), Resume: strings.TrimSpace(resume),
			})
			if err != nil {
				return withCallCommandExit(err)
			}
			if err := writeCommandOutput(cmd, callAwaitBundle(response)); err != nil {
				return err
			}
			if response.Outcome == "timeout" {
				return withCommandExitCode(3, errors.New("call await reached its timeout checkpoint"))
			}
			return nil
		},
	}
	addCallReadFlags(cmd, flags, false)
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "Maximum wait, clamped to 30m")
	cmd.Flags().StringVar(&resume, "resume", "", "Resume token from an earlier timeout")
	configureSingleProfileReadCommand(cmd, deps)
	return cmd
}

func newCallCancelCommand(deps commandDeps) *cobra.Command {
	flags := &callReadFlags{}
	var reason string
	cmd := &cobra.Command{
		Use:   "cancel <call-id>",
		Short: "Cancel a call idempotently",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
			if err != nil {
				return err
			}
			response, err := client.CancelCall(cmd.Context(), workspaceID, args[0], contract.CancelCallRequest{
				Reason: strings.TrimSpace(reason),
			})
			if err != nil {
				return withCallCommandExit(err)
			}
			return writeCommandOutput(cmd, callCancelBundle(response))
		},
	}
	addCallReadFlags(cmd, flags, false)
	cmd.Flags().StringVar(&reason, "reason", "", "Cancellation reason")
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func newCallPublishCommand(deps commandDeps) *cobra.Command {
	flags := &callReadFlags{}
	var channel string
	var threadID string
	cmd := &cobra.Command{
		Use:   "publish <call-id>",
		Short: "Publish completed call evidence to Network",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspaceID, err := resolveCallClient(cmd, deps, flags.workspace)
			if err != nil {
				return err
			}
			response, err := client.PublishCall(cmd.Context(), workspaceID, args[0], contract.PublishCallRequest{
				Channel: strings.TrimSpace(channel), ThreadID: strings.TrimSpace(threadID),
			})
			if err != nil {
				return withCallCommandExit(err)
			}
			return writeCommandOutput(cmd, callPublishBundle(response))
		},
	}
	addCallReadFlags(cmd, flags, false)
	cmd.Flags().StringVar(&channel, "channel", "", "Network channel")
	cmd.Flags().StringVar(&threadID, "thread", "", "Network thread; omit for the calls thread")
	if err := cmd.MarkFlagRequired("channel"); err != nil {
		panic(err)
	}
	configureProfileMutationCommand(cmd, deps)
	return cmd
}

func addCallReadFlags(cmd *cobra.Command, flags *callReadFlags, list bool) {
	cmd.Flags().StringVar(&flags.workspace, "workspace", "", "Override workspace name or id; omit for global scope")
	if !list {
		return
	}
	cmd.Flags().StringSliceVar(&flags.states, "state", nil, "Call states to include")
	cmd.Flags().StringVar(&flags.caller, "caller", "", "Caller session or run id")
	cmd.Flags().StringVar(&flags.cursor, "cursor", "", "Opaque page cursor")
	cmd.Flags().IntVar(&flags.limit, "limit", 50, "Page size (maximum 200)")
}
