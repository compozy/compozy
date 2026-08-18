package daemon

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func (s *daemonLoopAPIService) ListLoopRequests(
	ctx context.Context,
	workspaceID string,
	query core.LoopRequestListQuery,
) (contract.LoopRequestsResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopRequestsResponse{}, err
	}
	page, err := s.aggregate.ListRequests(ctx, ws, looppkg.RequestQuery{
		RunID: looppkg.RunID(strings.TrimSpace(query.RunID)), State: strings.TrimSpace(query.State),
		Cursor: strings.TrimSpace(query.Cursor), Limit: query.Limit,
	})
	if err != nil {
		return contract.LoopRequestsResponse{}, err
	}
	items := make([]contract.LoopRequestPayload, 0, len(page.Items))
	for _, request := range page.Items {
		items = append(items, loopRequestPayload(request))
	}
	return contract.LoopRequestsResponse{
		Items: items, Aggregates: contract.LoopRequestAggregates{Pending: page.Pending},
		NextCursor: page.NextCursor,
	}, nil
}

func (s *daemonLoopAPIService) GetLoopRequest(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	itemIndex int,
) (contract.LoopRequestPayload, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.LoopRequestPayload{}, err
	}
	request, err := s.aggregate.GetRequest(ctx, ws, looppkg.RequestRef{
		RunID: looppkg.RunID(strings.TrimSpace(runID)), NodeID: looppkg.NodeID(strings.TrimSpace(nodeID)),
		ItemIndex: itemIndex,
	})
	if err != nil {
		return contract.LoopRequestPayload{}, err
	}
	return loopRequestPayload(request), nil
}

func (s *daemonLoopAPIService) RespondLoopRequest(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.RespondLoopRequest,
	actor taskpkg.ActorContext,
) (contract.RespondLoopRequestResponse, error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return contract.RespondLoopRequestResponse{}, err
	}
	result, err := s.aggregate.Respond(ctx, looppkg.RespondInput{
		WorkspaceID: ws, RunID: looppkg.RunID(strings.TrimSpace(runID)),
		NodeID: looppkg.NodeID(strings.TrimSpace(nodeID)), ItemIndex: req.ItemIndex,
		Decision: strings.TrimSpace(req.Decision), Payload: append(json.RawMessage(nil), req.Payload...),
		Note: strings.TrimSpace(req.Note), Actor: actor,
	})
	if err != nil {
		return contract.RespondLoopRequestResponse{}, err
	}
	answeredAt := result.Request.ResolvedAt
	if answeredAt == nil {
		return contract.RespondLoopRequestResponse{}, looppkg.NewRequestReasonError(
			looppkg.ReasonCodeRequestValidationFailed,
			looppkg.ErrRequestValidationFailed,
			map[string]string{"request": "answered request is missing resolution provenance"},
		)
	}
	return contract.RespondLoopRequestResponse{
		OK:       true,
		RunID:    string(result.Request.LoopRunID),
		NodeID:   string(result.Request.NodeID),
		Decision: result.Request.AnsweredDecision,
		State:    result.Request.State,
		Provenance: contract.LoopRequestProvenance{
			ActorKind:  result.Request.ActorKind,
			ActorID:    result.Request.ActorID,
			AnsweredAt: *answeredAt,
		},
	}, nil
}

func loopRequestPayload(request looppkg.Request) contract.LoopRequestPayload {
	payload := contract.LoopRequestPayload{
		LoopRunID:  string(request.LoopRunID),
		LoopName:   request.LoopName,
		Generation: request.Generation,
		NodeID:     string(request.NodeID),
		ItemIndex:  request.ItemIndex,
		Kind:       request.Kind,
		State:      request.State,
		Prompt:     request.Prompt,
		Context: append(
			json.RawMessage(nil),
			request.Context...),
		Expect:           append(json.RawMessage(nil), request.Expect...),
		EditSchema:       append(json.RawMessage(nil), request.EditSchema...),
		RespondSchema:    append(json.RawMessage(nil), request.RespondSchema...),
		ProposedPreview:  append(json.RawMessage(nil), request.ProposedPreview...),
		Decisions:        append([]string(nil), request.Decisions...),
		Agents:           string(request.Agents),
		AnsweredDecision: request.AnsweredDecision,
		ActorKind:        request.ActorKind,
		ActorID:          request.ActorID,
		OpenedAt:         request.OpenedAt,
		ResolvedAt:       cloneOptional(request.ResolvedAt),
		ExpiresAt:        cloneOptional(request.ExpiresAt),
	}
	if request.State == looppkg.RequestStateAnswered {
		payload.AnsweredAt = cloneOptional(request.ResolvedAt)
	}
	return payload
}
