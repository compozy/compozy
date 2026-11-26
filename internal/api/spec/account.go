package spec

import (
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/account"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/remoteproto"
)

const specAccountKey = "account"

func accountOperations() []OperationSpec {
	operations := append(accountIdentityOperations(), accountLifecycleOperations()...)
	operations = append(operations, accountDeviceOperations()...)
	return append(operations, accountApprovalBillingOperations()...)
}

func accountIdentityOperations() []OperationSpec {
	const base = contract.AccountPath
	return []OperationSpec{
		accountOperation(
			http.MethodGet,
			base,
			"getAccount",
			"Read the installation account",
			nil,
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Installation account status",
				Body:        contract.AccountStatusResponse{},
			},
		),
		accountOperation(
			http.MethodPost,
			contract.AccountLoginPath,
			"startAccountLogin",
			"Start account sign-in",
			contract.AccountLoginRequest{},
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Device activation",
				Body:        contract.AccountLoginResponse{},
			},
		),
		accountOperation(
			http.MethodDelete,
			contract.AccountLoginPath,
			"cancelAccountLogin",
			"Cancel account sign-in",
			nil,
			ResponseSpec{Status: http.StatusNoContent, Description: "Sign-in attempt ended"},
		),
		accountOperation(
			http.MethodGet,
			contract.AccountSignInEventsPath,
			"streamAccountSignIn",
			"Stream anonymous sign-in progress",
			nil,
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Sign-in progress and account state",
				ContentType: specContentTypeEventStream,
				Bodies: overlappingResponseBodiesOf(
					responseBodyOf[contract.AccountSignInProgress](),
					responseBodyOf[contract.AccountStatePayload](),
				),
			},
		),
		accountOperation(
			http.MethodPost,
			contract.AccountLogoutPath,
			"logoutAccount",
			"Sign out this installation",
			contract.AccountLogoutRequest{},
			ResponseSpec{Status: http.StatusOK, Description: "Signed out", Body: contract.AccountLogoutResponse{}},
		),
		accountOperation(
			http.MethodGet,
			base+"/stream",
			"streamAccount",
			"Stream changes for the active account",
			nil,
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Account changes and stream resets",
				ContentType: specContentTypeEventStream,
				Body:        account.Event{},
			},
		),
	}
}

func accountLifecycleOperations() []OperationSpec {
	return []OperationSpec{
		accountOperation(
			http.MethodPatch,
			contract.AccountPath,
			"patchAccountEmail",
			"Update the product news preference",
			contract.AccountEmailPatchRequest{},
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Updated installation account",
				Body:        contract.AccountStatusResponse{},
			},
		),
		accountOperation(
			http.MethodDelete,
			contract.AccountPath,
			"deleteAccount",
			"Start account deletion and sign out",
			contract.AccountDeleteRequest{},
			ResponseSpec{
				Status:      http.StatusAccepted,
				Description: "Deletion started",
				Body:        contract.AccountDeleteResponse{},
			},
		),
		accountOperation(
			http.MethodPost, contract.AccountDismissSignInNoticesPath, "dismissAccountSignInNotices",
			"Dismiss sign-in notices on this installation", nil,
			ResponseSpec{Status: http.StatusNoContent, Description: "Notices dismissed"},
		),
		accountOperation(http.MethodGet, contract.AccountTokensPath, "listAccountTokens", "List account tokens", nil,
			ResponseSpec{Status: http.StatusOK, Body: remoteproto.AccountTokenList{}, Description: "Account tokens"}),
		accountOperation(
			http.MethodPost,
			contract.AccountTokensPath,
			"createAccountToken",
			"Create an account token for local use",
			remoteproto.AccountTokenCreate{},
			ResponseSpec{
				Status:      http.StatusCreated,
				Body:        remoteproto.AccountTokenCreated{},
				Description: "One-time account token",
			},
		),
		accountOperation(
			http.MethodDelete,
			contract.AccountTokensPath+"/{id}",
			"revokeAccountToken",
			"Revoke an account token",
			nil,
			ResponseSpec{Status: http.StatusNoContent, Description: "Account token revoked"},
		),
	}
}

