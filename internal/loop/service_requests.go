package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/task"
)

// ListRequests returns one workspace-scoped request inventory page.
func (s *service) ListRequests(
	ctx context.Context,
	workspaceID WorkspaceID,
	query RequestQuery,
) (RequestPage, error) {
	store, ok := s.store.(RequestStore)
	if !ok {
		return RequestPage{}, fmt.Errorf("%w: request store is unavailable", ErrActionDependencyMissing)
	}
	page, err := store.ListRequests(ctx, workspaceID, query)
	if err != nil {
		return RequestPage{}, err
	}
	for index := range page.Items {
		if err := s.hydrateRequestResponderPolicy(ctx, workspaceID, &page.Items[index]); err != nil {
			return RequestPage{}, err
		}
	}
	return page, nil
}

// GetRequest returns the full redacted request detail.
func (s *service) GetRequest(
	ctx context.Context,
	workspaceID WorkspaceID,
	ref RequestRef,
) (RequestDetail, error) {
	store, ok := s.store.(RequestStore)
	if !ok {
		return RequestDetail{}, fmt.Errorf("%w: request store is unavailable", ErrActionDependencyMissing)
	}
	request, err := store.GetRequest(ctx, workspaceID, ref, true)
	if err != nil {
		return RequestDetail{}, err
	}
	if err := s.hydrateRequestResponderPolicy(ctx, workspaceID, &request); err != nil {
		return RequestDetail{}, err
	}
	return request, nil
}

func (s *service) hydrateRequestResponderPolicy(
	ctx context.Context,
	workspaceID WorkspaceID,
	request *Request,
) error {
	request.Agents = dsl.ResponderAgentsDeny
	run, err := s.store.GetLoopRun(ctx, workspaceID, request.LoopRunID)
	if err != nil {
		return err
	}
	resolved, err := s.pinnedResolvedDefinition(ctx, run)
	if err != nil {
		return err
	}
	node, found := graphNode(resolved.Definition.Graph, request.NodeID)
	if !found {
		return fmt.Errorf("%w: request node %q is not in the pinned definition", ErrValidation, request.NodeID)
	}
	request.Agents, err = nodeResponderPolicy(node, request.Kind)
	if err != nil {
		return err
	}
	return nil
}

// Respond validates responder policy before admitting one request answer.
func (s *service) Respond(ctx context.Context, input RespondInput) (RespondResult, error) {
	if err := input.Actor.Validate(); err != nil {
		return RespondResult{}, fmt.Errorf("%w: actor context: %w", ErrValidation, err)
	}
	input.WorkspaceID = WorkspaceID(strings.TrimSpace(string(input.WorkspaceID)))
	input.RunID = RunID(strings.TrimSpace(string(input.RunID)))
	input.NodeID = NodeID(strings.TrimSpace(string(input.NodeID)))
	input.Decision = strings.TrimSpace(input.Decision)
	input.Note = strings.TrimSpace(input.Note)
	if input.WorkspaceID == "" || input.RunID == "" || input.NodeID == "" || input.ItemIndex < 0 {
		return RespondResult{}, fmt.Errorf("%w: request response identity is incomplete", ErrValidation)
	}
	run, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return RespondResult{}, err
	}
	resolved, err := s.pinnedResolvedDefinition(ctx, run)
	if err != nil {
		return RespondResult{}, err
	}
	node, found := graphNode(resolved.Definition.Graph, input.NodeID)
	if !found {
		return RespondResult{}, fmt.Errorf(
			"%w: request node %q is not in the pinned definition",
			ErrValidation,
			input.NodeID,
		)
	}
	input.RequestKind, input.RejectRoute, err = respondNodeContract(node, input.Decision)
	if err != nil {
		return RespondResult{}, err
	}
	if err := s.rejectSelfOperation(ctx, input.WorkspaceID, input.RunID, input.Actor); err != nil {
		return RespondResult{}, err
	}
	if input.Actor.Actor.Kind.Normalize() == task.ActorKindAgentSession {
		allowed, err := s.requestAllowsAgentResponder(ctx, input)
		if err != nil {
			return RespondResult{}, err
		}
		if !allowed {
			return RespondResult{}, NewRequestReasonError(
				ReasonCodeRespondNotPermitted, ErrRespondNotPermitted, nil,
			)
		}
	}
	store, ok := s.store.(RequestStore)
	if !ok {
		return RespondResult{}, fmt.Errorf("%w: request store is unavailable", ErrActionDependencyMissing)
	}
	result, err := store.RespondRequest(ctx, input)
	if err != nil {
		return RespondResult{}, err
	}
	if result.Coordinator != nil && s.coordinatorActivator != nil {
		s.coordinatorActivator.ActivateCoordinatorRun(context.WithoutCancel(ctx), *result.Coordinator)
	}
	return result, nil
}

