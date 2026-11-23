package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentSkillInstallConflictsClient interface {
	ListAgentSkillInstallConflicts(context.Context, url.Values) (contract.AgentSkillInstallConflictsResponse, error)
	ResolveAgentSkillInstallConflict(
		context.Context, url.Values, string, string,
	) (contract.AgentSkillInstallConflictResolveResponse, error)
}

func (c *daemonClient) ListAgentSkillInstallConflicts(
	ctx context.Context,
	query url.Values,
) (contract.AgentSkillInstallConflictsResponse, error) {
	var result contract.AgentSkillInstallConflictsResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/agent-skill-install-conflicts", query, nil, &result)
	return result, err
}

func (c *daemonClient) ResolveAgentSkillInstallConflict(
	ctx context.Context,
	query url.Values,
	opID, keep string,
) (contract.AgentSkillInstallConflictResolveResponse, error) {
	var result contract.AgentSkillInstallConflictResolveResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/agent-skill-install-conflicts/"+url.PathEscape(opID)+"/resolve",
		query, contract.AgentSkillInstallConflictResolveRequest{Keep: keep}, &result)
	return result, err
}

func newAgentSkillInstallConflictCommand(deps commandDeps) *cobra.Command {
	root := &cobra.Command{
		Use:   "install-conflict",
		Short: "Resolve skills changed while the daemon was replacing them (operator only)",
	}
	list := &cobra.Command{
		Use: agentListKey, Short: "List skills with an unresolved install conflict", Args: cobra.NoArgs,
	}
	addWorkspaceFlag(list)
	list.RunE = func(cmd *cobra.Command, _ []string) error {
		client, query, err := agentSkillInstallConflictsScope(cmd, deps)
		if err != nil {
			return err
		}
		result, err := client.ListAgentSkillInstallConflicts(cmd.Context(), query)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, agentSkillInstallConflictTable(result.Conflicts))
	}
	var keep string
	resolve := &cobra.Command{
		Use:   "resolve <op-id>",
		Short: "Keep the current or the previous version of a conflicted skill",
		Args:  exactOneNonBlankArg(),
	}
	addWorkspaceFlag(resolve)
	resolve.Flags().StringVar(&keep, "keep", "", "current (the edited version) or previous")
	resolve.RunE = func(cmd *cobra.Command, args []string) error {
		if keep != "current" && keep != "previous" {
			return errors.New("--keep must be current or previous")
		}
		client, query, err := agentSkillInstallConflictsScope(cmd, deps)
		if err != nil {
			return err
		}
		result, err := client.ResolveAgentSkillInstallConflict(cmd.Context(), query, args[0], keep)
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, agentSkillInstallConflictResolvedText(result))
	}
	root.AddCommand(list, resolve)
	return root
}

func agentSkillInstallConflictsScope(
	cmd *cobra.Command,
	deps commandDeps,
) (agentSkillInstallConflictsClient, url.Values, error) {
	base, err := clientFromDeps(deps)
	if err != nil {
		return nil, nil, err
	}
	client, ok := base.(agentSkillInstallConflictsClient)
	if !ok {
		return nil, nil, errors.New("cli: skill install conflicts unavailable")
	}
	scope, err := agentQueryFromCommand(cmd, deps, base)
	if err != nil {
		return nil, nil, err
	}
	query := url.Values{}
	if scope.Workspace != "" {
		query.Set("workspace", scope.Workspace)
	}
	return client, query, nil
}

func agentSkillInstallConflictTable(conflicts []contract.AgentSkillInstallConflictPayload) string {
	rows := make([][]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		since := ""
		if conflict.Since != nil {
			since = conflict.Since.UTC().Format(time.RFC3339)
		}
		rows = append(rows, []string{conflict.OpID, conflict.Skill, conflict.Action, conflict.State, since})
	}
	return renderHumanTable("", []string{"OP", "SKILL", "ACTION", cliStateHeader, "SINCE"}, rows) + "\n"
}

func agentSkillInstallConflictResolvedText(result contract.AgentSkillInstallConflictResolveResponse) string {
	if result.AlreadyResolved {
		return fmt.Sprintf("%s was already resolved with --keep %s.\n", result.OpID, result.Keep)
	}
	if result.Keep == "previous" {
		return fmt.Sprintf("Restored the previous %s. The edited copy is in %s/.\n", result.Skill, result.KeptCopy)
	}
	return fmt.Sprintf("Kept the current %s and discarded the previous copy.\n", result.Skill)
}
