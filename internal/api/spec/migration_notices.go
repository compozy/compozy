package spec

import (
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func migrationNoticeOperations() []OperationSpec {
	errors := []ResponseSpec{
		{Status: http.StatusUnauthorized, Description: "Sign-in required", Body: contract.ErrorPayload{}},
		{Status: http.StatusForbidden, Description: "Operator permission required", Body: contract.ErrorPayload{}},
		{
			Status:      http.StatusInternalServerError,
			Description: specInternalServerErrorDescription,
			Body:        contract.ErrorPayload{},
		},
	}
	tags := []string{"migration-notices"}
	transports := []Transport{TransportHTTP, TransportUDS}
	return []OperationSpec{
		{
			Method: http.MethodGet, Path: contract.MigrationNoticesPath, OperationID: "listMigrationNotices",
			Summary: "List undismissed upgrade notices on this computer", Tags: tags, Transports: transports,
			AppAccess: AppAccessDenied,
			Responses: append([]ResponseSpec{
				{Status: http.StatusOK, Description: "Upgrade notices", Body: contract.MigrationNoticesResponse{}},
			}, errors...),
		},
		{
			Method:      http.MethodPost,
			Path:        contract.MigrationNoticesPath + "/{key}/dismiss",
			OperationID: "dismissMigrationNotice",
			Summary:     "Dismiss an upgrade notice on this computer", Tags: tags, Transports: transports,
			AppAccess:  AppAccessDenied,
			Parameters: []ParameterSpec{pathParam("key", "Migration notice key")},
			Responses: append([]ResponseSpec{
				{Status: http.StatusNoContent, Description: "Notice dismissed"},
			}, errors...),
		},
	}
}
