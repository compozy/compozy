package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/workspaceaccess"
	"github.com/spf13/cobra"
)

// refuseAgentInstanceClearMain keeps clear-main operator-only, matching the native tool. The
// daemon client sends no agent credentials, so an agent session must be refused before any call.
func refuseAgentInstanceClearMain(deps commandDeps, action string) error {
	if action == agentInstanceClearMainAction && agentIdentityEnvironmentPresent(deps) {
		return errors.New("clear_main_operator_only: only the operator can clear a main session")
	}
	return nil
}

// clearAgentInstanceMain is the operator recovery for a main session that cannot resume. It
// keeps the instance, the session id, queued inputs and agent memory; only the conversation
// and its provider thread are replaced.
func clearAgentInstanceMain(cmd *cobra.Command, base any, client agentInstancesClient, id string) error {
	clearer, ok := base.(agentInstanceMainClearer)
	if !ok {
		return errors.New("cli: session clear unavailable")
	}
	current, err := client.MutateAgentInstance(cmd.Context(), id, configShowKey)
	if err != nil {
		return err
	}
	mainSessionID := strings.TrimSpace(current.Instance.MainSessionID)
	if mainSessionID == "" {
		return fmt.Errorf(
			"cli: instance %s has no main session; run `compozy agent instance ensure-main %s`", id, id,
		)
	}
	workspaceRef := strings.TrimSpace(current.Instance.WorkspaceID)
	if workspaceRef == "" {
		// An instance without a project belongs to the User scope.
		workspaceRef = workspaceaccess.UserSelector
	}
	if _, err := clearer.ClearSessionConversation(cmd.Context(), workspaceRef, mainSessionID); err != nil {
		return err
	}
	return writeInstanceOutput(cmd, current, instanceActionText(current.Instance, agentInstanceClearMainAction))
}
