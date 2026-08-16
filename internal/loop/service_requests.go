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
	node, found := graphNode(resolved.Definition.Graph, dsl.NodeID(request.NodeID))
	if !found || node.Class != dsl.NodeClassControl || dsl.ControlKind(node.Kind) != dsl.ControlAsk {
		return fmt.Errorf("%w: ask node %q is not in the pinned definition", ErrValidation, request.NodeID)
	}
	var params dsl.AskParams
	if err := node.Params.Decode(&params); err != nil {
		return fmt.Errorf("loop: decode pinned ask node %q: %w", request.NodeID, err)
	}
	if params.Responders != nil && params.Responders.Agents != "" {
		request.Agents = params.Responders.Agents
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
	if input.Decision != "" && input.Decision != RequestDecisionRespond {
		return RespondResult{}, NewRequestReasonError(
			ReasonCodeRequestValidationFailed,
			fmt.Errorf("%w: ask only accepts respond", ErrRequestValidationFailed),
			map[string]string{"decision": "ask only accepts respond"},
		)
	}
	if err := s.rejectSelfOperation(ctx, input.WorkspaceID, input.RunID, input.Actor); err != nil {
		return RespondResult{}, err
	}
	if input.Actor.Actor.Kind.Normalize() == task.ActorKindAgentSession {
		allowed, err := s.askAllowsAgentResponder(ctx, input)
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

func (s *service) askAllowsAgentResponder(ctx context.Context, input RespondInput) (bool, error) {
	run, err := s.store.GetLoopRun(ctx, input.WorkspaceID, input.RunID)
	if err != nil {
		return false, err
	}
	resolved, err := s.pinnedResolvedDefinition(ctx, run)
	if err != nil {
		return false, err
	}
	node, found := graphNode(resolved.Definition.Graph, dsl.NodeID(input.NodeID))
	if !found || node.Class != dsl.NodeClassControl || dsl.ControlKind(node.Kind) != dsl.ControlAsk {
		return false, fmt.Errorf("%w: ask node %q is not in the pinned definition", ErrValidation, input.NodeID)
	}
	var params dsl.AskParams
	if err := node.Params.Decode(&params); err != nil {
		return false, fmt.Errorf("loop: decode pinned ask node %q: %w", input.NodeID, err)
	}
	return params.Responders != nil && params.Responders.Agents == dsl.ResponderAgentsAllow, nil
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
