package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
)

func (s *service) resolveRunParticipation(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	request *participation.Request,
	requestSource participation.Source,
	definition *participation.Request,
) (participation.Spec, error) {
	if s == nil || s.participationResolver == nil {
		if hasParticipationIntent(request) || hasParticipationIntent(definition) {
			return participation.Spec{}, fmt.Errorf(
				"%w: loop participation resolver is required for live or explicit intent",
				ErrActionDependencyMissing,
			)
		}
		return participation.LocalSpec(), nil
	}
	return s.participationResolver.Resolve(ctx, participation.ResolveInput{
		WorkspaceID: strings.TrimSpace(string(workspaceID)),
		Owner: participation.OwnerRef{
			WorkspaceID: strings.TrimSpace(string(workspaceID)),
			Kind:        participation.OwnerKindLoopRun,
			ID:          strings.TrimSpace(string(runID)),
		},
		Request:       request,
		RequestSource: requestSource,
		Definition:    definition,
		LoopRunID:     strings.TrimSpace(string(runID)),
	})
}

func (s *service) observeCommittedRunParticipation(ctx context.Context, run Run) {
	if s == nil || s.participationResolver == nil {
		return
	}
	observationCtx := context.WithoutCancel(ctx)
	observation := participation.ResolvedObservation{
		WorkspaceID: strings.TrimSpace(string(run.WorkspaceID)),
		Owner: participation.OwnerRef{
			WorkspaceID: strings.TrimSpace(string(run.WorkspaceID)),
			Kind:        participation.OwnerKindLoopRun,
			ID:          strings.TrimSpace(string(run.ID)),
		},
		Spec: run.NetworkSpecSnapshot(),
	}
	if err := participation.ObserveResolved(
		observationCtx,
		s.participationResolver,
		observation,
	); err != nil {
		s.logger.ErrorContext(
			observationCtx,
			"loop: committed network participation observation failed",
			"error", err,
			"workspace_id", observation.WorkspaceID,
			"owner_id", observation.Owner.ID,
		)
	}
}

func hasParticipationIntent(request *participation.Request) bool {
	if request == nil {
		return false
	}
	normalized, err := participation.NormalizeIntent(*request)
	return err != nil || normalized != (participation.Request{})
}

func validateLoopParticipation(graph dsl.Graph, spec participation.Spec) error {
	if spec.Mode == participation.ModeLive {
		return nil
	}
	node, found := firstNetworkUsingNode(graph)
	if !found {
		return nil
	}
	return fmt.Errorf(
		"%w: node %q uses %q and requires live participation",
		participation.ErrLoopRequiresLive,
		node.ID,
		networkNodeCapability(node),
	)
}

func firstNetworkUsingNode(graph dsl.Graph) (dsl.Node, bool) {
	for _, node := range graph.Nodes {
		if networkNodeCapability(node) != "" {
			return node, true
		}
		if node.Body != nil {
			if nested, found := firstNetworkUsingNode(*node.Body); found {
				return nested, true
			}
		}
	}
	return dsl.Node{}, false
}

func networkNodeCapability(node dsl.Node) string {
	kind := strings.TrimSpace(node.Kind)
	if strings.HasPrefix(kind, "agh__network_") {
		return kind
	}
	if node.Harvest != nil && strings.TrimSpace(node.Harvest.Kind) == harvestKindChannelResult {
		return harvestKindChannelResult
	}
	return ""
}
