package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/memory"
	"github.com/spf13/cobra"
)

func newMemoryListCommand(deps commandDeps) *cobra.Command {
	var flags memorySelectorFlags
	var typeRaw string
	var sortRaw string
	var cursor string
	var limit int
	var includeSystem bool

	cmd := &cobra.Command{
		Use:   memoryListKey,
		Short: "List Memory v2 entries",
		Example: `  # List the newest global and current-workspace memories
  agh memory list

  # Walk agent-workspace memories by normalized name
  agh memory list --scope agent --agent reviewer --agent-tier workspace --sort name --limit 50`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if limit < 0 || limit > memory.MaxHeaderListLimit {
				return fmt.Errorf(
					"memory limit must be between 1 and %d when provided: %d",
					memory.MaxHeaderListLimit,
					limit,
				)
			}
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			selector, err := resolveMemorySelectorFlags(
				deps,
				flags,
				memorySelectorOptions{DefaultWorkspace: true},
			)
			if err != nil {
				return err
			}
			typ, err := parseOptionalMemoryType(typeRaw)
			if err != nil {
				return err
			}
			sortKey, err := parseMemoryListSort(sortRaw)
			if err != nil {
				return err
			}
			selector.IncludeSystem = includeSystem
			response, err := client.ListMemory(cmd.Context(), MemoryListQuery{
				MemorySelectorQuery: selector,
				Type:                typ,
				Sort:                sortKey,
				Cursor:              strings.TrimSpace(cursor),
				Limit:               limit,
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, memoryListBundle(response, deps.now))
		},
	}
	addMemorySelectorFlags(cmd, &flags)
	cmd.Flags().StringVar(&typeRaw, memoryTypeKey, "", "Memory type: user, feedback, project, or reference")
	cmd.Flags().StringVar(&sortRaw, "sort", "", "Sort order: recent or name")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Continue after a memory list cursor")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum memories to return (default 50, max 200)")
	cmd.Flags().BoolVar(&includeSystem, "include-system", false, "Include _system memory entries")
	return cmd
}

func parseMemoryListSort(raw string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", memory.HeaderListSortRecent, memory.HeaderListSortName:
		return normalized, nil
	default:
		return "", fmt.Errorf("memory sort must be recent or name: %q", raw)
	}
}