func accountDeviceOperations() []OperationSpec {
	const base = contract.AccountPath
	return []OperationSpec{
		accountOperation(
			http.MethodGet,
			base+"/devices",
			"listAccountDevices",
			"List the account's devices",
			nil,
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Account devices",
				Body:        contract.AccountDevicesResponse{},
			},
		),
		accountOperation(
			http.MethodPatch,
			base+"/devices/{id}",
			"patchAccountDevice",
			"Update a device on this account",
			remoteproto.AccountDevicePatch{},
			ResponseSpec{Status: http.StatusOK, Description: "Updated device", Body: remoteproto.AccountDevice{}},
		),
		accountOperation(
			http.MethodDelete,
			base+"/devices/{id}",
			"revokeAccountDevice",
			"Revoke a device on this account",
			nil,
			ResponseSpec{Status: http.StatusNoContent, Description: "Device revoked"},
		),
	}
}

func accountApprovalBillingOperations() []OperationSpec {
	const base = contract.AccountPath
	return []OperationSpec{
		accountOperation(
			http.MethodGet,
			base+"/approvals",
			"listAccountApprovals",
			"List pending and recently decided approvals",
			nil,
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Account approvals",
				Body:        contract.AccountApprovalsResponse{},
			},
		),
		accountOperation(
			http.MethodGet,
			base+"/approvals/{id}",
			"getAccountApproval",
			"Read an approval and its applied result",
			nil,
			ResponseSpec{Status: http.StatusOK, Description: "Device approval", Body: remoteproto.AccountApproval{}},
		),
		accountOperation(
			http.MethodPost,
			base+"/approvals/{id}/decision",
			"decideAccountApproval",
			"Deliver an approval decision",
			contract.AccountApprovalDecisionRequest{},
			ResponseSpec{Status: http.StatusAccepted, Description: "Decision queued for the computer",
				Body: remoteproto.ApprovalDelivery{}},
		),
		accountOperation(
			http.MethodPost,
			contract.AccountCheckoutPath,
			"checkoutAccount",
			"Start a Pro checkout",
			contract.AccountCheckoutRequest{},
			ResponseSpec{Status: http.StatusOK, Description: "Checkout URL", Body: contract.AccountBillingResponse{}},
		),
		accountOperation(
			http.MethodPost,
			base+"/billing-portal",
			"accountBillingPortal",
			"Open account billing",
			nil,
			ResponseSpec{
				Status:      http.StatusOK,
				Description: "Billing portal URL",
				Body:        contract.AccountBillingResponse{},
			},
		),
	}
}

func accountOperation(method, path, id, summary string, request any, success ResponseSpec) OperationSpec {
	operation := OperationSpec{
		Method:              method,
		Path:                path,
		OperationID:         id,
		Summary:             summary,
		RequestBody:         request,
		AppAccess:           AppAccessDenied,
		Tags:                []string{specAccountKey},
		Transports:          []Transport{TransportHTTP, TransportUDS},
		RequestBodyOptional: id == "startAccountLogin" || id == "logoutAccount",
		Responses: []ResponseSpec{
			success,
			{Status: http.StatusBadRequest, Description: "Invalid account request", Body: contract.ErrorPayload{}},
			{
				Status:      http.StatusUnauthorized,
				Description: "Sign-in required or session rejected",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      http.StatusForbidden,
				Description: "Device or account permission required",
				Body:        contract.ErrorPayload{},
			},
			{Status: http.StatusNotFound, Description: "Account resource not found", Body: contract.ErrorPayload{}},
			{
				Status:      http.StatusConflict,
				Description: "Account operation conflicts with current state",
				Body:        contract.ErrorPayload{},
			},
			{Status: http.StatusTooManyRequests, Description: "Cloud rate limit", Body: contract.ErrorPayload{}},
			{
				Status:      http.StatusServiceUnavailable,
				Description: "Account or cloud unavailable",
				Body:        contract.ErrorPayload{},
			},
		},
	}
	if strings.Contains(path, "{id}") {
		operation.Parameters = []ParameterSpec{pathParam("id", "Account resource ID")}
	}
	if id == "getAccount" {
		operation.Parameters = []ParameterSpec{boolQueryParam("refresh", "Refresh account details from Compozy Cloud")}
	}
	if id == "cancelAccountLogin" {
		operation.Parameters = []ParameterSpec{queryParam("attempt_id", "Cancel only the named sign-in attempt", false)}
	}
	if id == "streamAccount" {
		operation.Parameters = []ParameterSpec{
			queryParam("after_sequence", "Last account event ID in this auth generation", false),
		}
	}
	return operation
}
