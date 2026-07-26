package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/spf13/cobra"
)

const layoutCommandKey = "layout"

func newLayoutCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   layoutCommandKey,
		Short: "Inspect, preview, and change workspace layouts",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newLayoutGetCommand(deps))
	cmd.AddCommand(newLayoutPreviewCommand(deps))
	cmd.AddCommand(newLayoutArrangeCommand(deps))
	cmd.AddCommand(newLayoutResizeCommand(deps))
	cmd.AddCommand(newLayoutBalanceCommand(deps))
	cmd.AddCommand(newLayoutUndoCommand(deps))
	cmd.AddCommand(newLayoutRedoCommand(deps))
	cmd.AddCommand(newLayoutExportCommand(deps))
	cmd.AddCommand(newLayoutValidateCommand(deps))
	cmd.AddCommand(newLayoutApplyCommand(deps))
	cmd.AddCommand(newLayoutWatchCommand(deps))
	return cmd
}

func newLayoutGetCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use: cliGetKey, Short: "Get the authoritative topology and revision", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			snapshot, err := client.GetWindowManagerSnapshot(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(
				cmd,
				windowManagerJSONBundle("window_manager_snapshot", "Window manager snapshot", snapshot),
			)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	return cmd
}

func newLayoutPreviewCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var commandID, inlinePayload, payloadFile string
	cmd := &cobra.Command{
		Use:   windowManagerPreviewKey,
		Short: "Preview any semantic window-manager command without writing",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			id := contract.WindowManagerCommandID(strings.TrimSpace(commandID))
			if !isWindowManagerCommandID(id) {
				return newWindowManagerCLIValidationError(
					windowManagerCLIValidationUnsupported,
					"command",
					fmt.Errorf("cli: unsupported --command %q", commandID),
				)
			}
			if windowManagerCommandRequiresClient(id) {
				if err := requiredWindowManagerClient(flags); err != nil {
					return err
				}
			}
			payload, err := readWindowManagerPayload(cmd, inlinePayload, payloadFile)
			if err != nil {
				return err
			}
			request, err := flags.request(cmd, id, payload)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			preview, err := client.PreviewWindowManagerCommand(cmd.Context(), string(request.WorkspaceID), request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerPreviewBundle(preview))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&commandID, "command", "", "Semantic command ID, such as window.move")
	cmd.Flags().StringVar(&inlinePayload, windowManagerPayloadFlag, "", "Command payload as a JSON object")
	cmd.Flags().
		StringVar(&payloadFile, windowManagerPayloadFile, "", "Read the command payload from a file or - for stdin")
	return cmd
}

func newLayoutExportCommand(deps commandDeps) *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use: "export", Short: "Export a history-free declarative layout document", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			document, err := client.ExportWindowManagerLayout(cmd.Context(), workspace)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerJSONBundle("window_layout", "Window layout", document))
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	return cmd
}

func newLayoutValidateCommand(deps commandDeps) *cobra.Command {
	var workspace, file string
	cmd := &cobra.Command{
		Use: "validate", Short: "Validate a declarative layout without writing it", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
			if err != nil {
				return err
			}
			document, err := readWindowManagerDocument(cmd, file)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			validation, err := client.ValidateWindowManagerLayout(
				cmd.Context(),
				workspace,
				contract.WindowManagerLayoutValidationRequest{
					WorkspaceID: windowmanager.WorkspaceID(workspace), Document: document,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerValidationBundle(validation))
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().StringVar(&file, windowManagerDocumentFile, "", "Layout document file or - for stdin")
	return cmd
}

func newLayoutApplyCommand(deps commandDeps) *cobra.Command {
	var flags windowManagerMutationFlags
	var file string
	cmd := &cobra.Command{
		Use: "apply", Short: "Atomically replace topology from a validated layout document", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			document, err := readWindowManagerDocument(cmd, file)
			if err != nil {
				return err
			}
			semantic, err := flags.request(
				cmd,
				contract.WindowManagerCommandLayoutReplace,
				contract.WindowManagerReplaceLayoutPayload{Document: document},
			)
			if err != nil {
				return err
			}
			client, err := windowManagerClientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ApplyWindowManagerLayout(
				cmd.Context(),
				string(semantic.WorkspaceID),
				contract.WindowManagerLayoutReplaceRequest{
					WorkspaceID: semantic.WorkspaceID, ExpectedRevision: semantic.ExpectedRevision,
					ClientID: semantic.ClientID, Actor: semantic.Actor, Origin: semantic.Origin, Document: document,
				},
			)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, windowManagerResultBundle(result))
		},
	}
	flags.add(cmd)
	cmd.Flags().StringVar(&file, windowManagerDocumentFile, "", "Layout document file or - for stdin")
	return cmd
}

