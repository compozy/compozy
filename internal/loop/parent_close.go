package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

type parentCloseAction struct {
	ChildRunID RunID
	NodeID     NodeID
	Policy     dsl.ParentClosePolicy
}

func (s *service) parentCloseActions(
	ctx context.Context,
	workspaceID WorkspaceID,
	parentRunID RunID,
) ([]parentCloseAction, error) {
	reader, ok := s.store.(GenerationOutputReader)
	if !ok {
		return nil, nil
	}
	parent, err := s.store.GetLoopRun(ctx, workspaceID, parentRunID)
	if err != nil {
		return nil, err
	}
	if parent.Generation < 1 || strings.TrimSpace(parent.DefinitionDigest) == "" {
		return nil, nil
	}
	snapshot, err := s.store.GetLoopDefinitionSnapshot(ctx, workspaceID, parent.DefinitionDigest)
	if err != nil {
		return nil, err
	}
	resolved, err := LoadExecutedDefinitionSnapshot(snapshot.Definition, parent.DefinitionDigest)
	if err != nil {
		return nil, err
	}
	outputs, err := reader.ListGenerationOutputs(ctx, workspaceID, parentRunID, parent.Generation)
	if err != nil {
		return nil, err
	}
	return selectParentCloseActions(resolved.Definition.Graph, outputs)
}

func selectParentCloseActions(
	graph dsl.Graph,
	outputs []GenerationOutput,
) ([]parentCloseAction, error) {
	actions := make([]parentCloseAction, 0)
	seen := make(map[RunID]struct{})
	for _, output := range outputs {
		childRunID := RunID(strings.TrimSpace(output.ChildLoopRunID))
		if childRunID == "" {
			continue
		}
		node, found := graphNode(graph, dsl.NodeID(output.NodeID))
		if !found || node.Class != dsl.NodeClassAction || dsl.ActionKind(node.Kind) != dsl.ActionRunLoop {
			return nil, fmt.Errorf(
				"%w: child Loop run %q has no owning run-loop node %q",
				ErrValidation,
				childRunID,
				output.NodeID,
			)
		}
		policy := node.OnParentClose
		if policy == "" {
			policy = dsl.ParentCloseTerminate
		}
		if !dsl.IsKnownParentClosePolicy(policy) {
			return nil, fmt.Errorf("%w: invalid parent-close policy %q", ErrValidation, policy)
		}
		if policy == dsl.ParentCloseAbandon {
			continue
		}
		if _, duplicate := seen[childRunID]; duplicate {
			continue
		}
		seen[childRunID] = struct{}{}
		actions = append(actions, parentCloseAction{
			ChildRunID: childRunID,
			NodeID:     NodeID(output.NodeID),
			Policy:     policy,
		})
	}
	return actions, nil
}

func parentCloseActionsForNode(actions []parentCloseAction, nodeID NodeID) []parentCloseAction {
	filtered := make([]parentCloseAction, 0, len(actions))
	for _, action := range actions {
		if action.NodeID == nodeID {
			filtered = append(filtered, action)
		}
	}
	return filtered
}

func attachCoordinatorParentCloseIntents(
	parent Run,
	resolved *ResolvedDefinition,
	plan *task.CoordinatorCompletionPlan,
) error {
	if resolved == nil || plan == nil || plan.Terminal == nil {
		return nil
	}
	payload, err := GenerationSnapshotPayloadFrom(plan.Snapshot.Payload)
	if err != nil {
		return err
	}
	actions, err := selectParentCloseActions(resolved.Definition.Graph, payload.Outputs)
	if err != nil {
		return err
	}
	for _, action := range actions {
		plan.ParentCloses = append(plan.ParentCloses, task.CoordinatorParentCloseSpec{
			ParentLoopRunID: string(parent.ID),
			ChildLoopRunID:  string(action.ChildRunID),
			Policy:          string(action.Policy),
			ParentStatus:    strings.TrimSpace(plan.Terminal.Status),
		})
	}
	return nil
}

func (s *service) applyParentCloseActions(
	ctx context.Context,
	parentMutation CancellationMutation,
	actions []parentCloseAction,
) error {
	for _, action := range actions {
		reason := "parent Loop run " + string(parentMutation.RunID) + " closed"
		var err error
		switch action.Policy {
		case dsl.ParentCloseTerminate:
			err = s.KillRun(
				ctx, parentMutation.WorkspaceID, action.ChildRunID, reason, parentMutation.Actor,
			)
		case dsl.ParentCloseCancel:
			err = s.CancelRun(
				ctx, parentMutation.WorkspaceID, action.ChildRunID, reason, parentMutation.Actor,
			)
		default:
			return fmt.Errorf("%w: invalid parent-close policy %q", ErrValidation, action.Policy)
		}
		if err != nil {
			return fmt.Errorf("apply parent-close %s to child %q: %w", action.Policy, action.ChildRunID, err)
		}
	}
	return nil
}
