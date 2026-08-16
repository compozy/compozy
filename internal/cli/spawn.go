package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

const (
	spawnAgentValue     = "Agent"
	spawnParentValue    = "Parent"
	spawnProviderValue  = "Provider"
	spawnRootValue      = "Root"
	spawnSessionValue   = "Session"
	spawnWakeValue      = "Wake"
	spawnWorkspaceValue = "Workspace"
)

type spawnCommandFlags struct {
	agentName        string
	provider         string
	model            string
	name             string
	workspace        string
	promptOverlay    string
	spawnRole        string
	ttlSeconds       int64
	autoStopOnParent bool
	noNotifyCreator  bool
	tools            []string
	skills           []string
	mcpServers       []string
	workspacePaths   []string
	networkChannels  []string
	sandboxProfiles  []string
	idempotencyKey   string
}

func newSpawnCommand(deps commandDeps) *cobra.Command {
	flags := &spawnCommandFlags{}
	cmd := &cobra.Command{
		Use:   "spawn",
		Short: "Spawn a bounded child agent session",
		Args:  cobra.NoArgs,
		Example: `  # Spawn a worker child with a required TTL
  compozy spawn --agent reviewer --ttl-seconds 1800

  # Spawn with a role, prompt overlay, and narrowed permission atoms
  compozy spawn \
    --agent reviewer \
    --role reviewer \
    --ttl-seconds 1800 \
    --prompt-overlay "Review only the implementation diff." \
    --tool read \
    --skill code-review \
    --channel coord-run-123`,
		RunE: runSpawnCommand(deps, flags),
	}
	registerSpawnFlags(cmd, flags)
	return cmd
}

func registerSpawnFlags(cmd *cobra.Command, flags *spawnCommandFlags) {
	cmd.Flags().StringVar(&flags.agentName, "agent", "", "Child agent name")
	cmd.Flags().StringVar(&flags.provider, "provider", "", "Optional provider override")
	cmd.Flags().StringVar(&flags.model, "model", "", "Optional model override")
	cmd.Flags().StringVar(&flags.name, "name", "", "Optional child session display name")
	cmd.Flags().StringVar(&flags.workspace, "workspace", "", "Override child workspace (ID, name, or path)")
	cmd.Flags().StringVar(&flags.promptOverlay, "prompt-overlay", "", "Prompt overlay for the child session")
	cmd.Flags().StringVar(&flags.spawnRole, "role", "worker", "Child spawn role")
	cmd.Flags().Int64Var(&flags.ttlSeconds, "ttl-seconds", 0, "Mandatory child TTL in seconds")
	cmd.Flags().BoolVar(&flags.autoStopOnParent, "auto-stop-on-parent", true, "Stop the child when the parent stops")
	cmd.Flags().BoolVar(&flags.noNotifyCreator, "no-notify-creator", false, "Do not wake this session when the child needs attention or stops")
	cmd.Flags().StringArrayVar(&flags.tools, "tool", nil, "Allowed tool atom (repeatable)")
	cmd.Flags().StringArrayVar(&flags.skills, "skill", nil, "Allowed skill atom (repeatable)")
	cmd.Flags().StringArrayVar(&flags.mcpServers, "mcp-server", nil, "Allowed MCP server id (repeatable)")
	cmd.Flags().
		StringArrayVar(&flags.workspacePaths, "workspace-path", nil, "Allowed workspace path grant (repeatable)")
	cmd.Flags().StringArrayVar(&flags.networkChannels, "channel", nil, "Allowed network channel grant (repeatable)")
	cmd.Flags().StringArrayVar(
		&flags.sandboxProfiles,
		"sandbox-profile",
		nil,
		"Allowed sandbox profile grant (repeatable)",
	)
	cmd.Flags().StringVar(&flags.idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	mustMarkFlagRequired(cmd, "agent")
	mustMarkFlagRequired(cmd, "ttl-seconds")
}

func runSpawnCommand(deps commandDeps, flags *spawnCommandFlags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		request, err := flags.request(cmd.Flags().Changed("no-notify-creator"))
		if err != nil {
			return err
		}
		client, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		credentials, err := requireAgentCommandIdentity(cmd.Context(), deps, client, agentActionCLI("spawn"))
		if err != nil {
			return err
		}
		record, err := client.AgentSpawn(cmd.Context(), request, credentials)
		if err != nil {
			return err
		}
		return writeCommandOutput(cmd, agentSpawnBundle(&record))
	}
}

