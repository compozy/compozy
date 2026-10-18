package spec

import (
	"encoding/json"

	"github.com/compozy/compozy/internal/api/contract"
)

func openAPIDocumentOperation() OperationSpec {
	return OperationSpec{
		AppAccess:   AppAccessAllowed,
		Method:      httpMethodGet,
		Path:        "/api/openapi.json",
		OperationID: "getOpenAPIDocument",
		Summary:     "Get the running daemon's OpenAPI document",
		Tags:        []string{specDiagnosticsKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			{
				Name:        "If-None-Match",
				In:          "header",
				Kind:        specStringKey,
				Description: "Previously returned document ETag",
			},
			{
				Name: "Accept-Encoding", In: "header", Kind: specStringKey,
				Description: "Use gzip to request compressed document bytes",
			},
		},
		Responses: []ResponseSpec{
			{Status: 200, Description: "Release OpenAPI JSON; optionally gzip encoded", Body: json.RawMessage{},
				Headers: map[string]ParameterSpec{
					"ETag": {
						Kind:        specStringKey,
						Description: "Digest of the uncompressed release document",
					},
					"Cache-Control": {Kind: specStringKey, Enum: []string{"no-cache"}},
					"Content-Encoding": {
						Kind:        specStringKey,
						Enum:        []string{"gzip"},
						Description: "Present when gzip is negotiated",
					},
				}},
			{Status: 304, Description: "ETag matched; response body is empty"},
			{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
		},
	}
}
