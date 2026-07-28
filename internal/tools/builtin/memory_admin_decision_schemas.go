package builtin

var (
	memoryAdminDecisionListInputSchema = memoryAdminSchema(
		nil,
		memoryAdminSelectorProperties+
			`,"operation":{"type":"string"}`+
			`,"filename":{"type":"string"}`+
			`,"since":{"type":"string"}`+
			`,"reason":{"type":"string"}`+
			`,"limit":{"type":"integer"}`,
	)
	memoryAdminDecisionIDInputSchema = memoryAdminSchema(
		[]string{"decision_id"},
		`"decision_id":{"type":"string"}`,
	)
	memoryAdminDecisionRevertInputSchema = memoryAdminSchema(
		[]string{"decision_id"},
		`"decision_id":{"type":"string"},"reason":{"type":"string"},"dry_run":{"type":"boolean"}`,
	)
)
