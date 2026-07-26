package cli

import (
	"errors"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

func newAgentHeartbeatCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   authoredContextHeartbeatKey,
		Short: "Inspect and manage agent HEARTBEAT.md files",
	}
	cmd.AddCommand(newAgentHeartbeatInspectCommand(deps))
	cmd.AddCommand(newAgentHeartbeatValidateCommand(deps))
	cmd.AddCommand(newAgentHeartbeatWriteCommand(deps))
	cmd.AddCommand(newAgentHeartbeatDeleteCommand(deps))
	cmd.AddCommand(newAgentHeartbeatHistoryCommand(deps))
	cmd.AddCommand(newAgentHeartbeatRollbackCommand(deps))
	cmd.AddCommand(newAgentHeartbeatStatusCommand(deps))
	cmd.AddCommand(newAgentHeartbeatWakeCommand(deps))
	return cmd
}

func newAgentHeartbeatInspectCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "inspect <agent>",
		Short:   "Inspect one agent's resolved Heartbeat policy",
		Example: "  agh agent heartbeat inspect coder --workspace checkout-api --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			query, err := agentQueryFromCommand(cmd)
			if err != nil {
				return err
			}
			record, err := client.GetAgentHeartbeat(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	return cmd
}

func newAgentHeartbeatValidateCommand(deps commandDeps) *cobra.Command {
	var input authoredBodyInput
	cmd := &cobra.Command{
		Use:     "validate <agent>",
		Short:   "Validate a proposed Heartbeat policy body",
		Example: "  agh agent heartbeat validate coder --file HEARTBEAT.md --workspace checkout-api --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			body, err := readAuthoredBody(cmd, input, true)
			if err != nil {
				return err
			}
			record, err := client.ValidateAgentHeartbeat(cmd.Context(), args[0], AgentHeartbeatValidateRequest{
				WorkspaceID: workspace,
				AgentName:   args[0],
				Body:        body,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	addAuthoredBodyFlags(cmd, &input)
	return cmd
}

func newAgentHeartbeatWriteCommand(deps commandDeps) *cobra.Command {
	var (
		input          authoredBodyInput
		expectedDigest string
		ifMatchDigest  string
		idempotencyKey string
	)
	cmd := &cobra.Command{
		Use:     "write <agent>",
		Short:   "Create or replace HEARTBEAT.md through managed authoring",
		Example: "  agh agent heartbeat write coder --file HEARTBEAT.md --expected-digest sha256:old --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			body, err := readAuthoredBody(cmd, input, true)
			if err != nil {
				return err
			}
			digest, err := optionalExpectedDigestFlag(cmd, expectedDigest, ifMatchDigest)
			if err != nil {
				return err
			}
			record, err := client.PutAgentHeartbeat(cmd.Context(), args[0], AgentHeartbeatPutRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				Body:           body,
				ExpectedDigest: digest,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatMutationBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	addAuthoredBodyFlags(cmd, &input)
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current Heartbeat digest for CAS")
	cmd.Flags().StringVar(&ifMatchDigest, "if-match", "", "Alias for --expected-digest")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}

func newAgentHeartbeatDeleteCommand(deps commandDeps) *cobra.Command {
	var expectedDigest string
	var ifMatchDigest string
	cmd := &cobra.Command{
		Use:     "delete <agent>",
		Short:   "Delete HEARTBEAT.md through managed authoring",
		Example: "  agh agent heartbeat delete coder --expected-digest sha256:old --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			digest, err := changedExpectedDigestFlag(cmd, expectedDigest, ifMatchDigest)
			if err != nil {
				return err
			}
			record, err := client.DeleteAgentHeartbeat(cmd.Context(), args[0], AgentHeartbeatDeleteRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				ExpectedDigest: digest,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatMutationBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current Heartbeat digest for CAS")
	cmd.Flags().StringVar(&ifMatchDigest, "if-match", "", "Alias for --expected-digest")
	return cmd
}

func newAgentHeartbeatHistoryCommand(deps commandDeps) *cobra.Command {
	var (
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:     "history <agent>",
		Short:   "List managed Heartbeat authoring revisions",
		Example: "  agh agent heartbeat history coder --limit 10 --workspace checkout-api --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			record, err := client.ListAgentHeartbeatHistory(cmd.Context(), args[0], AgentHeartbeatHistoryRequest{
				WorkspaceID: workspace,
				AgentName:   args[0],
				Limit:       limit,
				Cursor:      strings.TrimSpace(cursor),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatHistoryBundle(record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of revisions to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Revision cursor")
	return cmd
}

func newAgentHeartbeatRollbackCommand(deps commandDeps) *cobra.Command {
	var (
		revisionID     string
		targetDigest   string
		expectedDigest string
		ifMatchDigest  string
		idempotencyKey string
	)
	cmd := &cobra.Command{
		Use:     "rollback <agent>",
		Short:   "Rollback HEARTBEAT.md to a managed revision or snapshot digest",
		Example: "  agh agent heartbeat rollback coder --revision-id rev_123 --expected-digest sha256:old --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			revisionChanged := cmd.Flags().Changed("revision-id")
			targetChanged := cmd.Flags().Changed("target-digest")
			if revisionChanged == targetChanged {
				return errors.New("cli: exactly one of --revision-id or --target-digest is required")
			}
			revision := strings.TrimSpace(revisionID)
			target := strings.TrimSpace(targetDigest)
			if revisionChanged {
				if revision == "" {
					return errors.New("cli: --revision-id cannot be empty")
				}
			}
			if targetChanged {
				if target == "" {
					return errors.New("cli: --target-digest cannot be empty")
				}
			}
			digest, err := changedExpectedDigestFlag(cmd, expectedDigest, ifMatchDigest)
			if err != nil {
				return err
			}
			record, err := client.RollbackAgentHeartbeat(cmd.Context(), args[0], AgentHeartbeatRollbackRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				RevisionID:     revision,
				TargetDigest:   target,
				ExpectedDigest: digest,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatMutationBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&revisionID, "revision-id", "", "Managed Heartbeat revision id to restore")
	cmd.Flags().StringVar(&targetDigest, "target-digest", "", "Heartbeat snapshot digest to restore")
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current Heartbeat digest for CAS")
	cmd.Flags().StringVar(&ifMatchDigest, "if-match", "", "Alias for --expected-digest")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}

func newAgentHeartbeatStatusCommand(deps commandDeps) *cobra.Command {
	var (
		sessionID     string
		includeHealth bool
		includeEvents bool
	)
	cmd := &cobra.Command{
		Use:     "status <agent>",
		Short:   "Read Heartbeat policy status and wake eligibility",
		Example: "  agh agent heartbeat status coder --session sess_123 --workspace checkout-api --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			session := strings.TrimSpace(sessionID)
			record, err := client.GetAgentHeartbeatStatus(cmd.Context(), args[0], AgentHeartbeatStatusRequest{
				WorkspaceID:             workspace,
				AgentName:               args[0],
				SessionID:               session,
				IncludeSessionHealth:    includeHealth || session != "",
				IncludeRecentWakeEvents: includeEvents,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatStatusBundle(record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&sessionID, sessionSessionKey, "", "Session id for wake state and health")
	cmd.Flags().BoolVar(
		&includeHealth,
		"include-session-health",
		false,
		"Include session health when a session id is supplied",
	)
	cmd.Flags().BoolVar(&includeEvents, "include-wake-events", false, "Include recent wake audit rows")
	return cmd
}

func newAgentHeartbeatWakeCommand(deps commandDeps) *cobra.Command {
	var (
		sessionID      string
		source         string
		dryRun         bool
		idempotencyKey string
	)
	cmd := &cobra.Command{
		Use:     "wake <agent>",
		Short:   "Request one manual advisory Heartbeat wake",
		Example: "  agh agent heartbeat wake coder --session sess_123 --dry-run --json",
		Args:    exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			workspace, err := commandWorkspaceFlag(cmd)
			if err != nil {
				return err
			}
			session, err := changedNonEmptyStringFlag(cmd, sessionSessionKey, sessionID)
			if err != nil {
				return err
			}
			wakeSource := contract.HeartbeatWakeSource(strings.TrimSpace(source))
			if wakeSource == "" {
				wakeSource = contract.HeartbeatWakeSourceManual
			}
			record, err := client.WakeAgentHeartbeat(cmd.Context(), args[0], AgentHeartbeatWakeRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				SessionID:      session,
				Source:         wakeSource,
				DryRun:         dryRun,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentHeartbeatWakeBundle(record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&sessionID, sessionSessionKey, "", "Session id to wake")
	cmd.Flags().StringVar(&source, "source", string(contract.HeartbeatWakeSourceManual), "Wake source")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Evaluate wake gates without sending a prompt")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}
