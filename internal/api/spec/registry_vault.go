package spec

import "github.com/compozy/agh/internal/api/contract"

func registryVaultOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        specAPIVaultEntriesPath,
		OperationID: "listVaultSecrets",
		Summary:     "List redacted vault secret metadata",
		Tags:        []string{specVaultKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			queryParam("prefix", "Filter by vault ref prefix", false),
			queryParam("namespace", "Filter by vault namespace", false),
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.VaultSecretsResponse{}},
			{Status: 400, Description: "Invalid vault filter", Body: contract.ErrorPayload{}},
			{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
			{Status: 503, Description: specVaultServiceUnavailableDescription, Body: contract.ErrorPayload{}},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodGet,
			Path:        "/api/vault/secrets/metadata",
			OperationID: "getVaultSecretMetadata",
			Summary:     "Read redacted vault secret metadata",
			Tags:        []string{specVaultKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("ref", "Vault ref", true),
			},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.VaultSecretResponse{}},
				{Status: 400, Description: "Invalid vault ref", Body: contract.ErrorPayload{}},
				{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Vault secret not found", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specVaultServiceUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodPut,
			Path:        specAPIVaultEntriesPath,
			OperationID: "putVaultSecret",
			Summary:     "Create or update one write-only vault secret",
			Tags:        []string{specVaultKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			RequestBody: contract.PutVaultSecretRequest{},
			Responses: []ResponseSpec{
				{Status: 200, Description: "Stored", Body: contract.VaultSecretResponse{}},
				{Status: 400, Description: "Invalid vault secret payload", Body: contract.ErrorPayload{}},
				{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
				{Status: 503, Description: specVaultServiceUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodDelete,
			Path:        specAPIVaultEntriesPath,
			OperationID: "deleteVaultSecret",
			Summary:     "Delete one vault secret",
			Tags:        []string{specVaultKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Parameters: []ParameterSpec{
				queryParam("ref", "Vault ref", true),
			},
			Responses: []ResponseSpec{
				{Status: 204, Description: "Deleted"},
				{Status: 400, Description: "Invalid vault ref", Body: contract.ErrorPayload{}},
				{Status: 403, Description: specForbiddenDescription, Body: contract.ErrorPayload{}},
				{Status: 404, Description: "Vault secret not found", Body: contract.ErrorPayload{}},
				{Status: 503, Description: specVaultServiceUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
