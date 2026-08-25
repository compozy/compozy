package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	callspkg "github.com/compozy/compozy/internal/calls"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondCallsError(c *gin.Context, err error) {
	c.JSON(statusForCallsError(err), callErrorResponse(err))
}

func callErrorResponse(err error) contract.CallErrorResponse {
	base := ErrorPayloadForError(err)
	payload := contract.CallErrorResponse{Error: base.Error, Code: base.Code, Details: base.Details}
	var typed *callspkg.Error
	if !errors.As(err, &typed) {
		return payload
	}
	payload.Code = string(typed.Code)
	payload.Widening = append([]string(nil), typed.Widening...)
	payload.OriginalID = typed.OriginalID
	for _, item := range typed.Available {
		payload.Available = append(payload.Available, contract.CallAgentOption{
			Name: item.Name, Description: item.Description,
		})
	}
	if payload.Details == nil {
		payload.Details = make(map[string]string)
	}
	if typed.ResetAt != "" {
		payload.Details["reset_at"] = typed.ResetAt
	}
	if typed.ExpiredAt != "" {
		payload.Details["expired_at"] = typed.ExpiredAt
	}
	if typed.Suggestion != "" {
		payload.Details["suggestion"] = typed.Suggestion
	}
	if len(payload.Details) == 0 {
		payload.Details = nil
	}
	return payload
}

func statusForCallsError(err error) int {
	var typed *callspkg.Error
	if !errors.As(err, &typed) {
		return http.StatusInternalServerError
	}
	switch typed.Code {
	case callspkg.CodeAgentUnknown, callspkg.CodeTargetNotFound,
		callspkg.CodeMessageNotFound:
		return http.StatusNotFound
	case callspkg.CodeTargetDenied, callspkg.CodeWorkspaceDenied, callspkg.CodeMessageTargetDenied,
		callspkg.CodeSettlementDenied:
		return http.StatusForbidden
	case callspkg.CodeTargetExpired:
		return http.StatusGone
	case callspkg.CodeIdempotencyConflict, callspkg.CodeAlreadySettled, callspkg.CodeNotSettled,
		callspkg.CodePublishNotSettled, callspkg.CodeMessageTargetBlocked, callspkg.CodeMessageDuplicate:
		return http.StatusConflict
	case callspkg.CodeMessageTooLarge:
		return http.StatusRequestEntityTooLarge
	case callspkg.CodeMessageRateLimited:
		return http.StatusTooManyRequests
	default:
		return http.StatusUnprocessableEntity
	}
}
