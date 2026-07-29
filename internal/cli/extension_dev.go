package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/filesnap"
	"github.com/compozy/compozy/internal/sse"
	"github.com/spf13/cobra"
)

type extensionDevClient interface {
	DaemonClient
	DevExtension(context.Context, string, DevLinkExtensionRequest) (ExtensionRecord, error)
	ReloadDevExtension(context.Context, string, string, ReloadExtensionRequest) (ExtensionRecord, error)
	ExtensionLogs(context.Context, string, string, int64) ([]ExtensionLogRecord, error)
	StreamExtensionLogs(context.Context, string, string, int64, SSEHandler) error
	RemoveDevExtension(context.Context, string, string) (ManagedExtensionRemoveRecord, error)
}

func newExtensionDevCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var watch bool
	command := &cobra.Command{
		Use:   "dev [directory]",
		Short: "Build and link an extension to the current workspace",
		Args:  optionalDirectoryArg,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, workspace, sourceDir, err := resolveExtensionDevContext(cmd, deps, workspaceRef, args)
			if err != nil {
				return err
			}
			result, err := extensionpkg.BuildBundle(cmd.Context(), extensionpkg.BuildRequest{SourceDir: sourceDir})
			if err != nil {
				return err
			}
			item, err := client.DevExtension(cmd.Context(), workspace.ID, DevLinkExtensionRequest{
				OriginPath:     sourceDir,
				GenerationHash: result.GenerationHash,
			})
			if err != nil {
				return err
			}
			if err := writeCommandOutput(cmd, extensionSuccessBundle(extensionDevVerb, item)); err != nil {
				return err
			}
			if !watch {
				return nil
			}
			return watchExtensionSource(cmd, deps, client, workspace.ID, item.Name, sourceDir)
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	command.Flags().BoolVar(&watch, "watch", false, "Build and reload when source files change")
	return command
}

func newExtensionReloadCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	command := &cobra.Command{
		Use:   "reload <name> [directory]",
		Short: "Build and atomically reload a dev-linked extension",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 || len(args) > 2 || strings.TrimSpace(args[0]) == "" {
				return errors.New("cli: reload requires a name and optional directory")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			directoryArgs := args[1:]
			client, workspace, sourceDir, err := resolveExtensionDevContext(
				cmd,
				deps,
				workspaceRef,
				directoryArgs,
			)
			if err != nil {
				return err
			}
			result, err := extensionpkg.BuildBundle(cmd.Context(), extensionpkg.BuildRequest{SourceDir: sourceDir})
			if err != nil {
				return err
			}
			item, err := client.ReloadDevExtension(
				cmd.Context(),
				workspace.ID,
				args[0],
				ReloadExtensionRequest{GenerationHash: result.GenerationHash},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionSuccessBundle(extensionReloadVerb, item))
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	return command
}

func newExtensionLogsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef string
	var follow bool
	var global bool
	var after int64
	command := &cobra.Command{
		Use:   "logs <name>",
		Short: "Read redacted logs for one extension instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireExtensionDevClient(cmd.Context(), deps)
			if err != nil {
				return err
			}
			workspace := ""
			if !global {
				workspace, err = resolveCLIWorkspaceRouteRef(cmd, deps, client, workspaceRef)
				if err != nil {
					return err
				}
			} else if strings.TrimSpace(workspaceRef) != "" {
				return errors.New("cli: --global and --workspace are mutually exclusive")
			}
			if follow {
				return streamExtensionLogs(cmd, client, workspace, args[0], after)
			}
			entries, err := client.ExtensionLogs(cmd.Context(), workspace, args[0], after)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, extensionLogsBundle(entries))
		},
	}
	command.Flags().StringVar(&workspaceRef, workspaceFlagName, "", "Override workspace context")
	command.Flags().BoolVar(&follow, "follow", false, "Stream new log entries")
	command.Flags().BoolVar(&global, "global", false, "Read the published global instance")
	command.Flags().Int64Var(&after, "after", 0, "Read entries after this sequence")
	return command
}

