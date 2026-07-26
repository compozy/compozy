package spec

var authoredContextOperationRegistry = buildAuthoredContextOperationRegistry()

func buildAuthoredContextOperationRegistry() []OperationSpec {
	groups := [][]OperationSpec{
		authoredContextSoulOperations(),
		authoredContextHeartbeatOperations(),
		authoredContextSessionOperations(),
	}
	items := make([]OperationSpec, 0)
	for _, group := range groups {
		items = append(items, group...)
	}
	return items
}
