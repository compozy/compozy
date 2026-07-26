package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/spf13/cobra"
)

func newLoopStatusCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID string
	cmd := &cobra.Command{
		Use:   loopStatusKey,
		Short: "Inspect one Loop run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			id, err := requiredLoopFlag(loopRunIDKey, runID)
			if err != nil {
				return err
			}
			response, err := client.GetLoopRun(cmd.Context(), workspaceID, id)
			if err != nil {
				return err
			}
			message := fmt.Sprintf("Loop run %s is %s", id, response.Run.Status)
			return writeCommandOutput(cmd, loopOutputBundle(response, message))
		},
	}
	addLoopRunIDFlags(cmd, &workspaceRef, &runID)
	return cmd
}

func newLoopRunsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, loopName, status, origin, originSession string
	var limit int
	cmd := &cobra.Command{
		Use:   loopRunsKey,
		Short: "List workspace Loop runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			response, err := client.ListLoopRuns(cmd.Context(), workspaceID, LoopRunListQuery{
				LoopName:      strings.TrimSpace(loopName),
				Status:        strings.TrimSpace(status),
				Origin:        strings.TrimSpace(origin),
				OriginSession: strings.TrimSpace(originSession),
				Limit:         limit,
			})
			if err != nil {
				return err
			}
			message := fmt.Sprintf("%d loop runs", len(response.Runs))
			return writeCommandOutput(cmd, loopOutputBundle(response, message))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&loopName, loopLoopKey, "", "Filter by Loop name")
	cmd.Flags().StringVar(&status, loopStatusKey, "", "Filter by Loop run status")
	cmd.Flags().StringVar(&origin, "origin", "", "Filter by run origin: catalog or session")
	cmd.Flags().StringVar(&originSession, "origin-session", "", "Filter by origin session ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum runs to return")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	return cmd
}

func newLoopTurnsCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, nodeID string
	var itemIndex, limit int
	var afterSeq int64
	cmd := &cobra.Command{
		Use:   loopTurnsKey,
		Short: "List one Loop run's Goal turn audit",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			id, err := requiredLoopFlag("run", runID)
			if err != nil {
				return err
			}
			query, err := goalTurnListQueryFromFlags(cmd, nodeID, itemIndex, afterSeq, limit)
			if err != nil {
				return err
			}
			page, err := client.ListGoalTurns(cmd.Context(), workspaceID, id, query)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, loopTurnsOutputBundle(page))
		},
	}
	cmd.Flags().StringVar(&workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(&runID, "run", "", "Loop run ID")
	cmd.Flags().StringVar(&nodeID, "node", "", "Filter by Goal node ID")
	cmd.Flags().IntVar(&itemIndex, "item", 0, "Filter by nonnegative item index")
	cmd.Flags().Int64Var(&afterSeq, "after-seq", 0, "Resume after this run-wide Goal sequence")
	cmd.Flags().IntVar(&limit, "limit", 0, "Page size from 1 to 200; defaults to 50")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	mustMarkFlagRequired(cmd, "run")
	return cmd
}

func goalTurnListQueryFromFlags(
	cmd *cobra.Command,
	nodeID string,
	itemIndex int,
	afterSeq int64,
	limit int,
) (GoalTurnListQuery, error) {
	nodeID = strings.TrimSpace(nodeID)
	if cmd.Flags().Changed("node") && nodeID == "" {
		return GoalTurnListQuery{}, fmt.Errorf("cli: --node must be nonempty when present")
	}
	if afterSeq < 0 {
		return GoalTurnListQuery{}, fmt.Errorf("cli: --after-seq must be nonnegative")
	}
	if cmd.Flags().Changed("limit") && (limit < 1 || limit > 200) {
		return GoalTurnListQuery{}, fmt.Errorf("cli: --limit must be between 1 and 200")
	}
	query := GoalTurnListQuery{NodeID: nodeID, AfterSeq: afterSeq, Limit: limit}
	if cmd.Flags().Changed("item") {
		if itemIndex < 0 {
			return GoalTurnListQuery{}, fmt.Errorf("cli: --item must be nonnegative")
		}
		if nodeID == "" {
			return GoalTurnListQuery{}, fmt.Errorf("cli: --item requires --node")
		}
		query.ItemIndex = &itemIndex
	}
	return query, nil
}

func loopTurnsOutputBundle(page contract.GoalTurnPage) outputBundle {
	bundle := loopOutputBundle(page, fmt.Sprintf("%d Goal turns", len(page.Turns)))
	bundle.jsonl = func(cmd *cobra.Command) error {
		return writeJSONLines(cmd, page.Turns)
	}
	return bundle
}

func newLoopRunActionCommand(deps commandDeps, verb string, short string) *cobra.Command {
	var workspaceRef, runID string
	cmd := &cobra.Command{
		Use:   verb,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			id, err := requiredLoopFlag(loopRunIDKey, runID)
			if err != nil {
				return err
			}
			if err := executeLoopRunAction(
				cmd.Context(),
				client,
				verb,
				workspaceID,
				id,
				agentCredentialsFromEnv(deps),
			); err != nil {
				return err
			}
			return writeLoopMutationOK(cmd, verb, id)
		},
	}
	addLoopRunIDFlags(cmd, &workspaceRef, &runID)
	return cmd
}

func newLoopApproveCommand(deps commandDeps) *cobra.Command {
	var workspaceRef, runID, gateID, decision string
	cmd := &cobra.Command{
		Use:   loopApproveKey,
		Short: "Apply one Loop human-gate decision",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, workspaceID, err := loopClientAndWorkspace(cmd, deps, workspaceRef)
			if err != nil {
				return err
			}
			id, err := requiredLoopFlag(loopRunIDKey, runID)
			if err != nil {
				return err
			}
			gate, err := requiredLoopFlag(loopGateIDKey, gateID)
			if err != nil {
				return err
			}
			normalizedDecision, err := parseLoopGateDecision(decision)
			if err != nil {
				return err
			}
			if err := client.ApproveLoopRun(cmd.Context(), workspaceID, id, contract.ApproveLoopRunRequest{
				GateID:   gate,
				Decision: contract.LoopGateDecision(normalizedDecision),
			}, agentCredentialsFromEnv(deps)); err != nil {
				return err
			}
			return writeLoopMutationOK(cmd, "approved", id)
		},
	}
	addLoopRunIDFlags(cmd, &workspaceRef, &runID)
	cmd.Flags().StringVar(&gateID, loopGateIDKey, "", "Gate node ID")
	cmd.Flags().StringVar(&decision, loopDecisionKey, "", "Decision: approve, request_changes, or reject")
	mustMarkFlagRequired(cmd, loopGateIDKey)
	mustMarkFlagRequired(cmd, loopDecisionKey)
	return cmd
}

func addLoopRunIDFlags(cmd *cobra.Command, workspaceRef *string, runID *string) {
	cmd.Flags().StringVar(workspaceRef, loopWorkspaceKey, "", "Workspace path, name, or ID")
	cmd.Flags().StringVar(runID, loopRunIDKey, "", "Loop run ID")
	mustMarkFlagRequired(cmd, loopWorkspaceKey)
	mustMarkFlagRequired(cmd, loopRunIDKey)
}
