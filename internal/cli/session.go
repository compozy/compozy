package cli

import (
	"errors"

	"strings"

	"github.com/compozy/compozy/internal/session"
	"github.com/spf13/cobra"
)

const (
	sessionTurnIDKey = "turn_id"
)

const (
	sessionSequenceKey = "sequence"
)

const (
	sessionAgentValue     = "Agent"
	sessionBackendValue   = "Backend"
	sessionChannelValue   = "Channel"
	sessionCreatedValue   = "Created"
	sessionBadgeValue     = "Badge"
	sessionNameValue      = "Name"
	sessionProfileValue   = "Profile"
	sessionProviderValue  = "Provider"
	sessionSessionValue   = "Session"
	sessionStateValue     = "State"
	sessionStatusValue    = "Status"
	sessionTimestampValue = "Timestamp"
	sessionUpdatedValue   = "Updated"
	sessionWorkspaceValue = "Workspace"
	sessionAgentNameKey   = "agent_name"
	sessionBadgeKey       = "badge"
	sessionChannelKey     = "channel"
	sessionCreatedAtKey   = "created_at"
	sessionHistoryIDValue = "history <id>"
	sessionListKey        = "list"
	sessionMessagesKey    = "messages"
	sessionNameKey        = "name"
	sessionNewKey         = "new"
	sessionProviderKey    = "provider"
	sessionRequestIDKey   = "request_id"
	sessionSessionIDKey   = "session_id"
	sessionStateKey       = "state"
	sessionStatusKey      = "status"
	sessionUpdatedAtKey   = "updated_at"
)

func newSessionStopCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id>",
		Short: "Stop a session",
		Example: `  # Stop a running session
  compozy session stop sess_1234`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if err := client.StopSession(cmd.Context(), args[0]); err != nil {
				return err
			}

			info, err := client.GetSession(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&info, deps.now))
		},
	}
}

func newSessionRemoveCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a session and its persisted history",
		Example: `  # Remove a stopped session
  compozy session remove sess_1234`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			info, err := client.GetSession(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := client.DeleteSession(cmd.Context(), args[0]); err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&info, deps.now))
		},
	}
}

func newSessionStatusCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "status <id>",
		Short: "Show session status",
		Example: `  # Show current state for one session
  compozy session status sess_1234

  # Read status as JSON for scripts
  compozy session status sess_1234 -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			info, err := client.GetSessionStatus(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionStatusBundle(info))
		},
	}
}

func newSessionResumeCommand(deps commandDeps) *cobra.Command {
	var (
		latest          bool
		workspaceFilter string
	)
	cmd := &cobra.Command{
		Use:   "resume [id]",
		Short: "Attach to a resumable session",
		Example: `  # Attach to a resumable session by ID
  compozy session resume sess_1234

  # Attach to the latest eligible session in a workspace
  compozy session resume --latest --workspace checkout-api`,
		Args: sessionResumeArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			if latest && len(args) > 0 {
				return errors.New("cli: --latest cannot be combined with a session id")
			}
			if !latest && len(args) == 0 {
				return errors.New("session_resume_ambiguous: pass a session id or --latest")
			}
			sessionID := ""
			if latest {
				workspaceID, err := resolveOptionalWorkspaceOverride(
					cmd.Context(),
					cmd,
					deps,
					client,
					workspaceFilter,
				)
				if err != nil {
					return err
				}
				page, err := client.ListSessions(cmd.Context(), SessionListQuery{
					Workspace: workspaceID,
					Resumable: true,
					Sort:      session.ListSortLastActivity,
					Limit:     1,
				})
				if err != nil {
					return err
				}
				if len(page.Sessions) == 0 {
					return writeCommandOutput(cmd, sessionResumeEmptyBundle())
				}
				sessionID = page.Sessions[0].ID
			} else {
				sessionID = args[0]
			}
			info, err := client.ResumeSession(cmd.Context(), sessionID)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionBundle(&info, deps.now))
		},
	}
	cmd.Flags().BoolVar(&latest, "latest", false, "Attach to the latest eligible session")
	cmd.Flags().
		StringVar(&workspaceFilter, workspaceSkillSource, "", "Override --latest workspace filter (ID, name, or path)")
	return cmd
}

func sessionResumeArgs(_ *cobra.Command, args []string) error {
	if len(args) > 1 {
		return errors.New("cli: expected at most one session id")
	}
	if len(args) == 1 && strings.TrimSpace(args[0]) == "" {
		return errors.New("cli: session id is required")
	}
	return nil
}

func newSessionRecapCommand(deps commandDeps) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "recap <id>",
		Short: "Show deterministic session recap",
		Example: `  # Show a bounded recap for one session
  compozy session recap sess_1234 --limit 20`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			recap, err := client.SessionRecap(cmd.Context(), args[0], limit)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionRecapBundle(&recap))
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum recent messages to include")
	return cmd
}

func newSessionRepairCommand(deps commandDeps) *cobra.Command {
	var (
		dryRun bool
		force  bool
	)

	cmd := &cobra.Command{
		Use:   "repair <id>",
		Short: "Inspect and repair an interrupted session transcript",
		Example: `  # Report the repair actions without writing new events
  compozy session repair sess_1234 --dry-run

  # Force repair for a stopped session whose stop reason is not crash or error
  compozy session repair sess_1234 --force`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}

			result, err := client.RepairSession(cmd.Context(), args[0], SessionRepairQuery{
				DryRun: dryRun,
				Force:  force,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionRepairBundle(result))
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report planned repairs without persisting events")
	cmd.Flags().BoolVar(&force, "force", false, "Allow repair for stopped non-crash sessions")
	return cmd
}

func newSessionApproveCommand(deps commandDeps) *cobra.Command {
	var request SessionApprovalRequest
	cmd := &cobra.Command{
		Use:   "approve <id>",
		Short: "Approve or reject a pending session permission request",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			request.RequestID = strings.TrimSpace(request.RequestID)
			request.TurnID = strings.TrimSpace(request.TurnID)
			request.Decision = strings.TrimSpace(request.Decision)
			if request.RequestID == "" && request.TurnID == "" {
				return errors.New("cli: --request-id or --turn-id is required")
			}
			if request.Decision == "" {
				return errors.New("cli: --decision is required")
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.ApproveSession(cmd.Context(), args[0], request)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, sessionApprovalBundle(args[0], result))
		},
	}
	cmd.Flags().StringVar(&request.RequestID, "request-id", "", "Pending permission request id")
	cmd.Flags().StringVar(&request.TurnID, "turn-id", "", "Pending permission turn id")
	cmd.Flags().
		StringVar(&request.Decision, "decision", "", "Decision: allow-once, allow-always, reject-once, or reject-always")
	mustMarkFlagRequired(cmd, "decision")
	return cmd
}
