package core

import (
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) respondSettingsSkillsError(c *gin.Context, err error) {
	var sourceError *compozyconfig.SkillSourceValidationError
	if !errors.As(err, &sourceError) {
		h.respondError(c, StatusForSettingsError(err), err)
		return
	}
	h.respondSkillSourceValidationError(c, err, sourceError)
}

func (h *BaseHandlers) respondSkillSourceValidationError(
	c *gin.Context,
	err error,
	sourceError *compozyconfig.SkillSourceValidationError,
) {
	c.JSON(StatusForSettingsError(err), contract.SkillSourceValidationErrorResponse{
		Error: contract.SkillSourceValidationErrorPayload{
			Code:           sourceError.Code,
			Message:        sourceError.Message,
			Field:          sourceError.Field,
			Path:           sourceError.Path,
			ExistingSource: sourceError.ExistingSource,
			Valid:          append([]string(nil), sourceError.Valid...),
			Suggestion:     sourceError.Suggestion,
		},
	})
}

func (h *BaseHandlers) respondSettingsSkillsTypedError(c *gin.Context, err error) bool {
	var sourceError *compozyconfig.SkillSourceValidationError
	if !errors.As(err, &sourceError) {
		return false
	}
	h.respondSkillSourceValidationError(c, err, sourceError)
	return true
}
