package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	configValidKey = "valid"
)

const (
	providerAuthStatePresent = "present"
)

const (
	automationEnabledKey             = "enabled"
	authoredContextPreviousDigestKey = "previous_digest"
	workspaceSkillSource             = "workspace"
)

const (
	authoredContextDigestKey    = "digest"
	authoredContextNewDigestKey = "new_digest"
	stateKey                    = "state"
)

const (
	automationCreatedAtKey = "created_at"
	memoryReasonKey        = "reason"
	sessionSessionKey      = "session"
)

const (
	authoredContextActionValue         = "Action"
	authoredContextActiveValue         = "Active"
	authoredContextAgentValue          = "Agent"
	authoredContextConfigDigestValue   = "Config Digest"
	authoredContextCreatedValue        = "Created"
	authoredContextDigestValue         = "Digest"
	authoredContextEnabledValue        = "Enabled"
	authoredContextEventValue          = "Event"
	authoredContextHealthValue         = "Health"
	authoredContextLastActivityValue   = "Last Activity"
	authoredContextMessageValue        = "Message"
	authoredContextNewDigestValue      = "New Digest"
	authoredContextOperationValue      = "Operation"
	authoredContextPresentValue        = "Present"
	authoredContextPreviousDigestValue = "Previous Digest"
	authoredContextReasonValue         = "Reason"
	authoredContextResultValue         = "Result"
	authoredContextSessionValue        = "Session"
	authoredContextSnapshotValue       = "Snapshot"
	authoredContextSourceValue         = "Source"
	authoredContextStateValue          = "State"
	authoredContextSummaryValue        = "Summary"
	authoredContextUpdatedValue        = "Updated"
	authoredContextValidValue          = "Valid"
	authoredContextWorkspaceValue      = "Workspace"
	authoredContextActionKey           = "action"
	authoredContextActiveKey           = "active"
	authoredContextConfigDigestKey     = "config_digest"
	authoredContextEventKey            = "event"
	authoredContextHealthKey           = "health"
	authoredContextHeartbeatKey        = "heartbeat"
	authoredContextOperationKey        = "operation"
	authoredContextSoulKey             = "soul"
)

type authoredBodyInput struct {
	file  string
	stdin bool
}

func newAgentSoulCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   authoredContextSoulKey,
		Short: "Inspect and manage agent SOUL.md files",
	}
	cmd.AddCommand(newAgentSoulInspectCommand(deps))
	cmd.AddCommand(newAgentSoulValidateCommand(deps))
	cmd.AddCommand(newAgentSoulWriteCommand(deps))
	cmd.AddCommand(newAgentSoulDeleteCommand(deps))
	cmd.AddCommand(newAgentSoulHistoryCommand(deps))
	cmd.AddCommand(newAgentSoulRollbackCommand(deps))
	return cmd
}

func newAgentSoulInspectCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "inspect <agent>",
		Short:   "Inspect one agent's resolved Soul",
		Example: "  agh agent soul inspect coder --workspace checkout-api --json",
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
			record, err := client.GetAgentSoul(cmd.Context(), args[0], query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulBundle(record))
		},
	}
	addWorkspaceFlag(cmd)
	return cmd
}

func newAgentSoulValidateCommand(deps commandDeps) *cobra.Command {
	var input authoredBodyInput
	cmd := &cobra.Command{
		Use:     "validate <agent>",
		Short:   "Validate a proposed Soul body or the current SOUL.md",
		Example: "  agh agent soul validate coder --file SOUL.md --workspace checkout-api --json",
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
			body, err := readAuthoredBody(cmd, input, false)
			if err != nil {
				return err
			}
			record, err := client.ValidateAgentSoul(cmd.Context(), args[0], AgentSoulValidateRequest{
				WorkspaceID: workspace,
				AgentName:   args[0],
				Body:        body,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulBundle(record))
		},
	}
	addWorkspaceFlag(cmd)
	addAuthoredBodyFlags(cmd, &input)
	return cmd
}

func newAgentSoulWriteCommand(deps commandDeps) *cobra.Command {
	var (
		input          authoredBodyInput
		expectedDigest string
		idempotencyKey string
	)
	cmd := &cobra.Command{
		Use:     "write <agent>",
		Short:   "Create or replace SOUL.md through managed authoring",
		Example: "  agh agent soul write coder --file SOUL.md --expected-digest sha256:old --workspace checkout-api --json",
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
			digest := optionalStringFlag(cmd, "expected-digest", expectedDigest)
			record, err := client.PutAgentSoul(cmd.Context(), args[0], AgentSoulPutRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				Body:           body,
				ExpectedDigest: digest,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulMutationBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	addAuthoredBodyFlags(cmd, &input)
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current Soul digest for CAS")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}

func newAgentSoulDeleteCommand(deps commandDeps) *cobra.Command {
	var expectedDigest string
	cmd := &cobra.Command{
		Use:     "delete <agent>",
		Short:   "Delete SOUL.md through managed authoring",
		Example: "  agh agent soul delete coder --expected-digest sha256:old --workspace checkout-api --json",
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
			digest, err := changedStringFlag(cmd, "expected-digest", expectedDigest)
			if err != nil {
				return err
			}
			record, err := client.DeleteAgentSoul(cmd.Context(), args[0], AgentSoulDeleteRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				ExpectedDigest: digest,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulMutationBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current Soul digest for CAS")
	return cmd
}

func newAgentSoulHistoryCommand(deps commandDeps) *cobra.Command {
	var (
		limit  int
		cursor string
	)
	cmd := &cobra.Command{
		Use:     "history <agent>",
		Short:   "List managed Soul authoring revisions",
		Example: "  agh agent soul history coder --limit 10 --workspace checkout-api --json",
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
			record, err := client.ListAgentSoulHistory(cmd.Context(), args[0], AgentSoulHistoryRequest{
				WorkspaceID: workspace,
				AgentName:   args[0],
				Limit:       limit,
				Cursor:      strings.TrimSpace(cursor),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulHistoryBundle(record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of revisions to return")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Revision cursor")
	return cmd
}

func newAgentSoulRollbackCommand(deps commandDeps) *cobra.Command {
	var (
		revisionID     string
		expectedDigest string
		idempotencyKey string
	)
	cmd := &cobra.Command{
		Use:     "rollback <agent>",
		Short:   "Rollback SOUL.md to a managed revision",
		Example: "  agh agent soul rollback coder --revision-id rev_123 --expected-digest sha256:old --json",
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
			revision, err := changedNonEmptyStringFlag(cmd, "revision-id", revisionID)
			if err != nil {
				return err
			}
			digest, err := changedStringFlag(cmd, "expected-digest", expectedDigest)
			if err != nil {
				return err
			}
			record, err := client.RollbackAgentSoul(cmd.Context(), args[0], AgentSoulRollbackRequest{
				WorkspaceID:    workspace,
				AgentName:      args[0],
				RevisionID:     revision,
				ExpectedDigest: digest,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentSoulMutationBundle(&record))
		},
	}
	addWorkspaceFlag(cmd)
	cmd.Flags().StringVar(&revisionID, "revision-id", "", "Managed Soul revision id to restore")
	cmd.Flags().StringVar(&expectedDigest, "expected-digest", "", "Expected current Soul digest for CAS")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	return cmd
}
