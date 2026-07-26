package cli

import (
	"errors"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

const (
	automationSessionIDKey = "session_id"
)

const (
	bridgeKindValue     = "Kind"
	networkTimestampKey = "timestamp"
)

const (
	agentKernelModelKey = "model"
)

const (
	agentKernelAgentValue   = "Agent"
	agentKernelChannelValue = "Channel"
	agentKernelFromValue    = "From"
	agentKernelRootValue    = "Root"
	agentKernelSessionValue = "Session"
	agentKernelAgentNameKey = "agent_name"
	agentKernelChannelKey   = "channel"
	agentKernelContextKey   = "context"
	agentKernelKindKey      = "kind"
	agentKernelListKey      = "list"
	agentKernelReplyKey     = "reply"
	agentKernelRunIDKey     = "run_id"
)

func newMeCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Inspect the current AGH-managed agent session",
		Example: `  # Show the current managed session identity
  agh me

  # Print machine-readable caller state
  agh me -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("me"))
			if err != nil {
				return err
			}
			record, err := client.AgentMe(cmd.Context(), credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentMeBundle(record))
		},
	}
	cmd.AddCommand(newMeContextCommand(deps))
	return cmd
}

func newMeContextCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   agentKernelContextKey,
		Short: "Inspect the bounded situation context for the current agent session",
		Example: `  # Show the bounded situation context injected for this session
  agh me context

  # Read the context payload as JSON
  agh me context -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("me.context"))
			if err != nil {
				return err
			}
			record, err := client.AgentContext(cmd.Context(), credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentContextBundle(&record))
		},
	}
}

func newChannelCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ch",
		Aliases: []string{agentKernelChannelKey},
		Short:   "Use agent-facing coordination channels",
		Example: `  # List channels visible to this session
  agh ch list

  # Wait for task-run coordination messages
  agh ch recv coord-run-123 --wait -o jsonl`,
	}
	cmd.AddCommand(newChannelListCommand(deps))
	cmd.AddCommand(newChannelRecvCommand(deps))
	cmd.AddCommand(newChannelSendCommand(deps))
	cmd.AddCommand(newChannelReplyCommand(deps))
	return cmd
}

func newChannelListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   agentKernelListKey,
		Short: "List coordination channels visible to the current agent session",
		Example: `  # List coordination channels visible to this session
  agh ch list

  # Print channel discovery metadata as JSON
  agh ch list -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("ch.list"))
			if err != nil {
				return err
			}
			channels, err := client.AgentChannels(cmd.Context(), credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentChannelsBundle(channels))
		},
	}
}

func newChannelRecvCommand(deps commandDeps) *cobra.Command {
	var wait bool
	var limit int

	cmd := &cobra.Command{
		Use:   "recv <channel>",
		Short: "Receive queued coordination messages for a channel",
		Args:  exactOneNonBlankArg(),
		Example: `  # Receive currently queued messages
  agh ch recv coord-run-123

  # Wait for messages and stream each record as JSONL
  agh ch recv coord-run-123 --wait --limit 10 -o jsonl`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("ch.recv"))
			if err != nil {
				return err
			}
			messages, err := client.AgentChannelRecv(cmd.Context(), strings.TrimSpace(args[0]), AgentChannelRecvQuery{
				Wait:  wait,
				Limit: limit,
			}, credentials)
			if err != nil {
				return err
			}
			return writeAgentChannelMessages(cmd, messages)
		},
	}
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait until at least one matching message is available")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum messages to return")
	return cmd
}

func newChannelSendCommand(deps commandDeps) *cobra.Command {
	var bodyRaw string
	var idempotencyKey string
	flags := coordinationMetadataFlags{kind: string(contract.CoordinationMessageStatus)}

	cmd := &cobra.Command{
		Use:   "send <channel>",
		Short: "Send one task-run coordination message",
		Args:  exactOneNonBlankArg(),
		Example: `  # Send a non-authoritative status message for a run
  agh ch send coord-run-123 \
    --task-id task-123 \
    --run-id run-123 \
    --kind status \
    --correlation-id run-123 \
    --body '{"status":"investigating"}'

  # Report a blocker in the same task-bound channel
  agh ch send coord-run-123 \
    --task-id task-123 \
    --run-id run-123 \
    --kind blocker \
    --correlation-id run-123 \
		--body '{"blocked_by":"missing credentials"}'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			channel := strings.TrimSpace(args[0])
			flags.kindExplicit = cmd.Flags().Changed(agentKernelKindKey)
			body, err := parseNetworkJSONValue("--body", bodyRaw)
			if err != nil {
				return err
			}
			metadata, err := flags.metadata(channel, contract.CoordinationMessageStatus, true)
			if err != nil {
				return err
			}
			if metadata.MessageKind == contract.CoordinationMessageReply {
				return errors.New("cli: use `agh ch reply --to-message` for reply messages")
			}
			request := AgentChannelSendRequest{
				Body:           body,
				Metadata:       metadata,
				IdempotencyKey: strings.TrimSpace(idempotencyKey),
			}
			if err := contract.ValidateNoRawClaimTokenField(request); err != nil {
				return err
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("ch.send"))
			if err != nil {
				return err
			}
			message, err := client.AgentChannelSend(cmd.Context(), channel, request, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentChannelMessageBundle(message))
		},
	}
	cmd.Flags().StringVar(&bodyRaw, "body", "", "Raw JSON body for the channel message")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key/message id")
	flags.bind(cmd)
	mustMarkFlagRequired(cmd, "body")
	return cmd
}

