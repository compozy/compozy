package cli

import (
	"context"

	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

type coordinationMetadataFlags struct {
	taskID                string
	runID                 string
	workflowID            string
	coordinationChannelID string
	kind                  string
	kindExplicit          bool
	correlationID         string
	extRaw                string
}

func (f *coordinationMetadataFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.taskID, "task-id", "", "Task id for coordination metadata")
	cmd.Flags().StringVar(&f.runID, "run-id", "", "Run id for coordination metadata")
	cmd.Flags().StringVar(&f.workflowID, "workflow-id", "", "Workflow id for coordination metadata")
	cmd.Flags().StringVar(&f.coordinationChannelID, "channel-id", "", "Channel id; defaults to channel")
	cmd.Flags().StringVar(&f.kind, agentKernelKindKey, f.kind, "Coordination message kind")
	cmd.Flags().StringVar(&f.correlationID, "correlation-id", "", "Correlation id; defaults to run id")
	cmd.Flags().StringVar(&f.extRaw, "metadata-ext", "", "Optional JSON object for coordination metadata ext")
}

func (f coordinationMetadataFlags) metadata(
	channel string,
	defaultKind contract.CoordinationMessageKind,
	required bool,
) (contract.CoordinationMessageMetadataPayload, error) {
	metadataExt, err := parseNetworkJSONObjectMap("--metadata-ext", f.extRaw)
	if err != nil {
		return contract.CoordinationMessageMetadataPayload{}, err
	}

	kindOverride := f.kindExplicit &&
		contract.CoordinationMessageKind(strings.TrimSpace(f.kind)) != defaultKind
	if !required &&
		strings.TrimSpace(f.taskID) == "" &&
		strings.TrimSpace(f.runID) == "" &&
		strings.TrimSpace(f.workflowID) == "" &&
		strings.TrimSpace(f.coordinationChannelID) == "" &&
		strings.TrimSpace(f.correlationID) == "" &&
		len(metadataExt) == 0 &&
		!kindOverride {
		return contract.CoordinationMessageMetadataPayload{}, nil
	}

	kind := contract.CoordinationMessageKind(firstCLIValue(f.kind, string(defaultKind)))
	metadata := contract.CoordinationMessageMetadataPayload{
		TaskID:        strings.TrimSpace(f.taskID),
		RunID:         strings.TrimSpace(f.runID),
		WorkflowID:    strings.TrimSpace(f.workflowID),
		ChannelID:     firstCLIValue(f.coordinationChannelID, channel),
		MessageKind:   kind,
		CorrelationID: firstCLIValue(f.correlationID, f.runID),
		Ext:           metadataExt,
	}
	if err := metadata.Validate(); err != nil {
		return contract.CoordinationMessageMetadataPayload{}, err
	}
	return metadata, nil
}

func requireAgentCommandIdentity(
	ctx context.Context,
	deps commandDeps,
	client DaemonClient,
	originRef string,
) (agentidentity.Credentials, error) {
	if _, err := resolveAgentCallerFromEnv(ctx, deps, client, "", originRef); err != nil {
		return agentidentity.Credentials{}, err
	}
	return agentCredentialsFromEnv(deps), nil
}

func agentActionCLI(action string) string {
	return "agent." + strings.TrimSpace(action)
}

func writeAgentChannelMessages(cmd *cobra.Command, messages []AgentChannelMessageRecord) error {
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode == OutputJSONL {
		for _, message := range messages {
			if err := writeJSONLine(cmd, message); err != nil {
				return err
			}
		}
		return nil
	}
	return writeCommandOutput(cmd, agentChannelMessagesBundle(messages))
}
