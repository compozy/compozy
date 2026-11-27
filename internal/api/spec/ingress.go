package spec

import "github.com/compozy/compozy/internal/api/contract"

func ingressOperations() []OperationSpec {
	return []OperationSpec{ingressBindOperation(), ingressUnbindOperation()}
}

func ingressBindOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodPost, Path: contract.IngressBindingsPath,
		AppAccess:   AppAccessDenied,
		OperationID: "bindIngress", Summary: "Confirm one subject against the verified public endpoint",
		Tags: []string{"ingress"}, Transports: []Transport{TransportHTTP, TransportUDS},
		RequestBody: contract.IngressBindRequest{},
		Responses: []ResponseSpec{
			{
				Status:      200,
				Description: "Already confirmed",
				Body:        contract.IngressBindingResponse{},
			},
			{
				Status:      201,
				Description: specCreatedDescription,
				Body:        contract.IngressBindingResponse{},
			},
			{
				Status:      400,
				Description: "Invalid ingress binding request",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      403,
				Description: "Ingress subject is outside the caller workspace",
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Ingress subject not found", Body: contract.ErrorPayload{}},
			{
				Status:      503,
				Description: "Public ingress is not reachable",
				Body:        contract.ErrorPayload{},
			},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}

func ingressUnbindOperation() OperationSpec {
	return OperationSpec{
		Method: httpMethodDelete, Path: contract.IngressBindingsPath + "/{subject_kind}/{subject_id}",
		AppAccess:   AppAccessDenied,
		OperationID: "unbindIngress", Summary: "Remove one public ingress confirmation",
		Tags: []string{"ingress"}, Transports: []Transport{TransportHTTP, TransportUDS},
		Parameters: []ParameterSpec{
			pathParam("subject_kind", "Ingress subject kind"),
			pathParam("subject_id", "Ingress subject id"),
		},
		Responses: []ResponseSpec{
			{Status: 204, Description: "Unbound"},
			{Status: 400, Description: "Invalid ingress subject", Body: contract.ErrorPayload{}},
			{
				Status:      403,
				Description: "Ingress subject is outside the caller workspace",
				Body:        contract.ErrorPayload{},
			},
			{Status: 404, Description: "Ingress subject not found", Body: contract.ErrorPayload{}},
			{
				Status:      500,
				Description: specInternalServerErrorDescription,
				Body:        contract.ErrorPayload{},
			},
		},
	}
}
