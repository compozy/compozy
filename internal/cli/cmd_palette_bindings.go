package cli

import (
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/spf13/cobra"
)

type cmdPaletteStatusResult struct {
	Status string `json:"status"`
}

type cmdPaletteBindingMutationResult struct {
	Status       string   `json:"status"`
	Bound        []string `json:"bound"`
	UnboundOwner string   `json:"unbound_owner,omitempty"`
}

type cmdPaletteAliasMutationResult struct {
	Status string `json:"status"`
	Alias  string `json:"alias"`
}

type cmdPalettePinMutationResult struct {
	Status string `json:"status"`
	Pinned bool   `json:"pinned"`
}

type cmdPaletteBindingsResult struct {
	Effective       map[string]windowmanager.ShortcutBinding       `json:"effective"`
	Aliases         map[string]string                              `json:"aliases"`
	DormantDefaults []contract.SettingsWindowManagerDefaultPayload `json:"dormant_defaults"`
	Conflicts       []contract.SettingsWindowManagerMutationError  `json:"conflicts"`
}

func newCmdPaletteBindingsCommand(deps commandDeps) *cobra.Command {
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use: cliBindingsKey, Short: "List effective command bindings and aliases", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspace, err := cmdPaletteMutationClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			response, err := client.GetCmdPaletteBindings(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPaletteBindingsOutput(response))
		},
	}
	scope.add(cmd, false)
	return cmd
}

func newCmdPalettePinCommand(deps commandDeps, pinned bool) *cobra.Command {
	verb := cliUnpinVerb
	short := "Remove a command pin"
	if pinned {
		verb = cliPinVerb
		short = "Pin a command"
	}
	var scope cmdPaletteScopeFlags
	cmd := &cobra.Command{
		Use: verb + " <id>", Short: short, Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			commandID, err := requiredCmdPaletteID(args[0], "command ID")
			if err != nil {
				return err
			}
			client, workspace, err := cmdPaletteMutationClientAndWorkspace(cmd, deps, scope.workspace)
			if err != nil {
				return err
			}
			response, err := client.SetCmdPalettePin(cmd.Context(), workspace, commandID, pinned)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, cmdPaletteMutationOutput(cmdPalettePinMutationResult{
				Status: "ok", Pinned: response.Pinned,
			}))
		},
	}
	scope.add(cmd, false)
	return cmd
}

func cmdPaletteBindingsOutput(response contract.SettingsWindowManagerResponse) outputBundle {
	dormant := make([]contract.SettingsWindowManagerDefaultPayload, 0)
	for _, item := range response.ExtensionDefaults {
		if item.Dormant {
			dormant = append(dormant, item)
		}
	}
	result := cmdPaletteBindingsResult{
		Effective:       response.EffectiveShortcuts,
		Aliases:         response.Aliases,
		DormantDefaults: dormant,
		Conflicts:       []contract.SettingsWindowManagerMutationError{},
	}
	return outputBundle{
		jsonValue: result,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, result) },
		human: func() (string, error) {
			rows := make([][]string, 0, len(response.Commands))
			for _, command := range response.Commands {
				rows = append(rows, []string{
					command.ID,
					strings.Join(result.Effective[command.ID], ", "),
					result.Aliases[command.ID],
				})
			}
			return renderHumanTable(
				"Command palette bindings",
				[]string{cliCommandValue, "BINDINGS", "ALIAS"},
				rows,
			), nil
		},
		toon: func() (string, error) {
			encoded, err := json.Marshal(result)
			return string(encoded), err
		},
	}
}

func cmdPaletteMutationOutput(value any) outputBundle {
	renderJSON := func() (string, error) {
		encoded, err := json.Marshal(value)
		return string(encoded), err
	}
	return outputBundle{
		jsonValue: value,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, value) },
		human:     renderJSON,
		toon:      renderJSON,
	}
}
