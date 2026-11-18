package spec

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/getkin/kin-openapi/openapi3"
)

const windowManagerSupersededBackupsKey = "superseded_backups"

func customizeWindowManagerMigrationRequestSchema(schema *openapi3.Schema) {
	customizeClosedObjectSchema(schema)
	setStringEnum(schema.Properties["action"].Value, []string{"ack", "retry"})
	schema.Properties["replace"].Value.Description = "Replace the layout on retry when it changed after failure."
}

func customizeWindowManagerMigrationStatusSchema(schema *openapi3.Schema) {
	schema.Description = "Layout migration status. A conversion diagnostic is present in error only " +
		"when state is failed; a successful retry clears error."
	setStringEnum(schema.Properties["state"].Value, []string{specNoneKey, "migrated", specFailedKey})
	schema.Properties["failure_revision"].Value.Description =
		"Topology revision installed on conversion failure; cleared after a successful retry."
	schema.Properties[windowManagerSupersededBackupsKey].Value.Description =
		"Number of set-aside layout backups currently retained for this profile; omitted when zero."
	schema.Properties[windowManagerSupersededBackupsKey].Value.Min = new(float64(0))
}

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
