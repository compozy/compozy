package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
)

func windowManagerMigrationOperations(
	transports []Transport,
	workspace ParameterSpec,
	failures []ResponseSpec,
) []OperationSpec {
	return []OperationSpec{
		{
			Method:      httpMethodGet,
			Path:        windowManagerPath + "/migration",
			AppAccess:   AppAccessDenied,
			OperationID: "getWindowManagerMigration",
			Summary:     "Read window layout migration status",
			Tags:        []string{specWorkspacesKey},
			Transports:  transports,
			Parameters:  withProfileSelector(workspace),
			Responses: append(
				[]ResponseSpec{{Status: 200, Description: "OK", Body: windowmanager.MigrationStatus{}}},
				failures...),
		},
		{
			Method:      httpMethodPost,
			Path:        windowManagerPath + "/migration",
			AppAccess:   AppAccessDenied,
			OperationID: "updateWindowManagerMigration",
			Summary:     "Acknowledge or retry window layout migration",
			Tags:        []string{specWorkspacesKey},
			Transports:  transports,
			Parameters:  withProfileSelector(workspace),
			RequestBody: contract.WindowManagerMigrationRequest{},
			Responses: append(
				[]ResponseSpec{{Status: 200, Description: "OK", Body: windowmanager.MigrationStatus{}}},
				failures...),
		},
	}
}
