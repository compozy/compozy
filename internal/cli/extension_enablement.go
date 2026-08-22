package cli

import (
	"context"
	"errors"
	"strconv"

	"github.com/spf13/cobra"
)

type extensionProfileEnablementClient interface {
	SetExtensionEnablement(context.Context, string, string, bool) (ExtensionEnablementRecord, error)
}

func setExtensionProfileEnablement(
	cmd *cobra.Command,
	deps commandDeps,
	name string,
	enabled bool,
) (ExtensionEnablementRecord, error) {
	client, err := requireExtensionDaemonClient(cmd.Context(), deps)
	if err != nil {
		return ExtensionEnablementRecord{}, err
	}
	enablement, ok := client.(extensionProfileEnablementClient)
	if !ok {
		return ExtensionEnablementRecord{}, errors.New("cli: extension profile enablement client is unavailable")
	}
	profiles, ok := client.(profileResolutionClient)
	if !ok {
		return ExtensionEnablementRecord{}, errors.New("cli: profile client is unavailable")
	}
	resolution, err := resolveCommandProfile(cmd.Context(), cmd, deps, profiles, client)
	if err != nil {
		return ExtensionEnablementRecord{}, err
	}
	return enablement.SetExtensionEnablement(cmd.Context(), name, resolution.Profile.Name, enabled)
}

func extensionEnablementBundle(item ExtensionEnablementRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, item) },
		human: func() (string, error) {
			state := "Disabled"
			if item.Enabled {
				state = "Enabled"
			}
			return state + " in profile " + item.Profile + ".", nil
		},
		toon: func() (string, error) {
			return renderToonObject("extension_enablement", []string{"profile", "enabled"}, []string{
				item.Profile, strconv.FormatBool(item.Enabled),
			}), nil
		},
	}
}
