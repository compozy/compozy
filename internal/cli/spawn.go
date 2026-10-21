package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	speedpkg "github.com/compozy/compozy/internal/speed"
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
	reasoningEffort  string
	speed            string
	acpOptions       []string
	acpToggles       []string
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
	toolsets         []string
	inheritPerms     bool
	worktree         string
	newWorktree      string
	baseRef          string
	branch           string
	prompt           string

	idempotencyKey string
}

type spawnRequestOptions struct {
	newWorktreeFlagChanged     bool
	notifyFlagChanged          bool
	reasoningEffortFlagChanged bool
	speedFlagChanged           bool
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
    --skill code-review

  # Spawn into a new managed worktree with the parent's grant and a first prompt
  compozy spawn \
    --agent general \
    --ttl-seconds 3600 \
    --new-worktree=feature-auth \
    --base main \
    --inherit-permissions \
    --prompt "Implement the auth task, commit, and report the branch."`,
		RunE: runSpawnCommand(deps, flags),
	}
	registerSpawnFlags(cmd, flags)
	return cmd
}

func registerSpawnFlags(cmd *cobra.Command, flags *spawnCommandFlags) {
	cmd.Flags().StringVar(&flags.agentName, "agent", "", "Child agent name")
	cmd.Flags().StringVar(&flags.provider, "provider", "", "Optional provider override")
	cmd.Flags().StringVar(&flags.model, "model", "", "Optional model override")
	cmd.Flags().StringVar(&flags.reasoningEffort, "reasoning-effort", "", "Optional reasoning effort override")
	cmd.Flags().StringVar(&flags.speed, "speed", "", "Optional runtime speed override (normal or fast)")
	bindACPOptionFlags(
		cmd,
		&flags.acpOptions,
		&flags.acpToggles,
		"ACP select override as id=value (repeatable)",
		"ACP boolean override as id=true|false (repeatable)",
	)
	cmd.Flags().StringVar(&flags.name, "name", "", "Optional child session display name")
	cmd.Flags().StringVar(&flags.workspace, "workspace", "", "Override child workspace (ID, name, or path)")
	cmd.Flags().StringVar(&flags.promptOverlay, "prompt-overlay", "", "Prompt overlay for the child session")
	cmd.Flags().StringVar(&flags.spawnRole, "role", "worker", "Child spawn role")
	cmd.Flags().Int64Var(&flags.ttlSeconds, "ttl-seconds", 0, "Mandatory child TTL in seconds")
	cmd.Flags().BoolVar(&flags.autoStopOnParent, "auto-stop-on-parent", true, "Stop the child when the parent stops")
	cmd.Flags().BoolVar(
		&flags.noNotifyCreator,
		"no-notify-creator",
		false,
		"Do not wake this session when the child needs attention or stops",
	)
	cmd.Flags().StringArrayVar(&flags.tools, "tool", nil, "Allowed tool atom (repeatable)")
	cmd.Flags().StringArrayVar(&flags.skills, "skill", nil, "Allowed skill atom (repeatable)")
	cmd.Flags().StringArrayVar(&flags.mcpServers, "mcp-server", nil, "Allowed MCP server id (repeatable)")
	cmd.Flags().
		StringArrayVar(&flags.workspacePaths, "workspace-path", nil, "Allowed workspace path grant (repeatable)")
	cmd.Flags().
		StringArrayVar(&flags.toolsets, "toolset", nil, "Select parent-granted tools by toolset ID (repeatable)")
	cmd.Flags().BoolVar(
		&flags.inheritPerms,
		"inherit-permissions",
		false,
		"Keep the parent's grant for every permission list not given by flags",
	)
	cmd.Flags().StringVar(&flags.worktree, "worktree", "", "Bind the child to an existing ready worktree")
	cmd.Flags().StringVar(
		&flags.newWorktree,
		"new-worktree",
		"",
		"Create and bind a managed worktree for the child; optionally provide its name",
	)
	cmd.Flags().Lookup("new-worktree").NoOptDefVal = sessionNewWorktreeAutoValue
	cmd.Flags().StringVar(&flags.baseRef, "base", "", "Base ref for --new-worktree")
	cmd.Flags().StringVar(&flags.branch, "branch", "", "Branch name for --new-worktree")
	cmd.Flags().StringVar(&flags.prompt, "prompt", "", "First prompt for the child; its answer wakes this session")
	cmd.Flags().StringVar(&flags.idempotencyKey, "idempotency-key", "", "Optional idempotency key")
	cmd.MarkFlagsMutuallyExclusive("worktree", "new-worktree")
	mustMarkFlagRequired(cmd, "agent")
	mustMarkFlagRequired(cmd, "ttl-seconds")
}

func runSpawnCommand(deps commandDeps, flags *spawnCommandFlags) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		request, err := flags.request(spawnRequestOptions{
			newWorktreeFlagChanged:     cmd.Flags().Changed("new-worktree"),
			notifyFlagChanged:          cmd.Flags().Changed("no-notify-creator"),
			reasoningEffortFlagChanged: cmd.Flags().Changed("reasoning-effort"),
			speedFlagChanged:           cmd.Flags().Changed("speed"),
		})
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

func (flags *spawnCommandFlags) request(
	options spawnRequestOptions,
) (AgentSpawnRequest, error) {
	if strings.TrimSpace(flags.agentName) == "" {
		return AgentSpawnRequest{}, fmt.Errorf("cli: --agent is required")
	}
	if flags.ttlSeconds <= 0 {
		return AgentSpawnRequest{}, fmt.Errorf("cli: --ttl-seconds must be positive")
	}
	if strings.EqualFold(strings.TrimSpace(flags.spawnRole), "coordinator") {
		return AgentSpawnRequest{}, fmt.Errorf("cli: coordinator spawn role is not supported")
	}
	if options.reasoningEffortFlagChanged {
		if err := validateAgentReasoningEffort(flags.reasoningEffort); err != nil {
			return AgentSpawnRequest{}, err
		}
	}
	var requestedSpeed speedpkg.Speed
	if options.speedFlagChanged {
		parsedSpeed, err := speedpkg.Parse(flags.speed)
		if err != nil {
			return AgentSpawnRequest{}, fmt.Errorf("cli: invalid --speed: %w", err)
		}
		requestedSpeed = parsedSpeed
	}
	acpOptions, err := parseACPFlags(flags.acpOptions, flags.acpToggles)
	if err != nil {
		return AgentSpawnRequest{}, err
	}
	request := AgentSpawnRequest{
		AgentName:        strings.TrimSpace(flags.agentName),
		Provider:         strings.TrimSpace(flags.provider),
		Model:            strings.TrimSpace(flags.model),
		ReasoningEffort:  contract.ReasoningEffort(strings.TrimSpace(flags.reasoningEffort)),
		Speed:            requestedSpeed,
		ACPOptions:       acpOptions,
		Name:             strings.TrimSpace(flags.name),
		Workspace:        strings.TrimSpace(flags.workspace),
		PromptOverlay:    strings.TrimSpace(flags.promptOverlay),
		SpawnRole:        firstCLIValue(flags.spawnRole, "worker"),
		TTLSeconds:       flags.ttlSeconds,
		AutoStopOnParent: flags.autoStopOnParent,
		Permissions: SpawnPermissionRequestRecord{
			Tools:          trimSpawnAtoms(flags.tools),
			Skills:         trimSpawnAtoms(flags.skills),
			MCPServers:     trimSpawnAtoms(flags.mcpServers),
			WorkspacePaths: trimSpawnAtoms(flags.workspacePaths),
		},
		InheritPermissions: flags.inheritPerms,
		Toolsets:           trimSpawnAtoms(flags.toolsets),
		Worktree:           strings.TrimSpace(flags.worktree),
		Prompt:             strings.TrimSpace(flags.prompt),
		IdempotencyKey:     strings.TrimSpace(flags.idempotencyKey),
	}
	newWorktree, err := flags.newWorktreeRequest(options.newWorktreeFlagChanged)
	if err != nil {
		return AgentSpawnRequest{}, err
	}
	request.NewWorktree = newWorktree
	if options.notifyFlagChanged {
		request.NotifyCreator = new(!flags.noNotifyCreator)
	}
	return request, nil
}

func (flags *spawnCommandFlags) newWorktreeRequest(changed bool) (*contract.SpawnNewWorktreeRequest, error) {
	baseRef, branch := strings.TrimSpace(flags.baseRef), strings.TrimSpace(flags.branch)
	if !changed {
		if baseRef != "" || branch != "" {
			return nil, fmt.Errorf("cli: --base and --branch require --new-worktree")
		}
		return nil, nil
	}
	name := strings.TrimSpace(flags.newWorktree)
	if name == sessionNewWorktreeAutoValue {
		name = ""
	}
	return &contract.SpawnNewWorktreeRequest{Name: name, BaseRef: baseRef, Branch: branch}, nil
}

// trimSpawnAtoms keeps an absent flag nil so inherit-permissions can tell it apart
// from an explicitly empty list.
func trimSpawnAtoms(values []string) []string {
	if values == nil {
		return nil
	}
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
					{Label: spawnProviderValue, Value: stringOrDash(sessionRuntimeProvider(&record.Session))},
					{Label: spawnWorkspaceValue, Value: stringOrDash(record.Session.WorkspaceID)},
					{Label: spawnParentValue, Value: stringOrDash(record.Lineage.ParentSessionID)},
					{Label: spawnRootValue, Value: stringOrDash(record.Lineage.RootSessionID)},
					{Label: "Depth", Value: fmt.Sprintf("%d", record.Lineage.SpawnDepth)},
					{Label: roleOutputLabel, Value: stringOrDash(record.Lineage.SpawnRole)},
					{Label: "TTL Expires", Value: stringOrDash(formatTimePtr(record.Lineage.TTLExpiresAt))},
					{Label: spawnWakeValue, Value: wake},
					{Label: "Worktree", Value: spawnWorktreeSummary(record.Worktree)},
					{Label: "First Prompt", Value: spawnPromptSummary(record.Prompt)},
				}),
			), nil
		},
		toon: func() (string, error) {
			return renderJSONPreview(record)
		},
	}
}

func spawnWorktreeSummary(worktree *contract.SpawnWorktreePayload) string {
	if worktree == nil {
		return "-"
	}
	return fmt.Sprintf("%s (%s) %s", worktree.Name, worktree.Branch, worktree.Path)
}

func spawnPromptSummary(prompt *contract.SpawnPromptPayload) string {
	if prompt == nil {
		return "-"
	}
	return fmt.Sprintf("%s %s", prompt.MessageID, prompt.Status)
}
