package cli

import (
	"strings"

	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/spf13/cobra"
)

const (
	toolOperatorToolIDKey = "tool_id"
)

const (
	toolOperatorReasonsKey = "reasons"
)

const (
	toolOperatorBackendValue     = "Backend"
	toolOperatorExpiresValue     = "Expires"
	toolOperatorSourceValue      = "Source"
	toolOperatorStatusValue      = "Status"
	toolOperatorTitleValue       = "Title"
	toolOperatorToolIDValue      = "Tool ID"
	toolOperatorAvailableKey     = "available"
	toolOperatorBackendKey       = "backend"
	toolOperatorCLIKey           = "cli"
	toolOperatorDisabledKey      = "disabled"
	toolOperatorExpiresAtKey     = "expires_at"
	toolOperatorListKey          = "list"
	toolOperatorSearchQueryValue = "search <query>"
)

type toolScopeFlags struct {
	workspaceID string
	sessionID   string
	agentName   string
}

type toolInvokeFlags struct {
	scope                toolScopeFlags
	input                string
	inputFile            string
	toolCallID           string
	turnID               string
	correlationID        string
	approvalToken        string
	sensitiveInputFields []string
}

type toolApprovalFlags struct {
	scope       toolScopeFlags
	input       string
	inputFile   string
	inputDigest string
}

func newToolListCommand(deps commandDeps) *cobra.Command {
	var scope toolScopeFlags
	cmd := &cobra.Command{
		Use:   toolOperatorListKey,
		Short: "List operator-visible registry tools",
		Example: `  # List all operator-visible tools as JSON
  agh tool list -o json

  # Inspect the session-scoped operator view for one agent
  agh tool list --workspace ws-1 --session sess-1 --agent coder -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				response, err := client.ListTools(cmd.Context(), scope.query())
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolListBundle(response))
			})
		},
	}
	scope.bind(cmd)
	return cmd
}

func newToolSearchCommand(deps commandDeps) *cobra.Command {
	var scope toolScopeFlags
	var limit int
	cmd := &cobra.Command{
		Use:   toolOperatorSearchQueryValue,
		Short: "Search operator-visible registry tools",
		Example: `  # Search tools by descriptor text
  agh tool search skill -o json

  # Limit search results for automation
  agh tool search task --limit 5 -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.TrimSpace(args[0])
			if query == "" {
				return writeToolCommandError(cmd, toolValidationCommandError(
					toolspkg.ToolID(""),
					"tool search query is required",
					toolspkg.NewValidationError("query", toolspkg.ReasonSchemaInvalid, "query is required"),
				))
			}
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				request := ToolSearchRequest{
					Query:       query,
					Limit:       limit,
					WorkspaceID: strings.TrimSpace(scope.workspaceID),
					SessionID:   strings.TrimSpace(scope.sessionID),
					AgentName:   strings.TrimSpace(scope.agentName),
				}
				response, err := client.SearchTools(cmd.Context(), request)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolListBundle(response))
			})
		},
	}
	scope.bind(cmd)
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of tools to return")
	return cmd
}

