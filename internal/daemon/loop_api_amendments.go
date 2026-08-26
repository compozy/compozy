package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/compozy/compozy/internal/contracts"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/diagnostics"
	looppkg "github.com/compozy/compozy/internal/loop"
	taskpkg "github.com/compozy/compozy/internal/task"
)

const amendmentInlineLimitBytes = 16 * 1024

func (s *daemonLoopAPIService) AmendLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	req contract.LoopNodeAmendRequest,
	actor taskpkg.ActorContext,
) (contract.LoopNodeAmendResponse, error) {
	ws, normalizedRunID, normalizedNodeID, err := normalizeLoopNodeIdentity(workspaceID, runID, nodeID)
	if err != nil {
		return contract.LoopNodeAmendResponse{}, err
	}
	amendment, err := s.aggregate.AmendNodeOutput(ctx, looppkg.AmendInput{
		WorkspaceID: ws, RunID: normalizedRunID, Generation: req.Generation,
		NodeID: normalizedNodeID, ItemIndex: req.ItemIndex, Payload: req.Payload,
		Reason: strings.TrimSpace(req.Reason), Actor: actor,
	})
	if err != nil {
		return contract.LoopNodeAmendResponse{}, err
	}
	payload, err := loopNodeAmendmentPayload(amendment)
	if err != nil {
		return contract.LoopNodeAmendResponse{}, err
	}
	return contract.LoopNodeAmendResponse{OK: true, Amendment: payload}, nil
}

func loopNodeAmendmentPayload(amendment looppkg.NodeAmendment) (contract.LoopNodeAmendmentPayload, error) {
	original, originalSummary, err := boundedAmendmentValue(amendment.Original)
	if err != nil {
		return contract.LoopNodeAmendmentPayload{}, err
	}
	amended, amendedSummary, err := boundedAmendmentValue(amendment.Amended)
	if err != nil {
		return contract.LoopNodeAmendmentPayload{}, err
	}
	return contract.LoopNodeAmendmentPayload{
		LoopRunID: string(amendment.LoopRunID), Generation: amendment.Generation,
		NodeID: string(amendment.NodeID), ItemIndex: amendment.ItemIndex, Sequence: amendment.Sequence,
		Original: original, Amended: amended,
		OriginalSummary: originalSummary, AmendedSummary: amendedSummary,
		ActorKind: amendment.ActorKind, ActorID: amendment.ActorID,
		Reason: amendment.Reason, CreatedAt: amendment.CreatedAt,
	}, nil
}

func boundedAmendmentValue(raw json.RawMessage) (
	json.RawMessage,
	*contract.LoopAmendmentValueSummary,
	error,
) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, nil, fmt.Errorf("daemon: decode Loop amendment value: %w", err)
	}
	redacted, err := json.Marshal(diagnostics.RedactValue(value))
	if err != nil {
		return nil, nil, fmt.Errorf("daemon: redact Loop amendment value: %w", err)
	}
	if len(redacted) <= amendmentInlineLimitBytes {
		return redacted, nil, nil
	}
	return nil, &contract.LoopAmendmentValueSummary{
		ByteSize: len(redacted), ContentHash: contracts.OutputRefForPayload(redacted),
	}, nil
}
