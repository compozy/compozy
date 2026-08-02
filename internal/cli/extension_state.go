package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"

	"strings"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/spf13/cobra"
)

func loadExtensionRecords(cmd *cobra.Command, deps commandDeps, workspaceRef string) ([]ExtensionRecord, error) {
	client, running, err := daemonClientIfRunning(cmd.Context(), deps)
	if err != nil {
		return nil, err
	}
	if running {
		if strings.TrimSpace(workspaceRef) == "" {
			return client.ListExtensions(cmd.Context())
		}
		workspaceID, err := resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
		if err != nil {
			return nil, err
		}
		return client.ListExtensionsScoped(cmd.Context(), workspaceID)
	}
	if strings.TrimSpace(workspaceRef) != "" {
		return nil, errors.New("cli: workspace-scoped extension reads require a running daemon")
	}

	return withLocalExtensionRegistry(
		cmd.Context(),
		deps,
		func(_ *runtimeContext, registry localExtensionRegistry) ([]ExtensionRecord, error) {
			infos, err := registry.List()
			if err != nil {
				return nil, err
			}

			items := make([]ExtensionRecord, 0, len(infos))
			for _, info := range infos {
				items = append(items, localExtensionRecord(info, deps.now, deps.getenv))
			}
			return items, nil
		},
	)
}

func enableExtension(
	ctx context.Context,
	deps commandDeps,
	name string,
	request EnableExtensionRequest,
) (ExtensionEnableRecord, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionEnableRecord{}, err
	}
	return client.EnableExtension(ctx, name, request)
}

func disableExtension(ctx context.Context, deps commandDeps, name string) (ExtensionRecord, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionRecord{}, err
	}
	return client.DisableExtension(ctx, name)
}

func extensionStatus(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
	name string,
) (ExtensionRecord, error) {
	client, err := requireExtensionDaemonClient(cmd.Context(), deps)
	if err != nil {
		return ExtensionRecord{}, err
	}
	if strings.TrimSpace(workspaceRef) == "" {
		return client.ExtensionStatus(cmd.Context(), name)
	}
	workspaceID, err := resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
	if err != nil {
		return ExtensionRecord{}, err
	}
	return client.ExtensionStatusScoped(cmd.Context(), workspaceID, name)
}

func extensionProvenance(ctx context.Context, deps commandDeps, name string) (ExtensionProvenanceRecord, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return ExtensionProvenanceRecord{}, err
	}
	return client.ExtensionProvenance(ctx, name)
}

func requireExtensionDaemonClient(ctx context.Context, deps commandDeps) (DaemonClient, error) {
	client, running, err := daemonClientIfRunning(ctx, deps)
	if err != nil {
		return nil, err
	}
	if !running {
		return nil, errors.New("cli: extension operations require a running daemon")
	}
	return client, nil
}

func confirmExtensionUnverifiedInstall(cmd *cobra.Command, allowUnverified bool, yes bool) error {
	if !allowUnverified || yes {
		return nil
	}
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputHuman {
		return errors.New("cli: --allow-unverified requires --yes for structured output")
	}
	message := "This extension checksum is unverified. Continue with allow_unverified? [y/N] "
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), message); err != nil {
		return fmt.Errorf("cli: write extension trust prompt: %w", err)
	}
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("cli: read extension trust prompt: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != yesFlagName {
		return errors.New("cli: extension install declined")
	}
	return nil
}

func withLocalExtensionRegistry[T any](
	ctx context.Context,
	deps commandDeps,
	fn func(runtime *runtimeContext, registry localExtensionRegistry) (T, error),
) (result T, err error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return result, err
	}
	if err := deps.ensureHome(runtime.HomePaths); err != nil {
		return result, err
	}

	globalDB, err := openLocalGlobalDatabase(ctx, runtime.HomePaths.DatabaseFile, "extension")
	if err != nil {
		return result, err
	}
	defer func() {
		closeErr := globalDB.Close(ctx)
		if closeErr == nil {
			return
		}
		closeErr = fmt.Errorf("cli: close extension database %q: %w", runtime.HomePaths.DatabaseFile, closeErr)
		if err == nil {
			err = closeErr
			return
		}
		err = errors.Join(err, closeErr)
	}()

	return fn(runtime, extensionpkg.NewRegistry(globalDB.DB()))
}