func resolveExtensionDevContext(
	cmd *cobra.Command,
	deps commandDeps,
	workspaceRef string,
	directoryArgs []string,
) (extensionDevClient, workspaceResolution, string, error) {
	client, err := requireExtensionDevClient(cmd.Context(), deps)
	if err != nil {
		return nil, workspaceResolution{}, "", err
	}
	workspace, err := resolveCommandWorkspace(
		cmd.Context(),
		cmd,
		deps,
		client,
		workspaceResolutionRequest{FlagRef: workspaceRef},
	)
	if err != nil {
		return nil, workspaceResolution{}, "", err
	}
	sourceDir, err := extensionAuthoringDirectory(deps, directoryArgs)
	if err != nil {
		return nil, workspaceResolution{}, "", err
	}
	absSource, err := filepath.Abs(sourceDir)
	if err != nil {
		return nil, workspaceResolution{}, "", fmt.Errorf("cli: resolve extension source: %w", err)
	}
	return client, workspace, absSource, nil
}

func requireExtensionDevClient(ctx context.Context, deps commandDeps) (extensionDevClient, error) {
	client, err := requireExtensionDaemonClient(ctx, deps)
	if err != nil {
		return nil, err
	}
	dev, ok := client.(extensionDevClient)
	if !ok {
		return nil, errors.New("cli: daemon client does not support extension development")
	}
	return dev, nil
}

func watchExtensionSource(
	cmd *cobra.Command,
	deps commandDeps,
	client extensionDevClient,
	workspaceID string,
	name string,
	sourceDir string,
) error {
	previous, err := snapshotExtensionSource(cmd.Context(), sourceDir)
	if err != nil {
		return err
	}
	cfg, err := deps.loadConfig()
	if err != nil {
		return err
	}
	interval := cfg.Extensions.Dev.WatchInterval
	if interval <= 0 {
		return errors.New("cli: extensions.dev.watch_interval must be positive")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-cmd.Context().Done():
			return cmd.Context().Err()
		case <-ticker.C:
			current, snapshotErr := snapshotExtensionSource(cmd.Context(), sourceDir)
			if snapshotErr != nil {
				return snapshotErr
			}
			if filesnap.Equal(previous, current) {
				continue
			}
			result, buildErr := extensionpkg.BuildBundle(cmd.Context(), extensionpkg.BuildRequest{SourceDir: sourceDir})
			if buildErr != nil {
				return buildErr
			}
			item, reloadErr := client.ReloadDevExtension(
				cmd.Context(),
				workspaceID,
				name,
				ReloadExtensionRequest{GenerationHash: result.GenerationHash},
			)
			if reloadErr != nil {
				return reloadErr
			}
			bundle := extensionSuccessBundle(extensionReloadVerb, item)
			if outputErr := writeCommandOutput(cmd, bundle); outputErr != nil {
				return outputErr
			}
			previous = current
		}
	}
}

func snapshotExtensionSource(ctx context.Context, root string) (map[string]filesnap.Snapshot, error) {
	snapshots := map[string]filesnap.Snapshot{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && path != root {
			switch entry.Name() {
			case ".git", "dist", "node_modules":
				return filepath.SkipDir
			}
		}
		if entry.IsDir() {
			return nil
		}
		snapshot, err := filesnap.FromPath(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return err
		}
		snapshots[path] = snapshot
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cli: snapshot extension source %q: %w", root, err)
	}
	return snapshots, nil
}

func streamExtensionLogs(
	cmd *cobra.Command,
	client extensionDevClient,
	workspace string,
	name string,
	after int64,
) error {
	return client.StreamExtensionLogs(cmd.Context(), workspace, name, after, func(event sse.Event) error {
		if event.Event != "extension_log" {
			return nil
		}
		var entry ExtensionLogRecord
		if err := json.Unmarshal(event.Data, &entry); err != nil {
			return fmt.Errorf("cli: decode extension log event: %w", err)
		}
		return writeCommandOutput(cmd, extensionLogsBundle([]ExtensionLogRecord{entry}))
	})
}
