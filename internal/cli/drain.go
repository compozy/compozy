package cli

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"
)

const (
	drainCommandKey   = "drain"
	undrainCommandKey = "undrain"
)

type drainClient interface {
	Drain(context.Context) (DrainStatusRecord, error)
	Undrain(context.Context) (DrainStatusRecord, error)
}

func newDrainCommand(deps commandDeps) *cobra.Command {
	return newDrainStateCommand(
		drainCommandKey,
		"Stop admitting new work while in-flight work completes",
		deps,
		func(client drainClient, cmd *cobra.Command) (DrainStatusRecord, error) {
			return client.Drain(cmd.Context())
		},
	)
}

func newUndrainCommand(deps commandDeps) *cobra.Command {
	return newDrainStateCommand(
		undrainCommandKey,
		"Resume admission of new work",
		deps,
		func(client drainClient, cmd *cobra.Command) (DrainStatusRecord, error) {
			return client.Undrain(cmd.Context())
		},
	)
}

func newDrainStateCommand(
	use string,
	short string,
	deps commandDeps,
	update func(drainClient, *cobra.Command) (DrainStatusRecord, error),
) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := update(client, cmd)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, drainStatusBundle(result))
		},
	}
}

func drainStatusBundle(result DrainStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, result)
		},
		human: func() (string, error) {
			return renderHumanSection("Daemon admission", []keyValue{
				{Label: networkStateValue, Value: string(result.State)},
				{Label: "Active session executions", Value: strconv.Itoa(result.ActiveSessionExecutions)},
				{Label: "Active task claims", Value: strconv.Itoa(result.ActiveTaskClaims)},
				{Label: "Safe to stop", Value: strconv.FormatBool(result.SafeToStop)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"drain",
				[]string{stateKey, "active_session_executions", "active_task_claims", "safe_to_stop"},
				[]string{
					string(result.State),
					strconv.Itoa(result.ActiveSessionExecutions),
					strconv.Itoa(result.ActiveTaskClaims),
					strconv.FormatBool(result.SafeToStop),
				},
			), nil
		},
	}
}
