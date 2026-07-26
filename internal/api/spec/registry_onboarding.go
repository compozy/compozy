package spec

import "github.com/compozy/agh/internal/api/contract"

func registryOnboardingOperations() []OperationSpec {
	return []OperationSpec{{
		Method:      httpMethodGet,
		Path:        "/api/onboarding",
		OperationID: "getOnboardingStatus",
		Summary:     "Get the first-run onboarding completion status",
		Tags:        []string{specOnboardingKey},
		Transports:  []Transport{TransportHTTP, TransportUDS},
		Responses: []ResponseSpec{
			{Status: 200, Description: "OK", Body: contract.OnboardingStatusResponse{}},
			{Status: 503, Description: specOnboardingStoreUnavailableDescription, Body: contract.ErrorPayload{}},
		},
	},
		{
			Method:      httpMethodPost,
			Path:        "/api/onboarding/complete",
			OperationID: "completeOnboarding",
			Summary:     "Mark first-run onboarding as completed",
			Tags:        []string{specOnboardingKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.OnboardingStatusResponse{}},
				{Status: 503, Description: specOnboardingStoreUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		},
		{
			Method:      httpMethodDelete,
			Path:        "/api/onboarding",
			OperationID: "resetOnboarding",
			Summary:     "Clear the onboarding completion flag",
			Tags:        []string{specOnboardingKey},
			Transports:  []Transport{TransportHTTP, TransportUDS},
			Responses: []ResponseSpec{
				{Status: 200, Description: "OK", Body: contract.OnboardingStatusResponse{}},
				{Status: 503, Description: specOnboardingStoreUnavailableDescription, Body: contract.ErrorPayload{}},
				{Status: 500, Description: specInternalServerErrorDescription, Body: contract.ErrorPayload{}},
			},
		}}
}
