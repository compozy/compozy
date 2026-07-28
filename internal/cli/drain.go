package cli

import "github.com/spf13/cobra"

const (
	drainCommandKey   = "drain"
	undrainCommandKey = "undrain"
)

func newDrainCommand(deps commandDeps) *cobra.Command {
	return newDrainStateCommand(
		drainCommandKey,
		"Stop admitting new work while in-flight work completes",
		deps,
		func(client DaemonClient, cmd *cobra.Command) (DrainStatusRecord, error) {
			return client.Drain(cmd.Context())
		},
	)
}

func newUndrainCommand(deps commandDeps) *cobra.Command {
	return newDrainStateCommand(
		undrainCommandKey,
		"Resume admission of new work",
		deps,
		func(client DaemonClient, cmd *cobra.Command) (DrainStatusRecord, error) {
			return client.Undrain(cmd.Context())
		},
	)
}

func newDrainStateCommand(
	use string,
	short string,
	deps commandDeps,
	update func(DaemonClient, *cobra.Command) (DrainStatusRecord, error),
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
				{Label: "State", Value: string(result.State)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("drain", []string{stateKey}, []string{string(result.State)}), nil
		},
	}
}