func (flags *spawnCommandFlags) request(notifyFlagChanged bool) (AgentSpawnRequest, error) {
	if strings.TrimSpace(flags.agentName) == "" {
		return AgentSpawnRequest{}, fmt.Errorf("cli: --agent is required")
	}
	if flags.ttlSeconds <= 0 {
		return AgentSpawnRequest{}, fmt.Errorf("cli: --ttl-seconds must be positive")
	}
	if strings.EqualFold(strings.TrimSpace(flags.spawnRole), "coordinator") {
		return AgentSpawnRequest{}, fmt.Errorf("cli: coordinator spawn role is not supported")
	}
	request := AgentSpawnRequest{
		AgentName:        strings.TrimSpace(flags.agentName),
		Provider:         strings.TrimSpace(flags.provider),
		Model:            strings.TrimSpace(flags.model),
		Name:             strings.TrimSpace(flags.name),
		Workspace:        strings.TrimSpace(flags.workspace),
		PromptOverlay:    strings.TrimSpace(flags.promptOverlay),
		SpawnRole:        firstCLIValue(flags.spawnRole, "worker"),
		TTLSeconds:       flags.ttlSeconds,
		AutoStopOnParent: flags.autoStopOnParent,
		Permissions: SpawnPermissionPolicyRecord{
			Tools:           trimSpawnAtoms(flags.tools),
			Skills:          trimSpawnAtoms(flags.skills),
			MCPServers:      trimSpawnAtoms(flags.mcpServers),
			WorkspacePaths:  trimSpawnAtoms(flags.workspacePaths),
			NetworkChannels: trimSpawnAtoms(flags.networkChannels),
			SandboxProfiles: trimSpawnAtoms(flags.sandboxProfiles),
		},
		IdempotencyKey: strings.TrimSpace(flags.idempotencyKey),
	}
	if notifyFlagChanged {
		notifyCreator := !flags.noNotifyCreator
		request.NotifyCreator = &notifyCreator
	}
	return request, nil
}

func trimSpawnAtoms(values []string) []string {
	atoms := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		atoms = append(atoms, trimmed)
	}
	return atoms
}

func agentSpawnBundle(record *AgentSpawnRecord) outputBundle {
	if record == nil {
		record = &AgentSpawnRecord{}
	}
	return outputBundle{
		jsonValue: record,
		human: func() (string, error) {
			wake := "off"
			if record.Lineage.NotifyCreator {
				wake = "on-settle (default)"
			}
			return renderHumanBlocks(
				renderHumanSection("Spawn", []keyValue{
					{Label: spawnSessionValue, Value: stringOrDash(record.Session.ID)},
					{Label: spawnAgentValue, Value: stringOrDash(record.Session.AgentName)},
					{Label: spawnProviderValue, Value: stringOrDash(sessionRuntimeProvider(record.Session))},
					{Label: spawnWorkspaceValue, Value: stringOrDash(record.Session.WorkspaceID)},
					{Label: spawnParentValue, Value: stringOrDash(record.Lineage.ParentSessionID)},
					{Label: spawnRootValue, Value: stringOrDash(record.Lineage.RootSessionID)},
					{Label: "Depth", Value: fmt.Sprintf("%d", record.Lineage.SpawnDepth)},
					{Label: roleOutputLabel, Value: stringOrDash(record.Lineage.SpawnRole)},
					{Label: "TTL Expires", Value: stringOrDash(formatTimePtr(record.Lineage.TTLExpiresAt))},
					{Label: spawnWakeValue, Value: wake},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}