func respondNodeContract(node dsl.Node, decision string) (string, NodeID, error) {
	if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlAsk {
		if decision != "" && decision != RequestDecisionRespond {
			return "", "", NewRequestReasonError(
				ReasonCodeRequestValidationFailed,
				fmt.Errorf("%w: ask only accepts respond", ErrRequestValidationFailed),
				map[string]string{"decision": "ask only accepts respond"},
			)
		}
		return RequestKindAsk, "", nil
	}
	if node.Class != dsl.NodeClassAction || node.Review == nil {
		return "", "", fmt.Errorf("%w: node %q does not own a request", ErrValidation, node.ID)
	}
	if decision == "" {
		return "", "", NewRequestReasonError(
			ReasonCodeRequestValidationFailed,
			fmt.Errorf("%w: review decision is required", ErrRequestValidationFailed),
			map[string]string{"decision": jsonSchemaRequiredKey},
		)
	}
	var route NodeID
	if node.Review.OnReject != nil {
		route = node.Review.OnReject.Route
	}
	return RequestKindReview, route, nil
}

func (s *service) requestAllowsAgentResponder(ctx context.Context, input RespondInput) (bool, error) {
	run, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return false, err
	}
	resolved, err := s.pinnedResolvedDefinition(ctx, run)
	if err != nil {
		return false, err
	}
	node, found := graphNode(resolved.Definition.Graph, input.NodeID)
	if !found {
		return false, fmt.Errorf("%w: request node %q is not in the pinned definition", ErrValidation, input.NodeID)
	}
	policy, err := nodeResponderPolicy(node, "")
	return policy == dsl.ResponderAgentsAllow, err
}

func nodeResponderPolicy(node dsl.Node, requestKind string) (dsl.ResponderAgentPolicy, error) {
	if node.Class == dsl.NodeClassControl && dsl.ControlKind(node.Kind) == dsl.ControlAsk &&
		(requestKind == "" || requestKind == RequestKindAsk) {
		var params dsl.AskParams
		if err := node.Params.Decode(&params); err != nil {
			return "", fmt.Errorf("loop: decode pinned ask node %q: %w", node.ID, err)
		}
		if params.Responders != nil && params.Responders.Agents != "" {
			return params.Responders.Agents, nil
		}
		return dsl.ResponderAgentsDeny, nil
	}
	if node.Class == dsl.NodeClassAction && node.Review != nil &&
		(requestKind == "" || requestKind == RequestKindReview) {
		if node.Review.Responders != nil && node.Review.Responders.Agents != "" {
			return node.Review.Responders.Agents, nil
		}
		return dsl.ResponderAgentsDeny, nil
	}
	return "", fmt.Errorf("%w: node %q does not own request kind %q", ErrValidation, node.ID, requestKind)
}

func (s *service) rejectSelfOperation(
	ctx context.Context,
	workspaceID WorkspaceID,
	runID RunID,
	actor task.ActorContext,
) error {
	if actor.Actor.Kind.Normalize() != task.ActorKindAgentSession {
		return nil
	}
	if s.responderPolicy == nil {
		return NewRequestReasonError(ReasonCodeRespondSelfDenied, ErrRespondSelfDenied, nil)
	}
	denied, err := s.responderPolicy.DeniesSelfOperation(ctx, string(workspaceID), string(runID), actor)
	if err != nil {
		return fmt.Errorf("loop: evaluate responder policy: %w", err)
	}
	if denied {
		return NewRequestReasonError(ReasonCodeRespondSelfDenied, ErrRespondSelfDenied, nil)
	}
	return nil
}
