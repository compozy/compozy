package spec

import "github.com/compozy/compozy/internal/api/contract"

const (
	specAppTokensKey              = "app-tokens"
	specAppLocalCallerDescription = "App-token management is restricted to local callers"
)

func appTokenOperations() []OperationSpec {
	return []OperationSpec{
		{
			AppAccess: AppAccessDenied, Stability: schemaStabilityExperimental,
			Method: httpMethodPost, Path: "/api/app-tokens", OperationID: "createAppToken",
			Summary: "Create an app token and reveal its bearer secret once", Tags: []string{specAppTokensKey},
			Transports: []Transport{TransportHTTP, TransportUDS}, RequestBody: contract.CreateAppTokenRequest{},
			Responses: []ResponseSpec{
				{
					Status:      201,
					Description: "App token created; store the secret now",
					Body:        contract.AppTokenIssuedResponse{},
				},
				{Status: 400, Description: "Invalid app token name or origins", Body: contract.ErrorPayload{}},
				{Status: 403, Description: specAppLocalCallerDescription, Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Active app token limit reached", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			AppAccess: AppAccessDenied, Stability: schemaStabilityExperimental,
			Method: httpMethodGet, Path: "/api/app-tokens", OperationID: "listAppTokens",
			Summary: "List app tokens without their bearer secrets", Tags: []string{specAppTokensKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				{
					Name:        "state",
					In:          specParameterInQuery,
					Kind:        specStringKey,
					Enum:        []string{"active", "revoked", "all"},
					Description: "Token state; defaults to active",
				},
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Active tokens first, then revoked; newest first within each group",
					Body: contract.AppTokensResponse{}},
				{Status: 400, Description: "Invalid app token state", Body: contract.ErrorPayload{}},
				{Status: 403, Description: specAppLocalCallerDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			AppAccess: AppAccessDenied, Stability: schemaStabilityExperimental,
			Method: httpMethodPatch, Path: "/api/app-tokens/{app_token_id}", OperationID: "updateAppToken",
			Summary: "Change an app token name or replace its origin allowlist", Tags: []string{specAppTokensKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters:  []ParameterSpec{pathParam("app_token_id", "App token id")},
			RequestBody: contract.UpdateAppTokenRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: specUpdatedDescription, Body: contract.AppTokenResponse{}},
				{Status: 400, Description: "Invalid app token name or origins", Body: contract.ErrorPayload{}},
				{Status: 403, Description: specAppLocalCallerDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: "App token not found", Body: contract.ErrorPayload{}},
				{Status: 409, Description: "Revoked app tokens cannot be updated", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			AppAccess: AppAccessDenied, Stability: schemaStabilityExperimental,
			Method: httpMethodDelete, Path: "/api/app-tokens/{app_token_id}", OperationID: "revokeAppToken",
			Summary: "Revoke an app token and cancel its active streams", Tags: []string{specAppTokensKey},
			Transports: []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{pathParam("app_token_id", "App token id")},
			Responses: []ResponseSpec{
				{Status: 200, Description: "App token revoked; changed is false when already revoked",
					Body: contract.AppTokenRevokeResponse{}},
				{Status: 403, Description: specAppLocalCallerDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: "App token not found", Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
	}
}
