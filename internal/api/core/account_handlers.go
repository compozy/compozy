package core

import (
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cloudclient"
	"github.com/compozy/compozy/internal/remoteproto"
	"github.com/compozy/compozy/internal/version"
	"github.com/gin-gonic/gin"
)

func AccountStatus(installation account.Installation) contract.AccountStatusResponse {
	status := contract.AccountStatusResponse{
		Gate:     "off",
		State:    installation.State,
		Source:   optionalAccountString(installation.Source),
		SignedIn: installation.State == account.SignedIn,
		SignedOutReason: optionalAccountString(
			installation.SignedOutReason,
		),
		PreAccountInstall: installation.PreAccountInstall,
		RecentSignIns:     []contract.AccountSignInNotice{},
	}
	if version.SignInRequired == "true" {
		status.Gate = "enforced"
	}
	if !status.SignedIn {
		return status
	}
	profile := installation.Profile
	status.Email, status.Name, status.Provider = profile.Email, profile.Name, profile.Provider
	status.AccountPlan, status.MarketingOptIn = profile.AccountPlan, profile.MarketingOptIn
	if profile.RecentSignIns != nil {
		status.RecentSignIns = profile.RecentSignIns
	}
	if profile.Device.ID != "" {
		status.Device = &contract.AccountDeviceSummary{ID: string(profile.Device.ID), Name: profile.Device.Name,
			Admin: profile.Device.Admin}
	}
	return status
}

func optionalAccountString(value string) *string {
	if value == "" {
		return nil
	}
	return new(value)
}

func (h *BaseHandlers) GetAccount(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	c.JSON(http.StatusOK, AccountStatus(h.Account.Status()))
}

func (h *BaseHandlers) StartAccountLogin(c *gin.Context) {
	var input contract.AccountLoginRequest
	if !h.accountReady(c) || !decodeAccountRequest(c, &input, true) {
		return
	}
	var provider string
	if input.Provider != nil {
		provider = *input.Provider
	}
	result, err := h.Account.StartLoginWithConsent(c.Request.Context(), provider, input.MarketingOptIn)
	h.accountResult(c, http.StatusOK, contract.AccountLoginResponse(result), err)
}

func (h *BaseHandlers) CancelAccountLogin(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	h.accountResult(c, http.StatusNoContent, nil, h.Account.CancelLogin(c.Request.Context(), c.Query("attempt_id")))
}

func (h *BaseHandlers) LogoutAccount(c *gin.Context) {
	var input contract.AccountLogoutRequest
	if !h.accountReady(c) || !decodeAccountRequest(c, &input, true) {
		return
	}
	h.accountResult(
		c,
		http.StatusOK,
		contract.AccountLogoutResponse{State: account.SignedOut},
		h.Account.Logout(c.Request.Context()),
	)
}

func (h *BaseHandlers) ListAccountDevices(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	devices, err := h.Account.Devices(c.Request.Context())
	h.accountResult(c, http.StatusOK, contract.AccountDevicesResponse{Devices: devices}, err)
}

func (h *BaseHandlers) PatchAccountDevice(c *gin.Context) {
	var input remoteproto.AccountDevicePatch
	if !h.accountReady(c) || !decodeAccountRequest(c, &input, false) {
		return
	}
	device, err := h.Account.PatchDevice(c.Request.Context(), c.Param("id"), input)
	h.accountResult(c, http.StatusOK, device, err)
}

func (h *BaseHandlers) RevokeAccountDevice(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	h.accountResult(c, http.StatusNoContent, nil, h.Account.RevokeDevice(c.Request.Context(), c.Param("id")))
}

func (h *BaseHandlers) ListAccountApprovals(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	approvals, err := h.Account.Approvals(c.Request.Context())
	h.accountResult(c, http.StatusOK, contract.AccountApprovalsResponse{Approvals: approvals}, err)
}

func (h *BaseHandlers) GetAccountApproval(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	approval, err := h.Account.Approval(c.Request.Context(), c.Param("id"))
	h.accountResult(c, http.StatusOK, approval, err)
}

func (h *BaseHandlers) DecideAccountApproval(c *gin.Context) {
	var input contract.AccountApprovalDecisionRequest
	if !h.accountReady(c) || !decodeAccountRequest(c, &input, false) {
		return
	}
	if input.Approve == nil {
		invalidAccountRequest(c)
		return
	}
	result, err := h.Account.DecideApproval(c.Request.Context(), c.Param("id"), *input.Approve)
	h.accountResult(c, http.StatusAccepted, result, err)
}

func (h *BaseHandlers) CheckoutAccount(c *gin.Context) {
	var input contract.AccountCheckoutRequest
	if !h.accountReady(c) || !decodeAccountRequest(c, &input, false) {
		return
	}
	url, err := h.Account.Checkout(c.Request.Context(), input.Interval)
	h.accountResult(c, http.StatusOK, contract.AccountBillingResponse{URL: url}, err)
}

func (h *BaseHandlers) AccountBillingPortal(c *gin.Context) {
	if !h.accountReady(c) {
		return
	}
	url, err := h.Account.BillingPortal(c.Request.Context())
	h.accountResult(c, http.StatusOK, contract.AccountBillingResponse{URL: url}, err)
}

func (h *BaseHandlers) accountReady(c *gin.Context) bool {
	c.Header("Cache-Control", "no-store")
	if h != nil && h.Account != nil {
		return true
	}
	c.JSON(
		http.StatusServiceUnavailable,
		contract.ErrorPayload{Code: "account_unavailable", Error: "Account service unavailable."},
	)
	return false
}

func decodeAccountRequest(c *gin.Context, input any, optional bool) bool {
	if optional && (c.Request.Body == nil || c.Request.ContentLength == 0) {
		return true
	}
	err := json.UnmarshalRead(
		http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10),
		input,
		json.RejectUnknownMembers(true),
	)
	if err == nil || (optional && errors.Is(err, io.EOF)) {
		return true
	}
	invalidAccountRequest(c)
	return false
}

func invalidAccountRequest(c *gin.Context) {
	c.JSON(
		http.StatusBadRequest,
		contract.ErrorPayload{Code: cmdPaletteInvalidRequestError, Error: "Check the account request and try again."},
	)
}

func (h *BaseHandlers) accountResult(c *gin.Context, status int, body any, err error) {
	if failure, ok := errors.AsType[*cloudclient.Failure](err); ok {
		payload := contract.ErrorPayload{Code: failure.Code, Error: failure.Message}
		if failure.Reason != "" {
			payload.Details = map[string]string{"reason": failure.Reason}
		}
		if !failure.RetryAt.IsZero() {
			remaining := max(time.Second, time.Until(failure.RetryAt))
			seconds := int64((remaining + time.Second - 1) / time.Second)
			c.Header("Retry-After", strconv.FormatInt(seconds, 10))
		}
		c.JSON(failure.Status, payload)
		return
	}
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	if status == http.StatusNoContent {
		c.Status(status)
		return
	}
	c.JSON(status, body)
}