func newChannelReplyCommand(deps commandDeps) *cobra.Command {
	var bodyRaw string
	var toMessage string
	var idempotencyKey string
	flags := coordinationMetadataFlags{kind: string(contract.CoordinationMessageReply)}

	cmd := &cobra.Command{
		Use:   agentKernelReplyKey,
		Short: "Reply to a received coordination message",
		Example: `  # Reply to one received coordination message
  agh ch reply --to-message msg-123 --body '{"answer":"ready for review"}'

  # Add explicit correlation metadata when replying from outside an inherited delivery
  agh ch reply \
    --to-message msg-123 \
    --task-id task-123 \
    --run-id run-123 \
    --channel-id coord-run-123 \
    --correlation-id run-123 \
    --body '{"answer":"ready for review"}'`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags.kindExplicit = cmd.Flags().Changed(agentKernelKindKey)
			if flags.kindExplicit &&
				contract.CoordinationMessageKind(strings.TrimSpace(flags.kind)) != contract.CoordinationMessageReply {
				return errors.New("cli: --kind must be reply for `agh ch reply`")
			}
			body, err := parseNetworkJSONValue("--body", bodyRaw)
			if err != nil {
				return err
			}
			metadata, err := flags.metadata("", contract.CoordinationMessageReply, false)
			if err != nil {
				return err
			}
			if !zeroCLIAgentCoordinationMetadata(metadata) &&
				metadata.MessageKind != contract.CoordinationMessageReply {
				return errors.New("cli: --kind must be reply for `agh ch reply`")
			}
			request := AgentChannelReplyRequest{
				ReplyToMessageID: strings.TrimSpace(toMessage),
				Body:             body,
				Metadata:         metadata,
				IdempotencyKey:   strings.TrimSpace(idempotencyKey),
			}
			if err := contract.ValidateNoRawClaimTokenField(request); err != nil {
				return err
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("ch.reply"))
			if err != nil {
				return err
			}
			message, err := client.AgentChannelReply(cmd.Context(), request, credentials)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, agentChannelMessageBundle(message))
		},
	}
	cmd.Flags().StringVar(&toMessage, "to-message", "", "Message id to reply to")
	cmd.Flags().StringVar(&bodyRaw, "body", "", "Raw JSON body for the reply message")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Optional idempotency key/message id")
	flags.bind(cmd)
	mustMarkFlagRequired(cmd, "to-message")
	mustMarkFlagRequired(cmd, "body")
	return cmd
}
