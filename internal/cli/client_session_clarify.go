package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/agh/internal/api/contract"
)

// ClarificationsRecord is the shared live clarification list response.
type ClarificationsRecord = contract.ClarificationsResponse

// ClarificationPendingRecord is one live pending clarification.
type ClarificationPendingRecord = contract.ClarificationPendingPayload

// ClarificationAnswerRequest resolves one live clarification.
type ClarificationAnswerRequest = contract.ClarificationAnswerRequest

// ClarificationAnswerRecord is the exact shared clarification result.
type ClarificationAnswerRecord = contract.ClarificationAnswerPayload

func (c *unixSocketClient) ListSessionClarifications(
	ctx context.Context,
	sessionID string,
) (ClarificationsRecord, error) {
	var response ClarificationsRecord
	path, err := c.sessionScopedPath(ctx, sessionID, "/clarifications")
	if err != nil {
		return ClarificationsRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return ClarificationsRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) AnswerSessionClarification(
	ctx context.Context,
	sessionID string,
	requestID string,
	request ClarificationAnswerRequest,
) (ClarificationAnswerRecord, error) {
	target, err := requireNetworkPathValue("request_id", requestID)
	if err != nil {
		return ClarificationAnswerRecord{}, err
	}
	path, err := c.sessionScopedPath(
		ctx,
		sessionID,
		"/clarifications/"+url.PathEscape(target)+"/answer",
	)
	if err != nil {
		return ClarificationAnswerRecord{}, err
	}
	var response ClarificationAnswerRecord
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return ClarificationAnswerRecord{}, err
	}
	return response, nil
}
