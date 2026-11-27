package core

import (
	"cmp"
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
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

func (h *BaseHandlers) logoutInstallation(ctx context.Context, unlink bool) error {
	if h.Account.Status().InstallationLinked {
		if !unlink || h.Remote == nil {
			return account.ErrLogoutRequiresUnlink
		}
		if err := h.Remote.Unlink(ctx); err != nil {
			return err
		}
	}
	return h.Account.Logout(ctx)
}

func (h *BaseHandlers) ListAccountTokens(c *gin.Context) {
	if h.accountReady(c) {
		result, err := h.Account.AccountTokens(c.Request.Context())
		h.accountResult(c, http.StatusOK, result, err)
	}
}

func (h *BaseHandlers) CreateAccountToken(c *gin.Context) {
	var input remoteproto.AccountTokenCreate
	if h.accountReady(c) && decodeAccountRequest(c, &input, false) {
		result, err := h.Account.CreateToken(c.Request.Context(), input.Name)
		h.accountResult(c, http.StatusCreated, result, err)
	}
}

func (h *BaseHandlers) RevokeAccountToken(c *gin.Context) {
	if h.accountReady(c) {
		h.accountResult(c, http.StatusNoContent, nil, h.Account.RevokeToken(c.Request.Context(), c.Param("id")))
	}
}
