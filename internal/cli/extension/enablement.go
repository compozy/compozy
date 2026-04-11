package extension

import (
	"context"
	"fmt"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/spf13/cobra"
)

func newEnableCommand(deps commandDeps) *cobra.Command {
	return newToggleCommand(deps, true)
}

func newDisableCommand(deps commandDeps) *cobra.Command {
	return newToggleCommand(deps, false)
}

func newToggleCommand(deps commandDeps, enable bool) *cobra.Command {
	use := "enable <name>"
	short := "Enable an extension on this machine"
	if !enable {
		use = "disable <name>"
		short = "Disable an extension on this machine"
	}

	return &cobra.Command{
		Use:          use,
		Short:        short,
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runToggleCommand(cmd, deps, args[0], enable)
		},
	}
}

func runToggleCommand(cmd *cobra.Command, deps commandDeps, rawName string, enable bool) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	name, err := normalizeExtensionName(rawName)
	if err != nil {
		return err
	}

	env, err := deps.resolveEnv(ctx)
	if err != nil {
		return err
	}

	result, err := deps.discoverAll(ctx, env)
	if err != nil {
		return err
	}

	entry, ok := findEffectiveExtension(result, name)
	if !ok {
		return fmt.Errorf("extension %q not found", name)
	}

	if err := toggleEntry(ctx, env.store, entry, enable); err != nil {
		return err
	}

	verb := "Enabled"
	if !enable {
		verb = "Disabled"
	}
	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"%s extension %q (%s).\n",
		verb,
		entry.Ref.Name,
		entry.Ref.Source,
	); err != nil {
		return fmt.Errorf("write toggle summary: %w", err)
	}
	return nil
}

func toggleEntry(
	ctx context.Context,
	store *extensions.EnablementStore,
	entry extensions.DiscoveredExtension,
	enable bool,
) error {
	switch entry.Ref.Source {
	case extensions.SourceBundled:
		if enable {
			return fmt.Errorf("bundled extension %q is always enabled", entry.Ref.Name)
		}
		return fmt.Errorf("bundled extension %q cannot be disabled", entry.Ref.Name)
	case extensions.SourceUser, extensions.SourceWorkspace:
		if enable {
			if err := store.Enable(ctx, entry.Ref); err != nil {
				return fmt.Errorf("enable extension %q: %w", entry.Ref.Name, err)
			}
			return nil
		}
		if err := store.Disable(ctx, entry.Ref); err != nil {
			return fmt.Errorf("disable extension %q: %w", entry.Ref.Name, err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported extension source %q", entry.Ref.Source)
	}
}