func newLayoutWatchCommand(deps commandDeps) *cobra.Command {
	var workspace, clientID string
	var after uint64
	cmd := &cobra.Command{
		Use: "watch", Short: "Watch topology events and optional client presentation frames", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runLayoutWatchCommand(cmd, deps, workspace, clientID, after)
		},
	}
	cmd.Flags().StringVar(&workspace, windowManagerWorkspaceFlag, "", "Workspace ID")
	cmd.Flags().Uint64Var(&after, "after-revision", 0, "Resume strictly after this revision")
	cmd.Flags().StringVar(&clientID, windowManagerClientFlag, "", "Registered client ID for presentation frames")
	return cmd
}

func runLayoutWatchCommand(
	cmd *cobra.Command,
	deps commandDeps,
	workspace string,
	clientID string,
	after uint64,
) error {
	workspace, err := requiredWindowManagerFlag(workspace, windowManagerWorkspaceFlag)
	if err != nil {
		return err
	}
	format, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if format != OutputHuman && format != OutputJSONL {
		return errors.New("cli: layout watch supports human or jsonl output")
	}
	afterRevision, err := optionalWindowManagerRevision(cmd, "after-revision", after)
	if err != nil {
		return err
	}
	boundClientID, err := optionalWindowManagerID[windowmanager.ClientID](cmd, windowManagerClientFlag, clientID)
	if err != nil {
		return err
	}
	client, err := windowManagerClientFromDeps(deps)
	if err != nil {
		return err
	}
	return client.WatchWindowManager(
		cmd.Context(),
		workspace,
		boundClientID,
		afterRevision,
		windowManagerStreamHandlers(cmd, format),
	)
}

func windowManagerStreamHandlers(cmd *cobra.Command, format OutputFormat) WindowManagerStreamHandlers {
	return WindowManagerStreamHandlers{
		Snapshot: func(frame contract.WindowManagerSnapshotFrame) error {
			return writeWindowManagerSnapshotFrame(cmd, format, frame)
		},
		Event: func(frame contract.WindowManagerEventFrame) error {
			if format == OutputJSONL {
				return writeJSONLine(cmd, frame)
			}
			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"event\trevision=%d\tcommand=%s\n",
				frame.Revision,
				frame.Event.CommandID,
			)
			return err
		},
		Client: func(frame contract.WindowManagerClientFrame) error {
			return writeWindowManagerClientFrame(cmd, format, frame)
		},
	}
}

func writeWindowManagerSnapshotFrame(
	cmd *cobra.Command,
	format OutputFormat,
	frame contract.WindowManagerSnapshotFrame,
) error {
	if format == OutputJSONL {
		return writeJSONLine(cmd, frame)
	}
	if frame.Client != nil {
		_, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"snapshot\trevision=%d\tdesktops=%d\twindows=%d\tclient=%s\tpresentation_revision=%d\n",
			frame.Revision,
			len(frame.Snapshot.Desktops),
			len(frame.Snapshot.Windows),
			frame.Client.ClientID,
			frame.Client.PresentationRevision,
		)
		return err
	}
	_, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"snapshot\trevision=%d\tdesktops=%d\twindows=%d\n",
		frame.Revision,
		len(frame.Snapshot.Desktops),
		len(frame.Snapshot.Windows),
	)
	return err
}

func writeWindowManagerClientFrame(
	cmd *cobra.Command,
	format OutputFormat,
	frame contract.WindowManagerClientFrame,
) error {
	if format == OutputJSONL {
		return writeJSONLine(cmd, frame)
	}
	focusedWindowID := ""
	if frame.Client.FocusedWindowID != nil {
		focusedWindowID = string(*frame.Client.FocusedWindowID)
	}
	_, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"client\trevision=%d\tpresentation_revision=%d\tactive_desktop=%s\tfocused_window=%s\n",
		frame.Revision,
		frame.Client.PresentationRevision,
		frame.Client.ActiveDesktopID,
		focusedWindowID,
	)
	return err
}

func windowManagerCommandRequiresClient(commandID contract.WindowManagerCommandID) bool {
	switch commandID {
	case contract.WindowManagerCommandDesktopSwitch,
		contract.WindowManagerCommandWindowFocus,
		contract.WindowManagerCommandWindowZoom:
		return true
	default:
		return false
	}
}
