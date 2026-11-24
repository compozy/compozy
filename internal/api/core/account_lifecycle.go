package core

import (
	"cmp"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) PatchAccountEmail(c *gin.Context) {
	var patch contract.AccountEmailPatchRequest
	if !h.accountReady(c) || !decodeAccountRequest(c, &patch, false) {
		return
	}
	if patch.MarketingOptIn == nil {
		invalidAccountRequest(c)
		return
	}
	result, err := h.Account.SetMarketingConsent(
		c.Request.Context(), *patch.MarketingOptIn, cmp.Or(patch.Source, "settings"),
	)
	h.accountResult(c, http.StatusOK, AccountStatus(result), err)
}

func (h *BaseHandlers) DeleteAccount(c *gin.Context) {
	var input contract.AccountDeleteRequest
	if !h.accountReady(c) || !decodeAccountRequest(c, &input, false) {
		return
	}
	result, err := h.Account.DeleteAccount(c.Request.Context(), input.ConfirmEmail)
	h.accountResult(c, http.StatusAccepted, contract.AccountDeleteResponse(result), err)
}

func (h *BaseHandlers) DismissAccountSignInNotices(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	h.accountResult(c, http.StatusNoContent, nil, h.Account.DismissSignInNotices(c.Request.Context()))
}
