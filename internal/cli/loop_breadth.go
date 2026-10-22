package cli

import (
	"context"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

func loopListClientAndWorkspace(
	cmd *cobra.Command,
	deps commandDeps,
	workspace string,
	all bool,
) (loopCommandClient, string, error) {
	if all {
		client, err := loopClientFromDeps(deps)
		return client, "", err
	}
	return loopClientAndWorkspace(cmd, deps, workspace)
}

func loopCatalogBatchBundle(batch contract.LoopCatalogBatchResponse, labels map[string]string) outputBundle {
	bundle := loopOutputBundle(batch, "Loop definitions")
	bundle.human = func() (string, error) {
		rows := make([][]string, 0, len(batch.Entries))
		for _, entry := range batch.Entries {
			rows = append(rows, []string{labels[entry.WorkspaceID], entry.Name, entry.Source.Layer})
		}
		blocks := []string{renderHumanTable("Loops", []string{cliProjectHeader, "NAME", "LAYER"}, rows)}
		for _, failure := range batch.Errors {
			blocks = append(blocks, loopProjectLabel(failure.WorkspaceID)+": "+failure.Code+": "+failure.Message)
		}
		if batch.Truncated {
			blocks = append(blocks, "Results truncated (200 per project, 2000 total).")
		}
		return renderHumanBlocks(blocks...), nil
	}
	return bundle
}

func loopProjectLabel(workspace string) string {
	if workspace == "" {
		return cliUserScopeLabel
	}
	return workspace
}

func loopProjectNames(ctx context.Context, client loopCommandClient, ids []string) (map[string]string, error) {
	labels := map[string]string{"": cliUserScopeLabel}
	if len(ids) == 0 {
		return labels, nil
	}
	workspaces, err := client.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	for _, ws := range workspaces {
		labels[ws.ID] = ws.Name
	}
	for _, id := range ids {
		if _, ok := labels[id]; !ok {
			labels[id] = id
		}
	}
	return labels, nil
}
