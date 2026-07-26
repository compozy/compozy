package cli

import (
	"errors"

	"strings"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"

	"github.com/spf13/cobra"
)

const (
	memoryTypeKey = "type"
)

const (
	memoryFilenameValue = "Filename"
	memoryFilenameKey   = "filename"
	memorySummaryKey    = "summary"
)

const (
	memoryActiveValue      = "Active"
	memoryAgentValue       = "Agent"
	memoryDescriptionValue = "Description"
	memoryEnabledValue     = "Enabled"
	memoryOperationValue   = "Operation"
	memoryReasonValue      = "Reason"
	memorySourceValue      = "Source"
	memoryStatusValue      = "Status"
	memoryActiveKey        = "active"
	memoryAdhocKey         = "adhoc"
	memoryAgentNameKey     = "agent_name"
	memoryContentKey       = "content"
	memoryDailyKey         = "daily"
	memoryDecisionsKey     = "decisions"
	memoryDescriptionKey   = "description"
	memoryDrainKey         = "drain"
	memoryDreamKey         = "dream"
	memoryEnabledKey       = "enabled"
	memoryExtractorKey     = "extractor"
	memoryHealthKey        = "health"
	memoryHistoryKey       = "history"
	memoryListKey          = "list"
	memoryMemoryKey        = "memory"
	memoryOperationKey     = "operation"
	memoryPayloadKey       = "payload"
	memoryScopeShowValue   = "scope-show"
	memorySearchQueryValue = "search <query>"
	memoryStatusKey        = "status"
	memoryTriggerKey       = "trigger"
)

type memorySelectorFlags struct {
	Scope     string
	Workspace string
	Agent     string
	AgentTier string
}

type memorySelectorOptions struct {
	DefaultScope     memcontract.Scope
	DefaultWorkspace bool
}

type memoryListItem struct {
	Filename        string                `json:"filename"`
	Name            string                `json:"name"`
	Type            memcontract.Type      `json:"type"`
	Scope           memcontract.Scope     `json:"scope"`
	WorkspaceID     string                `json:"workspace_id,omitempty"`
	AgentName       string                `json:"agent_name,omitempty"`
	AgentTier       memcontract.AgentTier `json:"agent_tier,omitempty"`
	Age             string                `json:"age"`
	Description     string                `json:"description,omitempty"`
	StalenessBanner string                `json:"staleness_banner,omitempty"`
	ModTime         time.Time             `json:"mod_time"`
}

type memorySearchItem struct {
	Filename      string                `json:"filename"`
	Name          string                `json:"name"`
	Type          memcontract.Type      `json:"type"`
	Scope         memcontract.Scope     `json:"scope"`
	AgentTier     memcontract.AgentTier `json:"agent_tier,omitempty"`
	Score         float64               `json:"score"`
	Snippet       string                `json:"snippet,omitempty"`
	WhyRecalled   []string              `json:"why_recalled,omitempty"`
	ShadowedBy    string                `json:"shadowed_by,omitempty"`
	AlreadyShown  bool                  `json:"already_shown"`
	StalenessNote string                `json:"staleness_banner,omitempty"`
}