func newToolInfoCommand(deps commandDeps) *cobra.Command {
	var scope toolScopeFlags
	cmd := &cobra.Command{
		Use:   "info <tool_id>",
		Short: "Show one registry tool descriptor and diagnostics",
		Example: `  # Show a tool descriptor and availability diagnostics
  agh tool info agh__skill_view -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseToolIDArg(args[0])
			if err != nil {
				return writeToolCommandError(cmd, err)
			}
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				response, err := client.GetTool(cmd.Context(), id.String(), scope.query())
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolInfoBundle(&response))
			})
		},
	}
	scope.bind(cmd)
	return cmd
}

func newToolApproveCommand(deps commandDeps) *cobra.Command {
	var flags toolApprovalFlags
	cmd := &cobra.Command{
		Use:   "approve <tool_id>",
		Short: "Mint a one-shot approval token for one tool invocation",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseToolIDArg(args[0])
			if err != nil {
				return writeToolCommandError(cmd, err)
			}
			input, err := resolveToolApprovalInput(cmd, flags)
			if err != nil {
				return writeToolCommandError(cmd, toolValidationCommandError(id, "tool approval input is invalid", err))
			}
			if strings.TrimSpace(flags.scope.sessionID) == "" {
				return writeToolCommandError(cmd, toolValidationCommandError(
					id,
					"tool approval scope is invalid",
					toolspkg.NewValidationError(
						"session_id",
						toolspkg.ReasonSchemaInvalid,
						"session id is required",
					),
				))
			}
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				request := ToolApprovalRequest{
					SessionID:   strings.TrimSpace(flags.scope.sessionID),
					WorkspaceID: strings.TrimSpace(flags.scope.workspaceID),
					AgentName:   strings.TrimSpace(flags.scope.agentName),
					Input:       input,
					InputDigest: strings.TrimSpace(flags.inputDigest),
				}
				approval, err := client.CreateToolApproval(cmd.Context(), id.String(), request)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolApprovalBundle(approval))
			})
		},
	}
	flags.scope.bind(cmd)
	cmd.Flags().StringVar(&flags.input, "input", "", "Inline JSON input")
	cmd.Flags().StringVar(&flags.inputFile, "input-file", "", "Path to JSON input file, or '-' for stdin")
	cmd.Flags().StringVar(&flags.inputDigest, "input-digest", "", "Precomputed input digest")
	mustMarkFlagRequired(cmd, "session")
	return cmd
}

func newToolInvokeCommand(deps commandDeps) *cobra.Command {
	var flags toolInvokeFlags
	cmd := &cobra.Command{
		Use:   "invoke <tool_id>",
		Short: "Invoke one registry tool through daemon policy",
		Example: `  # Invoke a tool with inline JSON input
  agh tool invoke agh__tool_info --input '{"tool_id":"agh__skill_view"}' -o json

  # Invoke a tool with JSON read from a file
  agh tool invoke agh__tool_info --input-file ./input.json -o json

  # Invoke a tool with JSON read from stdin
  echo '{"tool_id":"agh__skill_view"}' | agh tool invoke agh__tool_info -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseToolIDArg(args[0])
			if err != nil {
				return writeToolCommandError(cmd, err)
			}
			input, err := resolveToolInvokeInput(cmd, flags)
			if err != nil {
				return writeToolCommandError(cmd, toolValidationCommandError(id, "tool input is invalid", err))
			}
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				request := ToolInvokeRequest{
					SessionID:            strings.TrimSpace(flags.scope.sessionID),
					WorkspaceID:          strings.TrimSpace(flags.scope.workspaceID),
					AgentName:            strings.TrimSpace(flags.scope.agentName),
					ToolCallID:           strings.TrimSpace(flags.toolCallID),
					TurnID:               strings.TrimSpace(flags.turnID),
					CorrelationID:        strings.TrimSpace(flags.correlationID),
					ApprovalToken:        strings.TrimSpace(flags.approvalToken),
					Input:                input,
					SensitiveInputFields: trimNonEmptyStrings(flags.sensitiveInputFields),
				}
				response, err := client.InvokeTool(cmd.Context(), id.String(), request)
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolInvokeBundle(sanitizeToolInvokeResponse(response)))
			})
		},
	}
	flags.scope.bind(cmd)
	cmd.Flags().StringVar(&flags.input, "input", "", "Inline JSON input")
	cmd.Flags().StringVar(&flags.inputFile, "input-file", "", "Path to JSON input file, or '-' for stdin")
	cmd.Flags().StringVar(&flags.toolCallID, "tool-call-id", "", "Optional caller tool-call id")
	cmd.Flags().StringVar(&flags.turnID, "turn-id", "", "Optional caller turn id")
	cmd.Flags().StringVar(&flags.correlationID, "correlation-id", "", "Optional correlation id")
	cmd.Flags().
		StringVar(&flags.approvalToken, "approval-token", "", "Single-use approval token for approval-gated tools")
	cmd.Flags().
		StringArrayVar(&flags.sensitiveInputFields, "sensitive-input-field", nil, "Input field path to redact in events")
	return cmd
}

func newToolsetsCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "toolsets",
		Short: "Inspect registry toolsets",
	}
	cmd.AddCommand(newToolsetsListCommand(deps))
	cmd.AddCommand(newToolsetsInfoCommand(deps))
	return cmd
}

func newToolsetsListCommand(deps commandDeps) *cobra.Command {
	var scope toolScopeFlags
	cmd := &cobra.Command{
		Use:   toolOperatorListKey,
		Short: "List registry toolsets",
		Example: `  # List known toolsets and expansion diagnostics
  agh toolsets list -o json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				response, err := client.ListToolsets(cmd.Context(), scope.query())
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolsetListBundle(response))
			})
		},
	}
	scope.bind(cmd)
	return cmd
}

func newToolsetsInfoCommand(deps commandDeps) *cobra.Command {
	var scope toolScopeFlags
	cmd := &cobra.Command{
		Use:   "info <toolset_id>",
		Short: "Show one registry toolset expansion",
		Example: `  # Show one toolset and expanded tool ids
  agh toolsets info agh__catalog -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseToolsetIDArg(args[0])
			if err != nil {
				return writeToolCommandError(cmd, err)
			}
			return runToolCommand(cmd, deps, func(client DaemonClient) error {
				response, err := client.GetToolset(cmd.Context(), id.String(), scope.query())
				if err != nil {
					return err
				}
				return writeCommandOutput(cmd, toolsetInfoBundle(response))
			})
		},
	}
	scope.bind(cmd)
	return cmd
}

func (f *toolScopeFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.workspaceID, "workspace", "", "Workspace id for scoped diagnostics")
	cmd.Flags().StringVar(&f.sessionID, "session", "", "Session id for scoped diagnostics")
	cmd.Flags().StringVar(&f.agentName, "agent", "", "Agent name for scoped diagnostics")
}

func (f toolScopeFlags) query() ToolQuery {
	return ToolQuery{
		WorkspaceID: strings.TrimSpace(f.workspaceID),
		SessionID:   strings.TrimSpace(f.sessionID),
		AgentName:   strings.TrimSpace(f.agentName),
	}
}
