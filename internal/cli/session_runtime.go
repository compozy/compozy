package cli

import (
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/spf13/cobra"
)

type sessionRuntimeCommandFlags struct {
	provider         string
	model            string
	reasoningEffort  string
	speed            string
	acpOptions       []string
	acpToggles       []string
	expectedRevision int64
}

func newSessionRuntimeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{Use: loopRuntimeKey, Short: "Manage the default runtime for future session prompts"}
	cmd.AddCommand(newSessionRuntimeSetCommand(deps), newSessionRuntimeClearCommand(deps))
	return cmd
}

func newSessionRuntimeSetCommand(deps commandDeps) *cobra.Command {
	var flags sessionRuntimeCommandFlags
	cmd := &cobra.Command{
		Use:   "set <id>",
		Short: "Select the default runtime for future prompts",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := strings.TrimSpace(flags.provider)
			if provider == "" {
				return errors.New("cli: --provider is required")
			}
			speed := speedpkg.SpeedNormal
			if cmd.Flags().Changed("speed") {
				parsed, err := speedpkg.Parse(flags.speed)
				if err != nil {
					return err
				}
				speed = parsed
			}
			acpOptions, err := parseACPFlags(flags.acpOptions, flags.acpToggles)
			if err != nil {
				return err
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			revision, err := sessionRuntimeExpectedRevision(cmd, client, args[0], flags.expectedRevision)
			if err != nil {
				return err
			}
			info, err := client.SetSessionRuntime(cmd.Context(), args[0], SetSessionRuntimeRequest{
				Runtime: contract.PromptRuntimeSelectionPayload{
					Provider:        provider,
					Model:           strings.TrimSpace(flags.model),
					ReasoningEffort: contract.ReasoningEffort(strings.TrimSpace(flags.reasoningEffort)),
					Speed:           speed,
					ACPOptions:      acpOptions,
				},
				ExpectedRevision: &revision,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&info, deps.now))
		},
	}
	cmd.Flags().StringVar(&flags.provider, sessionProviderKey, "", "Runtime provider")
	cmd.Flags().StringVar(&flags.model, "model", "", "Runtime model")
	cmd.Flags().StringVar(&flags.reasoningEffort, "reasoning-effort", "", "Runtime reasoning effort")
	cmd.Flags().StringVar(&flags.speed, "speed", string(speedpkg.SpeedNormal), "Runtime speed (normal|fast)")
	bindACPOptionFlags(
		cmd,
		&flags.acpOptions,
		&flags.acpToggles,
		"ACP select as id=value (repeatable)",
		"ACP boolean as id=true|false (repeatable)",
	)
	cmd.Flags().Int64Var(&flags.expectedRevision, "expected-revision", 0, "Current runtime selection revision")
	return cmd
}

func newSessionRuntimeClearCommand(deps commandDeps) *cobra.Command {
	var expectedRevision int64
	cmd := &cobra.Command{
		Use:   "clear <id>",
		Short: "Clear the default runtime for future prompts",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			revision, err := sessionRuntimeExpectedRevision(cmd, client, args[0], expectedRevision)
			if err != nil {
				return err
			}
			info, err := client.ClearSessionRuntime(cmd.Context(), args[0], revision)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&info, deps.now))
		},
	}
	cmd.Flags().Int64Var(&expectedRevision, "expected-revision", 0, "Current runtime selection revision")
	return cmd
}

func sessionRuntimeExpectedRevision(
	cmd *cobra.Command,
	client sessionClientAPI,
	sessionID string,
	flagValue int64,
) (int64, error) {
	if cmd.Flags().Changed("expected-revision") {
		if flagValue < 0 {
			return 0, errors.New("cli: --expected-revision must not be negative")
		}
		return flagValue, nil
	}
	info, err := client.GetSession(cmd.Context(), sessionID)
	if err != nil {
		return 0, err
	}
	return info.Runtime.SelectionRevision, nil
}
