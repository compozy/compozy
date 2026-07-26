package cli

import (
	"strconv"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	onboardingCommandKey     = "onboarding"
	onboardingCompleteKey    = "complete"
	onboardingCompletedKey   = "completed"
	onboardingCompletedAtKey = "completed_at"
	onboardingResetKey       = "reset"
	onboardingCompletedAtLbl = "Completed At"
)

func newOnboardingCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   onboardingCommandKey,
		Short: "Inspect and manage first-run onboarding state",
	}
	cmd.AddCommand(newOnboardingStatusCommand(deps))
	cmd.AddCommand(newOnboardingCompleteCommand(deps))
	cmd.AddCommand(newOnboardingResetCommand(deps))
	return cmd
}

func newOnboardingStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   statusCommandKey,
		Short: "Show whether first-run onboarding has been completed",
		Example: `  # Show onboarding status
  agh onboarding status

  # Return machine-readable status for agents
  agh onboarding status -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.GetOnboardingStatus(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, onboardingBundle(response.Onboarding))
		},
	}
}

func newOnboardingCompleteCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   onboardingCompleteKey,
		Short: "Mark first-run onboarding as completed",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.CompleteOnboarding(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, onboardingBundle(response.Onboarding))
		},
	}
}

func newOnboardingResetCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   onboardingResetKey,
		Short: "Clear the onboarding completion flag so the wizard runs again",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			response, err := client.ResetOnboarding(cmd.Context())
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, onboardingBundle(response.Onboarding))
		},
	}
}

func onboardingBundle(status contract.OnboardingStatusPayload) outputBundle {
	return outputBundle{
		jsonValue: status,
		human: func() (string, error) {
			return renderHumanSection("Onboarding", []keyValue{
				{Label: "Completed", Value: strconv.FormatBool(status.Completed)},
				{Label: onboardingCompletedAtLbl, Value: stringOrDash(status.CompletedAt)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				onboardingCommandKey,
				[]string{onboardingCompletedKey, onboardingCompletedAtKey},
				[]string{strconv.FormatBool(status.Completed), status.CompletedAt},
			), nil
		},
	}
}