type memoryHistoryItem struct {
	ID          string                `json:"id"`
	Operation   memcontract.Operation `json:"operation"`
	Scope       memcontract.Scope     `json:"scope,omitempty"`
	WorkspaceID string                `json:"workspace_id,omitempty"`
	Filename    string                `json:"filename,omitempty"`
	AgentName   string                `json:"agent_name,omitempty"`
	AgentTier   memcontract.AgentTier `json:"agent_tier,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Age         string                `json:"age"`
	Timestamp   time.Time             `json:"timestamp"`
}

func newMemoryCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   memoryMemoryKey,
		Short: "Show, write, search, and operate Memory v2 durable context",
	}

	cmd.AddCommand(newMemoryListCommand(deps))
	cmd.AddCommand(newMemoryShowCommand(deps))
	cmd.AddCommand(newMemoryWriteCommand(deps))
	cmd.AddCommand(newMemoryEditCommand(deps))
	cmd.AddCommand(newMemoryDeleteCommand(deps))
	cmd.AddCommand(newMemorySearchCommand(deps))
	cmd.AddCommand(newMemoryReindexCommand(deps))
	cmd.AddCommand(newMemoryHistoryCommand(deps))
	cmd.AddCommand(newMemoryHealthCommand(deps))
	cmd.AddCommand(newMemoryPromoteCommand(deps))
	cmd.AddCommand(newMemoryResetCommand(deps))
	cmd.AddCommand(newMemoryReloadCommand(deps))
	cmd.AddCommand(newMemoryScopeShowCommand(deps))
	cmd.AddCommand(newMemoryDecisionsCommand(deps))
	cmd.AddCommand(newMemoryRecallCommand(deps))
	cmd.AddCommand(newMemoryDreamCommand(deps))
	cmd.AddCommand(newMemoryDailyCommand(deps))
	cmd.AddCommand(newMemoryExtractorCommand(deps))
	cmd.AddCommand(newMemoryProviderCommand(deps))
	cmd.AddCommand(newMemoryAdhocCommand())
	return cmd
}

func newMemoryShowCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var includeSystem bool

	cmd := &cobra.Command{
		Use:   "show <filename>",
		Short: "Show one Memory v2 entry",
		Example: `  # Show a workspace memory entry
  agh memory show runtime-notes.md --scope workspace

  # Show an agent-global memory entry as JSON
  agh memory show prefs.md --scope agent --agent reviewer --agent-tier global -o json`,
		Args: exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			selector.IncludeSystem = includeSystem
			response, err := client.ShowMemory(cmd.Context(), strings.TrimSpace(args[0]), selector)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryEntryBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().BoolVar(&includeSystem, "include-system", false, "Allow showing _system memory entries")
	return cmd
}

func newMemoryWriteCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var typeRaw string
	var name string
	var description string
	var contentFlag string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "write --type <type> --name <name> --content <@file|text>",
		Short: "Create a Memory v2 entry through the controller",
		Example: `  # Write workspace-scoped project memory from a file
  agh memory write --scope workspace --type project --name "Runtime docs" --content @runtime.md

  # Write agent-global feedback
  agh memory write --scope agent --agent reviewer --agent-tier global \
    --type feedback --name "Review tone" --content @feedback.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			typ, err := parseRequiredMemoryType(typeRaw)
			if err != nil {
				return err
			}
			defaultScope, err := memcontract.DefaultScopeForType(typ)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultScope: defaultScope})
			if err != nil {
				return err
			}
			content, err := resolveMemoryContent(cmd, deps, contentFlag)
			if err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return errors.New("memory.name_required: --name is required")
			}
			response, err := client.CreateMemory(cmd.Context(), MemoryCreateRequest{
				Scope:       selector.Scope,
				WorkspaceID: selector.WorkspaceID,
				AgentName:   selector.AgentName,
				AgentTier:   selector.AgentTier,
				Origin:      memcontract.OriginCLI,
				Type:        typ,
				Name:        strings.TrimSpace(name),
				Description: strings.TrimSpace(description),
				Content:     content,
				DryRun:      dryRun,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryMutationBundle("Memory Write", response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().StringVar(&typeRaw, memoryTypeKey, "", "Memory type: user, feedback, project, or reference")
	cmd.Flags().StringVar(&name, automationNameKey, "", "Memory display name")
	cmd.Flags().StringVar(&description, memoryDescriptionKey, "", "One-line durable memory description")
	cmd.Flags().
		StringVar(&contentFlag, memoryContentKey, "", "Memory content; use @file to read from disk or - for stdin")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Ask the controller for a decision without applying it")
	mustMarkFlagRequired(cmd, memoryTypeKey)
	mustMarkFlagRequired(cmd, automationNameKey)
	mustMarkFlagRequired(cmd, memoryContentKey)
	return cmd
}

func newMemoryEditCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var typeRaw string
	var name string
	var description string
	var contentFlag string
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "edit <filename> --content <@file|text>",
		Short: "Edit a Memory v2 entry through the controller",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			typ, err := parseOptionalMemoryType(typeRaw)
			if err != nil {
				return err
			}
			content, err := resolveMemoryContent(cmd, deps, contentFlag)
			if err != nil {
				return err
			}
			response, err := client.EditMemory(cmd.Context(), strings.TrimSpace(args[0]), MemoryEditRequest{
				Scope:       selector.Scope,
				WorkspaceID: selector.WorkspaceID,
				AgentName:   selector.AgentName,
				AgentTier:   selector.AgentTier,
				Type:        typ,
				Name:        strings.TrimSpace(name),
				Description: strings.TrimSpace(description),
				Content:     content,
				DryRun:      dryRun,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryMutationBundle("Memory Edit", response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().StringVar(&typeRaw, memoryTypeKey, "", "Memory type override")
	cmd.Flags().StringVar(&name, automationNameKey, "", "Memory display name override")
	cmd.Flags().StringVar(&description, memoryDescriptionKey, "", "Memory description override")
	cmd.Flags().
		StringVar(&contentFlag, memoryContentKey, "", "Memory content; use @file to read from disk or - for stdin")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Ask the controller for a decision without applying it")
	mustMarkFlagRequired(cmd, memoryContentKey)
	return cmd
}

func newMemoryDeleteCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags

	cmd := &cobra.Command{
		Use:   "delete <filename>",
		Short: "Delete a Memory v2 entry through the controller",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			response, err := client.DeleteMemory(cmd.Context(), strings.TrimSpace(args[0]), selector)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryDeleteBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	return cmd
}

func newMemorySearchCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var topK int
	var includeSystem bool

	cmd := &cobra.Command{
		Use:   memorySearchQueryValue,
		Short: "Search deterministic Memory v2 recall",
		Example: `  # Search global and current-workspace memories
  agh memory search "auth sessions"

  # Search agent memory with system entries included
  agh memory search "review tone" --scope agent --agent reviewer --agent-tier global --include-system`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(deps, flags, memorySelectorOptions{DefaultWorkspace: true})
			if err != nil {
				return err
			}
			query := strings.TrimSpace(strings.Join(args, " "))
			if query == "" {
				return errors.New("memory.query_required: query is required")
			}
			response, err := client.SearchMemory(cmd.Context(), MemorySearchRequest{
				QueryText:     query,
				Scope:         selector.Scope,
				WorkspaceID:   selector.WorkspaceID,
				AgentName:     selector.AgentName,
				AgentTier:     selector.AgentTier,
				TopK:          topK,
				IncludeSystem: includeSystem,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memorySearchBundle(response))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().IntVar(&topK, "top-k", 0, "Maximum number of recalled entries")
	cmd.Flags().BoolVar(&includeSystem, "include-system", false, "Include _system memory entries")
	return cmd
}
